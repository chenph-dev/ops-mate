package session

import (
	"context"
	"fmt"
)

// ApproveCommand 批准命令（command 为用户最终确认的命令，可能编辑过）。
func (m *SessionManager) ApproveCommand(sid, command string) error {
	s, err := m.sessionFor(sid)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.state != stAwaitingApproval || s.approvalType != "command" {
		s.mu.Unlock()
		return fmt.Errorf("当前无待审批命令（state=%s type=%s）", s.state, s.approvalType)
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
	if s.state != stAwaitingApproval || s.approvalType != "command" {
		s.mu.Unlock()
		return fmt.Errorf("当前无待审批命令（state=%s type=%s）", s.state, s.approvalType)
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

// ApprovePlan 批准执行计划，恢复模型开始按计划逐步执行。
func (m *SessionManager) ApprovePlan(sid string) error {
	s, err := m.sessionFor(sid)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.state != stAwaitingApproval || s.approvalType != "plan" {
		s.mu.Unlock()
		return fmt.Errorf("当前无待审批计划（state=%s type=%s）", s.state, s.approvalType)
	}
	s.state = stRunning
	input := s.lastInput
	s.mu.Unlock()

	m.emitState(sid, StateRunning)
	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	go m.run(s, runCtx, input, true, "approved")
	return nil
}

// RejectPlan 拒绝执行计划，回灌模型重新规划或询问用户。
func (m *SessionManager) RejectPlan(sid string) error {
	s, err := m.sessionFor(sid)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.state != stAwaitingApproval || s.approvalType != "plan" {
		s.mu.Unlock()
		return fmt.Errorf("当前无待审批计划（state=%s type=%s）", s.state, s.approvalType)
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
