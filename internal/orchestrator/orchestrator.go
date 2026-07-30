package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"ops-mate/internal/llm"
	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"
)

// Exec 执行器接口（供测试 stub 与真实 SSHExecutor 实现）。
type Exec interface {
	Exec(ctx context.Context, hostID, command string) (<-chan sshexec.Line, error)
}

// LLM AI 后端接口别名，便于替换。
type LLM = llm.LLMClient

// Session 一个对话会话。
type Session struct {
	ID      string
	HostID  string
	stateMu sync.Mutex
	state   string // "Idle"|"AwaitingApproval"|"Running"|"FeedingBack"
	history []llm.Message
	current *llm.CommandSuggestion
	ctx     context.Context
	cancel  context.CancelFunc
}

// Orchestrator 管理所有会话，依赖 store/llm/executor。
type Orchestrator struct {
	store       *store.Store
	LLM         LLM
	ExecutorFor func(hostID string) Exec
	sessionsMu  sync.Mutex
	sessions    map[string]*Session
	emit        func(sessionID, event string, data any)
}

func NewOrchestrator(st *store.Store) *Orchestrator {
	return &Orchestrator{store: st, sessions: map[string]*Session{}}
}

// SetEmitter 注入事件推送函数（App 层绑定 Wails EventsEmit）。
func (o *Orchestrator) SetEmitter(fn func(sessionID, event string, data any)) {
	o.emit = fn
}

func (o *Orchestrator) emitEvent(sid, event string, data any) {
	if o.emit != nil {
		o.emit(sid, event, data)
	}
}

func (o *Orchestrator) NewSession(hostID, title string) (string, error) {
	sid, err := o.store.NewConversation(hostID, title)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{ID: sid, HostID: hostID, state: "Idle", ctx: ctx, cancel: cancel}
	o.sessionsMu.Lock()
	o.sessions[sid] = s
	o.sessionsMu.Unlock()
	return sid, nil
}

func (o *Orchestrator) getSession(sid string) (*Session, error) {
	o.sessionsMu.Lock()
	defer o.sessionsMu.Unlock()
	s, ok := o.sessions[sid]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sid)
	}
	return s, nil
}

func (s *Session) setState(st string) {
	s.stateMu.Lock()
	s.state = st
	s.stateMu.Unlock()
}

func (s *Session) getState() string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.state
}

// SendMessage 用户发起/继续对话。
func (o *Orchestrator) SendMessage(sid, text string) error {
	s, err := o.getSession(sid)
	if err != nil {
		return err
	}
	if o.LLM == nil {
		o.emitEvent(sid, "ai:text", "AI 后端未配置，请到设置页配置")
		return nil
	}
	s.history = append(s.history, llm.Message{Role: llm.RoleUser, Content: text})
	o.store.AppendMessage(sid, "user", text, "")
	go o.runLLMTurn(s)
	return nil
}

// runLLMTurn 调一次 AI，处理其流式输出。
func (o *Orchestrator) runLLMTurn(s *Session) {
	ctx := s.ctx
	recall, _ := o.store.Recall(s.HostID, lastUserText(s.history))
	prompt := append([]llm.Message{}, s.history...)
	if len(recall.PastCommands) > 0 {
		prompt = append([]llm.Message{{
			Role: llm.RoleUser, Content: pastCommandsNote(recall.PastCommands),
		}}, prompt...)
	}
	ch, err := o.LLM.Chat(ctx, prompt)
	if err != nil {
		o.emitEvent(s.ID, "ai:text", "AI 后端不可用："+err.Error())
		s.setState("Idle")
		o.emitEvent(s.ID, "session:state", "Idle")
		return
	}
	var assistantText string
	for ck := range ch {
		if ck.Command != nil {
			s.current = ck.Command
			assessed := AssessRisk(ck.Command.Command)
			risk := ck.Command.Risk
			if assessed == "high" {
				risk = "high"
			}
			s.setState("AwaitingApproval")
			o.emitEvent(s.ID, "ai:command", map[string]any{
				"command":      ck.Command.Command,
				"why":          ck.Command.Why,
				"risk":         risk,
				"assessedRisk": assessed,
			})
			o.emitEvent(s.ID, "session:state", "AwaitingApproval")
			return // 暂停，等批准
		}
		assistantText += ck.Text
		o.emitEvent(s.ID, "ai:text", ck.Text)
	}
	if assistantText != "" {
		s.history = append(s.history, llm.Message{Role: llm.RoleAssistant, Content: assistantText})
		o.store.AppendMessage(s.ID, "assistant", assistantText, "")
	}
	s.setState("Idle")
	o.emitEvent(s.ID, "session:state", "Idle")
}

// ApproveCommand 用户批准（可传改后的命令）。
func (o *Orchestrator) ApproveCommand(sid, command string) error {
	s, err := o.getSession(sid)
	if err != nil {
		return err
	}
	if s.getState() != "AwaitingApproval" {
		return fmt.Errorf("当前状态不可批准")
	}
	go o.executeCommand(s, command)
	return nil
}

// RejectCommand 用户拒绝 → 让 AI 换方案。
func (o *Orchestrator) RejectCommand(sid string) error {
	s, err := o.getSession(sid)
	if err != nil {
		return err
	}
	s.history = append(s.history, llm.Message{Role: llm.RoleUser, Content: "用户拒绝了这条命令，请换一个方案。"})
	o.store.AppendMessage(sid, "user", "用户拒绝了这条命令，请换一个方案。", "")
	s.current = nil
	s.setState("Idle")
	o.emitEvent(sid, "session:state", "Idle")
	go o.runLLMTurn(s)
	return nil
}

func (o *Orchestrator) executeCommand(s *Session, command string) {
	s.setState("Running")
	o.emitEvent(s.ID, "session:state", "Running")
	ex := o.ExecutorFor(s.HostID)
	if ex == nil {
		o.emitEvent(s.ID, "ai:text", "执行器未配置")
		s.setState("Idle")
		o.emitEvent(s.ID, "session:state", "Idle")
		return
	}
	ch, err := ex.Exec(s.ctx, s.HostID, command)
	if err != nil {
		o.emitEvent(s.ID, "ai:text", "连接失败："+err.Error())
		s.setState("AwaitingApproval")
		o.emitEvent(s.ID, "session:state", "AwaitingApproval")
		return
	}
	var output string
	for ln := range ch {
		output += ln.Text + "\n"
		o.emitEvent(s.ID, "run:line", ln)
	}
	o.emitEvent(s.ID, "run:done", map[string]any{"exitCode": 0})
	o.store.SaveCommand(s.ID, command, 0, output)

	s.setState("FeedingBack")
	o.emitEvent(s.ID, "session:state", "FeedingBack")
	// 回灌执行结果
	s.history = append(s.history, llm.Message{Role: llm.RoleTool, Content: command, ToolResult: output})
	o.store.AppendMessage(s.ID, "tool", command, output)
	o.runLLMTurn(s)
}

// CancelRun 中止正在执行的命令。
func (o *Orchestrator) CancelRun(sid string) error {
	s, err := o.getSession(sid)
	if err != nil {
		return err
	}
	s.cancel()
	return nil
}

func lastUserText(msgs []llm.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == llm.RoleUser {
			return msgs[i].Content
		}
	}
	return ""
}

func pastCommandsNote(pcs []store.PastCommand) string {
	note := "该主机过去执行过的相关命令记录（供参考）：\n"
	for _, c := range pcs {
		note += fmt.Sprintf("- %s → %s\n", c.Command, truncate(c.Output, 200))
	}
	return note
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// === 供测试同步收集事件的 Collect 机制 ===

type Collector struct {
	Text    chan string
	Command chan map[string]any
	Line    chan sshexec.Line
	Done    chan map[string]any
	State   chan string
}

func (o *Orchestrator) Collect(sid string) *Collector {
	c := &Collector{
		Text: make(chan string, 64), Command: make(chan map[string]any, 8),
		Line: make(chan sshexec.Line, 64), Done: make(chan map[string]any, 4),
		State: make(chan string, 16),
	}
	o.emit = func(sessionID, event string, data any) {
		if sessionID != sid {
			return
		}
		switch event {
		case "ai:text":
			c.Text <- data.(string)
		case "ai:command":
			c.Command <- data.(map[string]any)
		case "run:line":
			c.Line <- data.(sshexec.Line)
		case "run:done":
			c.Done <- data.(map[string]any)
		case "session:state":
			c.State <- data.(string)
		}
	}
	return c
}
