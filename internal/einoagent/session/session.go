// Package session 提供 SessionManager：Agent 会话的异步执行、审批与配置热更新。
package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/einoagent/checkpoint"
	"ops-mate/internal/einoagent/graph"
	"ops-mate/internal/einoagent/history"
	agentmodel "ops-mate/internal/einoagent/model"
	"ops-mate/internal/einoagent/prompt"
	agenttools "ops-mate/internal/einoagent/tools"
	"ops-mate/internal/sshexec"
	configstore "ops-mate/internal/store/config"
	convstore "ops-mate/internal/store/conversations"
	memorystore "ops-mate/internal/store/memory"
	"ops-mate/internal/store"
)

// 前端可见的会话状态（session:state 事件载荷）。
const (
	StateIdle             = "Idle"
	StateThinking         = "Thinking"
	StateAwaitingApproval = "AwaitingApproval"
	StateRunning          = "Running"
)

// runState 内部状态机。
type runState int

const (
	stIdle runState = iota
	stThinking
	stAwaitingApproval
	stRunning
)

func (s runState) String() string {
	switch s {
	case stThinking:
		return StateThinking
	case stAwaitingApproval:
		return StateAwaitingApproval
	case stRunning:
		return StateRunning
	default:
		return StateIdle
	}
}

// executorHolder 线程安全的 per-session SSH 执行器持有器。
type executorHolder struct {
	mu       sync.Mutex
	executor sshexec.Exec
}

func (h *executorHolder) Set(exec sshexec.Exec) {
	h.mu.Lock()
	h.executor = exec
	h.mu.Unlock()
}

// Exec 实现 sshexec.Exec，委托当前持有的执行器。
func (h *executorHolder) Exec(ctx context.Context, command string) (<-chan sshexec.Line, error) {
	h.mu.Lock()
	ex := h.executor
	h.mu.Unlock()
	if ex == nil {
		return nil, fmt.Errorf("执行器未配置")
	}
	return ex.Exec(ctx, command)
}

// agentSession 单个会话的运行时状态。
type agentSession struct {
	id     string
	hostID string

	mu          sync.Mutex
	state       runState
	graph       compose.Runnable[[]*schema.Message, []*schema.Message]
	builtAt     int // 构建时的 configVersion
	checkpoints *checkpoint.MemCheckpointStore
	interruptID string
	lastInput   []*schema.Message
	cancel      context.CancelFunc

	holder    *executorHolder
	toolCalls *agenttools.ToolCallHolder
}

// SessionManager 管理所有 Agent 会话。
type SessionManager struct {
	app         *store.DB
	convs       *convstore.ConvStore
	mem         *memorystore.MemoryStore
	cfg         *configstore.ConfigStore
	executorFor func(hostID string) sshexec.Exec
	emit        func(sessionID, event string, data any)

	// modelFactory 构造基础模型；可注入以便测试。
	// 默认走 einoagent.agentmodel.NewChatModel（eino-ext provider）。
	modelFactory func(ctx context.Context, cfg configstore.AIConfig) (einomodel.ToolCallingChatModel, error)

	mu            sync.Mutex
	sessions      map[string]*agentSession
	configVersion int
}

// NewSessionManager 构造会话管理器。
func NewSessionManager(
	app *store.DB,
	cfg *configstore.ConfigStore,
	executorFor func(hostID string) sshexec.Exec,
	emit func(sessionID, event string, data any),
) *SessionManager {
	return &SessionManager{
		app:         app,
		convs:       convstore.NewConvStore(app),
		mem:         memorystore.NewMemoryStore(app),
		cfg:         cfg,
		executorFor: executorFor,
		emit:        emit,
		modelFactory: func(ctx context.Context, c configstore.AIConfig) (einomodel.ToolCallingChatModel, error) {
			return agentmodel.NewChatModel(ctx, c)
		},
		sessions: map[string]*agentSession{},
	}
}

// InvalidateConfig 使所有会话的模型缓存失效（AI 配置热更新入口）。
// 进行中的轮次不受影响；下一轮 SendMessage/审批恢复时按新配置重建。
func (m *SessionManager) InvalidateConfig() {
	m.mu.Lock()
	m.configVersion++
	m.mu.Unlock()
}

// EnsureSession 返回主机当前会话（最近更新的 conversation），没有则懒创建。
func (m *SessionManager) EnsureSession(hostID string) (string, error) {
	convs, err := m.convs.ListConversations(hostID)
	if err != nil {
		return "", err
	}
	if len(convs) > 0 {
		return convs[0].ID, nil
	}
	return m.CreateSession(hostID, "AI 对话")
}

// CreateSession 新建一个 conversation（前端"新建对话"按钮）。
func (m *SessionManager) CreateSession(hostID, title string) (string, error) {
	return m.convs.NewConversation(hostID, title)
}

// sessionFor 获取或懒构建会话运行时。
func (m *SessionManager) sessionFor(sid string) (*agentSession, error) {
	m.mu.Lock()
	s, ok := m.sessions[sid]
	m.mu.Unlock()
	if ok {
		return s, nil
	}
	conv, err := m.convs.GetConversation(sid)
	if err != nil {
		return nil, fmt.Errorf("session %s not found: %w", sid, err)
	}
	s = &agentSession{
		id: sid, hostID: conv.HostID,
		checkpoints: checkpoint.NewMemCheckpointStore(),
		holder:      &executorHolder{},
		toolCalls:   agenttools.NewToolCallHolder(),
	}
	m.mu.Lock()
	if existing, ok := m.sessions[sid]; ok {
		s = existing
	} else {
		m.sessions[sid] = s
	}
	m.mu.Unlock()
	return s, nil
}

// sessionState 供测试与调试查询当前状态字符串。
func (m *SessionManager) sessionState(sid string) string {
	s, err := m.sessionFor(sid)
	if err != nil {
		return StateIdle
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.String()
}

// SendMessage 发送用户消息并异步启动 Agent 轮次。
func (m *SessionManager) SendMessage(sid, text string) error {
	s, err := m.sessionFor(sid)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.state != stIdle {
		s.mu.Unlock()
		return fmt.Errorf("会话进行中（%s），请等待本轮结束", s.state)
	}
	s.state = stThinking
	s.mu.Unlock()

	// 落库 user 消息
	if err := m.convs.SaveMessage(convstore.Message{
		SessionID: sid, Role: history.RoleUser, Content: text,
	}); err != nil {
		s.mu.Lock()
		s.state = stIdle
		s.mu.Unlock()
		return fmt.Errorf("保存消息失败: %w", err)
	}

	// 执行器检查（提前失败，避免进入图执行才报错）
	ex := m.executorFor(s.hostID)
	if ex == nil {
		s.mu.Lock()
		s.state = stIdle
		s.mu.Unlock()
		m.emitError(sid, "主机凭据不可用，请在主机页重新录入该主机的密码/密钥")
		return fmt.Errorf("executor unavailable for host %s", s.hostID)
	}
	s.holder.Set(ex)

	if err := m.ensureGraph(s); err != nil {
		s.mu.Lock()
		s.state = stIdle
		s.mu.Unlock()
		m.emitError(sid, "AI 后端不可用："+err.Error())
		return err
	}

	input, err := m.buildInput(s, text)
	if err != nil {
		s.mu.Lock()
		s.state = stIdle
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.lastInput = input
	s.mu.Unlock()

	m.emitState(sid, StateThinking)
	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	go m.run(s, runCtx, input, false, "")
	return nil
}

// ApproveCommand 批准命令（command 为用户最终确认的命令，可能编辑过）。
func (m *SessionManager) ApproveCommand(sid, command string) error {
	s, err := m.sessionFor(sid)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.state != stAwaitingApproval {
		s.mu.Unlock()
		return fmt.Errorf("当前无待审批命令（state=%s）", s.state)
	}
	s.state = stRunning
	input := s.lastInput
	s.mu.Unlock()

	m.emitState(sid, StateRunning)
	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	go m.run(s, runCtx, input, true, command)
	return nil
}

// RejectCommand 拒绝命令，回灌模型换方案。
func (m *SessionManager) RejectCommand(sid string) error {
	s, err := m.sessionFor(sid)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.state != stAwaitingApproval {
		s.mu.Unlock()
		return fmt.Errorf("当前无待审批命令（state=%s）", s.state)
	}
	s.state = stThinking
	input := s.lastInput
	s.mu.Unlock()

	m.emitState(sid, StateThinking)
	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	go m.run(s, runCtx, input, true, "rejected")
	return nil
}

// CancelRun 中止本轮执行：思考中（等待 LLM 生成）与执行中（命令运行）均可取消。
// 取消后 Graph Invoke 返回 context.Canceled，run() 会清理状态并回 Idle。
func (m *SessionManager) CancelRun(sid string) error {
	s, err := m.sessionFor(sid)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stRunning && s.state != stThinking {
		return fmt.Errorf("当前没有可取消的任务（state=%s）", s.state)
	}
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

// ClearMessages 清空会话的全部消息（保留会话，快捷命令 /clear）。
// 仅 Idle 态可执行；同时清理 checkpoint 与待执行 tool_call 队列，
// 避免残留状态混入后续轮次。
func (m *SessionManager) ClearMessages(sid string) error {
	s, err := m.sessionFor(sid)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.state != stIdle {
		s.mu.Unlock()
		return fmt.Errorf("会话进行中（%s），请等待本轮结束", s.state)
	}
	s.mu.Unlock()
	_ = s.checkpoints.Delete(context.Background(), sid)
	s.toolCalls.Reset()
	return m.convs.ClearMessages(sid)
}

// DeleteSession 删除会话（含运行时与 DB 记录）。
func (m *SessionManager) DeleteSession(sid string) error {
	m.mu.Lock()
	s, ok := m.sessions[sid]
	if ok {
		delete(m.sessions, sid)
	}
	m.mu.Unlock()
	if ok {
		s.mu.Lock()
		if s.cancel != nil {
			s.cancel()
		}
		s.mu.Unlock()
	}
	return m.convs.DeleteConversation(sid)
}

// run 执行 Graph（首次或 Resume），处理中断/错误/完成。
func (m *SessionManager) run(s *agentSession, ctx context.Context, input []*schema.Message, resume bool, resumeData string) {
	invokeCtx := ctx
	if resume {
		s.mu.Lock()
		id := s.interruptID
		s.mu.Unlock()
		invokeCtx = compose.ResumeWithData(ctx, id, resumeData)
	}

	_, err := s.graph.Invoke(invokeCtx, input, compose.WithCheckPointID(s.id))

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		if info, ok := compose.ExtractInterruptInfo(err); ok && len(info.InterruptContexts) > 0 {
			s.interruptID = info.InterruptContexts[0].ID
			s.state = stAwaitingApproval
			m.emitState(s.id, StateAwaitingApproval)
			return
		}
		s.state = stIdle
		s.interruptID = ""
		// 非中断错误也清理 checkpoint：若残留，下一轮 Invoke 会从上一轮
		// 中断/失败点恢复，导致 state 消息翻倍错乱。
		_ = s.checkpoints.Delete(ctx, s.id)
		// 整轮对话结束：清空 tool call 队列，防止取消/中断遗留混入下一轮导致配对错位。
		// （AwaitingApproval 分支不清——当前待审批 tool_call 仍需 Take。）
		s.toolCalls.Reset()
		if errors.Is(err, context.Canceled) {
			m.emitError(s.id, "本次执行已取消")
		} else {
			m.emitError(s.id, "AI 对话失败："+err.Error())
		}
		m.emitState(s.id, StateIdle)
		return
	}
	s.state = stIdle
	s.interruptID = ""
	_ = s.checkpoints.Delete(ctx, s.id)
	s.toolCalls.Reset()
	m.emitState(s.id, StateIdle)
}

// ensureGraph 懒构建/按配置版本重建 Graph。
func (m *SessionManager) ensureGraph(s *agentSession) error {
	m.mu.Lock()
	version := m.configVersion
	m.mu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.graph != nil && s.builtAt == version {
		return nil
	}

	ctx := context.Background()
	cfg, err := m.cfg.GetAIConfig()
	if err != nil {
		return fmt.Errorf("读取 AI 配置失败: %w", err)
	}
	if cfg.Provider == "" {
		return fmt.Errorf("尚未配置 AI 后端，请先到「AI 配置」页设置")
	}
	base, err := m.modelFactory(ctx, cfg)
	if err != nil {
		return fmt.Errorf("构建模型失败: %w", err)
	}

	sshTool := agenttools.NewSSHTool(s.id, s.holder, m.emit, m.convs, s.toolCalls)
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []einotool.BaseTool{sshTool}, ExecuteSequentially: true,
	})
	if err != nil {
		return fmt.Errorf("构建工具节点失败: %w", err)
	}

	wrapped := agentmodel.NewStreamingChatModel(base, s.id, m.emit, m.onAssistant(s))
	info, err := sshTool.Info(ctx)
	if err != nil {
		return fmt.Errorf("tool info: %w", err)
	}
	withTools, err := wrapped.WithTools([]*schema.ToolInfo{info})
	if err != nil {
		return fmt.Errorf("绑定工具失败: %w", err)
	}

	graph, err := graph.BuildAgentGraph(ctx, withTools, toolsNode, s.checkpoints)
	if err != nil {
		return fmt.Errorf("构建 Graph 失败: %w", err)
	}
	s.graph = graph
	s.builtAt = version
	return nil
}

// onAssistant 返回 assistant 消息落库回调（含 tool_calls 配对记录）。
func (m *SessionManager) onAssistant(s *agentSession) func(msg *schema.Message) {
	return func(msg *schema.Message) {
		_ = m.convs.SaveMessage(convstore.Message{
			SessionID: s.id, Role: history.RoleAssistant,
			Content:   msg.Content,
			ToolCalls: history.ToolCallsToJSON(msg.ToolCalls),
		})
		for i := range msg.ToolCalls {
			s.toolCalls.Add(&msg.ToolCalls[i])
		}
	}
}

// buildInput 组装模型输入：系统提示 + 记忆注入 + DB 历史。
func (m *SessionManager) buildInput(s *agentSession, userText string) ([]*schema.Message, error) {
	hist, err := m.convs.LoadMessages(s.id)
	if err != nil {
		return nil, fmt.Errorf("加载历史失败: %w", err)
	}
	msgs, err := history.HistoryToEino(hist)
	if err != nil {
		return nil, err
	}

	input := make([]*schema.Message, 0, len(msgs)+2)
	input = append(input, prompt.SystemMessage())

	// FTS5 记忆注入（失败不阻断主流程）
	if recall, err := m.mem.Recall(s.hostID, userText); err == nil && len(recall.PastCommands) > 0 {
		var note strings.Builder
		note.WriteString("该主机过去执行过的相关命令记录（供参考）：\n")
		for _, c := range recall.PastCommands {
			note.WriteString("- ")
			note.WriteString(c.Command)
			note.WriteString("\n")
		}
		input = append(input, schema.UserMessage(note.String()))
	}

	input = append(input, msgs...)
	return input, nil
}

func (m *SessionManager) emitState(sid, state string) {
	if m.emit != nil {
		m.emit(sid, "session:state", state)
	}
}

func (m *SessionManager) emitError(sid, message string) {
	if m.emit != nil {
		m.emit(sid, "ai:error", map[string]any{"message": message})
	}
}
