package einoagent

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"
)

// executorHolder 线程安全的 per-session 执行器持有器。
// 实现 sshexec.Exec 接口，SSHTool 可直接持有此类型。
type executorHolder struct {
	mu       sync.Mutex
	executor sshexec.Exec
}

func (h *executorHolder) Set(exec sshexec.Exec) {
	h.mu.Lock()
	h.executor = exec
	h.mu.Unlock()
}

func (h *executorHolder) Get() sshexec.Exec {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.executor
}

// Exec 实现 sshexec.Exec 接口，委托给当前持有的 executor。
func (h *executorHolder) Exec(ctx context.Context, command string) (<-chan sshexec.Line, error) {
	h.mu.Lock()
	ex := h.executor
	h.mu.Unlock()
	if ex == nil {
		return nil, fmt.Errorf("执行器未配置")
	}
	return ex.Exec(ctx, command)
}

// SessionManager 管理所有对话会话。
type SessionManager struct {
	store     *store.Store
	baseModel model.ToolCallingChatModel

	mu       sync.Mutex
	sessions map[string]*agentSession
}

// agentSession 内部会话状态。
type agentSession struct {
	id          string
	hostID      string
	graph       compose.Runnable[*GraphState, *GraphState]
	state       *GraphState
	tools       []tool.BaseTool
	holder      *executorHolder
	interruptID string
}

// NewSessionManager 构造会话管理器。
func NewSessionManager(st *store.Store, baseModel model.ToolCallingChatModel) *SessionManager {
	return &SessionManager{
		store:     st,
		baseModel: baseModel,
		sessions:  map[string]*agentSession{},
	}
}

// CreateSession 创建新会话。
func (m *SessionManager) CreateSession(ctx context.Context, hostID, title string, emit func(string, string, any)) (string, error) {
	sid, err := m.store.NewConversation(hostID, title)
	if err != nil {
		return "", err
	}

	// Per-session 执行器持有器
	holder := &executorHolder{}

	// Per-session 工具（闭包捕获 holder + emit + sessionID）
	sshTool := NewSSHTool(sid, holder, emit)
	tools := []tool.BaseTool{sshTool}

	// 提取 ToolInfo 并绑定到模型
	toolInfos := make([]*schema.ToolInfo, 0, len(tools))
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			return "", fmt.Errorf("tool info: %w", err)
		}
		toolInfos = append(toolInfos, info)
	}
	modelWithTools, err := m.baseModel.WithTools(toolInfos)
	if err != nil {
		return "", fmt.Errorf("with tools: %w", err)
	}

	// Per-session Graph
	graph, err := BuildAgentGraph(ctx, modelWithTools, tools, m.store)
	if err != nil {
		return "", fmt.Errorf("build graph: %w", err)
	}

	s := &agentSession{
		id:     sid,
		hostID: hostID,
		graph:  graph,
		state:  NewGraphState(sid, hostID, nil, emit),
		tools:  tools,
		holder: holder,
	}

	m.mu.Lock()
	m.sessions[sid] = s
	m.mu.Unlock()

	return sid, nil
}

// SendMessage 发送消息到会话。
func (m *SessionManager) SendMessage(ctx context.Context, sid, text string, executor sshexec.Exec) error {
	m.mu.Lock()
	s, ok := m.sessions[sid]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", sid)
	}

	s.state.History = append(s.state.History, Message{Role: RoleUser, Content: text})
	s.holder.Set(executor)

	_, err := s.graph.Invoke(ctx, s.state)
	if err != nil {
		if info, ok := compose.ExtractInterruptInfo(err); ok && len(info.InterruptContexts) > 0 {
			s.interruptID = info.InterruptContexts[0].ID
			return nil // 正常中断，等审批
		}
		return err
	}
	return nil
}

// ApproveCommand 批准命令。
func (m *SessionManager) ApproveCommand(ctx context.Context, sid, command string) error {
	m.mu.Lock()
	s, ok := m.sessions[sid]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", sid)
	}

	resumeCtx := compose.ResumeWithData(ctx, s.interruptID, "approved")
	_, err := s.graph.Invoke(resumeCtx, s.state)
	if err != nil {
		if info, ok := compose.ExtractInterruptInfo(err); ok && len(info.InterruptContexts) > 0 {
			s.interruptID = info.InterruptContexts[0].ID
			return nil
		}
		return err
	}
	return nil
}

// RejectCommand 拒绝命令。
func (m *SessionManager) RejectCommand(ctx context.Context, sid string) error {
	m.mu.Lock()
	s, ok := m.sessions[sid]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", sid)
	}

	resumeCtx := compose.ResumeWithData(ctx, s.interruptID, "rejected")
	_, err := s.graph.Invoke(resumeCtx, s.state)
	if err != nil {
		if info, ok := compose.ExtractInterruptInfo(err); ok && len(info.InterruptContexts) > 0 {
			s.interruptID = info.InterruptContexts[0].ID
			return nil
		}
		return err
	}
	return nil
}

// CancelRun 中止正在执行的命令。
func (m *SessionManager) CancelRun(sid string) error {
	m.mu.Lock()
	_, ok := m.sessions[sid]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", sid)
	}
	// TODO: 实现 context 取消
	return nil
}

// DeleteSession 删除会话。
func (m *SessionManager) DeleteSession(sid string) error {
	m.mu.Lock()
	delete(m.sessions, sid)
	m.mu.Unlock()
	return m.store.DeleteConversation(sid)
}
