package handler

import (
	"ops-mate/internal/einoagent"
	convstore "ops-mate/internal/store/conversations"
)

// SessionsHandler 处理会话/对话相关的前端调用。
// 事件推送不经过本 handler：SessionManager 持有 main.go 注入的
// emitEvent 函数直接推 Wails 事件。
type SessionsHandler struct {
	convs          *convstore.ConvStore
	sessionManager *einoagent.SessionManager
}

// NewSessionsHandler 构造 SessionsHandler。
func NewSessionsHandler(convs *convstore.ConvStore, sm *einoagent.SessionManager) *SessionsHandler {
	return &SessionsHandler{convs: convs, sessionManager: sm}
}

// EnsureSession 获取（必要时懒创建）主机当前会话。
func (h *SessionsHandler) EnsureSession(hostID string) (string, error) {
	return h.sessionManager.EnsureSession(hostID)
}

// NewSession 新建一个会话（前端"新建对话"）。
func (h *SessionsHandler) NewSession(hostID, title string) (string, error) {
	return h.sessionManager.CreateSession(hostID, title)
}

// SendMessage 发送消息。executor 由 SessionManager 通过注入的
// ExecutorFor 工厂解析——修复旧版把 sessionID 误当 hostID 的 bug。
func (h *SessionsHandler) SendMessage(sid, text string) error {
	return h.sessionManager.SendMessage(sid, text)
}

func (h *SessionsHandler) ApproveCommand(sid, command string) error {
	return h.sessionManager.ApproveCommand(sid, command)
}

func (h *SessionsHandler) RejectCommand(sid string) error {
	return h.sessionManager.RejectCommand(sid)
}

func (h *SessionsHandler) CancelRun(sid string) error {
	return h.sessionManager.CancelRun(sid)
}

func (h *SessionsHandler) ListConversations(hostID string) ([]convstore.Conversation, error) {
	return h.convs.ListConversations(hostID)
}

func (h *SessionsHandler) LoadMessages(sid string) ([]convstore.Message, error) {
	return h.convs.LoadMessages(sid)
}

func (h *SessionsHandler) DeleteConversation(sid string) error {
	return h.sessionManager.DeleteSession(sid)
}
