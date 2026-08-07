// Package session 提供 SessionManager：Agent 会话的异步执行、审批与配置热更新。
//
// 按职责拆分为多个文件：
//   - session.go  类型定义、状态机、结构体、构造函数
//   - lifecycle.go 会话生命周期（创建/查找/状态/事件）
//   - message.go   消息收发（发送/取消/清空/删除）
//   - approval.go  命令与计划的审批
//   - graph.go     图构建与执行（run/ensureGraph）
//   - input.go     模型输入组装（buildInput）
package session

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/callbacks"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/einoagent/callback"
	"ops-mate/internal/einoagent/checkpoint"
	agentmodel "ops-mate/internal/einoagent/model"
	agenttools "ops-mate/internal/einoagent/tools"
	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"
	configstore "ops-mate/internal/store/config"
	convstore "ops-mate/internal/store/conversations"
	memorystore "ops-mate/internal/store/memory"
)

// 前端可见的会话状态（session:state 事件载荷）。
const (
	StateIdle             = "Idle"
	StateThinking         = "Thinking"
	StateAwaitingApproval = "AwaitingApproval"
	StateRunning          = "Running"
)

// maxHistoryMessages 注入模型的会话历史条数上限（超出省略早期，控制上下文）。
const maxHistoryMessages = 30

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
	// 审批类型：等待审批时区分是"计划"（create_plan）还是"命令"（execute_command）
	approvalType string // "" | "plan" | "command"

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
	hostNameFor func(hostID string) (string, error) // 解析主机名（注入系统提示词）
	emit        func(sessionID, event string, data any)
	logHandler  callbacks.Handler // 智能体调用观测日志

	// modelFactory 构造基础模型；可注入以便测试。
	// 默认走 einoagent.agentmodel.NewChatModel（eino-ext provider）。
	modelFactory func(ctx context.Context, cfg configstore.AIConfig) (einomodel.ToolCallingChatModel, error)

	mu            sync.Mutex
	sessions      map[string]*agentSession
	configVersion int
}

// NewSessionManager 构造会话管理器。hostNameFor 可为 nil（此时系统提示词不含主机名）。
func NewSessionManager(
	app *store.DB,
	cfg *configstore.ConfigStore,
	executorFor func(hostID string) sshexec.Exec,
	hostNameFor func(hostID string) (string, error),
	emit func(sessionID, event string, data any),
) *SessionManager {
	return &SessionManager{
		app:         app,
		convs:       convstore.NewConvStore(app),
		mem:         memorystore.NewMemoryStore(app),
		cfg:         cfg,
		executorFor: executorFor,
		hostNameFor: hostNameFor,
		emit:        emit,
		logHandler:  callback.NewLogHandler(),
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
