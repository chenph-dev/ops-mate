package session

import (
	"fmt"

	"ops-mate/internal/einoagent/checkpoint"
	agenttools "ops-mate/internal/einoagent/tools"
)

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

// emitState 推送会话状态事件。
func (m *SessionManager) emitState(sid, state string) {
	if m.emit != nil {
		m.emit(sid, "session:state", state)
	}
}

// emitError 推送错误事件。
func (m *SessionManager) emitError(sid, message string) {
	if m.emit != nil {
		m.emit(sid, "ai:error", map[string]any{"message": message})
	}
}
