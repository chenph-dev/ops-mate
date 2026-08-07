package session

import (
	"context"
	"fmt"

	"ops-mate/internal/einoagent/history"
	convstore "ops-mate/internal/store/conversations"
)

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
