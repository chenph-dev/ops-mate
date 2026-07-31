package handler

import (
	"ops-mate/internal/einoagent"
	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// SessionsHandler 处理会话/对话相关的前端调用。
type SessionsHandler struct {
	store          *store.Store
	sessionManager *einoagent.SessionManager
}

// NewSessionsHandler 构造 SessionsHandler。
func NewSessionsHandler(store *store.Store, sm *einoagent.SessionManager) *SessionsHandler {
	return &SessionsHandler{store: store, sessionManager: sm}
}

// getExecutor 按 hostID 取凭据构造 SSHExecutor。
func (h *SessionsHandler) getExecutor(hostID string) sshexec.Exec {
	secret, authType, err := h.store.GetHostSecret(hostID)
	if err != nil {
		return nil
	}
	meta, err := h.store.HostMetaByID(hostID)
	if err != nil || meta == nil {
		return nil
	}
	return sshexec.NewExecutor(sshexec.Host{
		Addr: meta.Addr, Port: meta.Port, User: meta.User,
		AuthType: authType, Secret: secret,
	})
}

// emit 向 Wails 前端推送事件。
func (h *SessionsHandler) emit(sessionID, event string, data any) {
	wailsruntime.EventsEmit(Ctx(), event, map[string]any{
		"sessionId": sessionID, "data": data,
	})
}

func (h *SessionsHandler) NewSession(hostID, title string) (string, error) {
	return h.sessionManager.CreateSession(Ctx(), hostID, title, h.emit)
}

func (h *SessionsHandler) SendMessage(sid, text string) error {
	return h.sessionManager.SendMessage(Ctx(), sid, text, h.getExecutor(sid))
}

func (h *SessionsHandler) ApproveCommand(sid, command string) error {
	return h.sessionManager.ApproveCommand(Ctx(), sid, command)
}

func (h *SessionsHandler) RejectCommand(sid string) error {
	return h.sessionManager.RejectCommand(Ctx(), sid)
}

func (h *SessionsHandler) CancelRun(sid string) error {
	return h.sessionManager.CancelRun(sid)
}

func (h *SessionsHandler) ListConversations(hostID string) ([]store.Conversation, error) {
	return h.store.ListConversations(hostID)
}

func (h *SessionsHandler) LoadMessages(sid string) ([]store.Message, error) {
	return h.store.LoadMessages(sid)
}

func (h *SessionsHandler) DeleteConversation(sid string) error {
	return h.store.DeleteConversation(sid)
}
