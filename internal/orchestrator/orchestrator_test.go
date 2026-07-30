package orchestrator

import (
	"context"
	"testing"

	"ops-mate/internal/llm"
	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"
)

// scriptLLM 按脚本依次返回 Chunk 流。
type scriptLLM struct {
	scripts [][]llm.Chunk
	calls   int
}

func (s *scriptLLM) Chat(ctx context.Context, msgs []llm.Message) (<-chan llm.Chunk, error) {
	ch := make(chan llm.Chunk, 8)
	go func() {
		defer close(ch)
		idx := s.calls
		s.calls++
		if idx < len(s.scripts) {
			for _, c := range s.scripts[idx] {
				ch <- c
			}
		}
	}()
	return ch, nil
}

// stubExecutor 同步返回固定行。
type stubExecutor struct {
	lines []sshexec.Line
}

func (e *stubExecutor) Exec(ctx context.Context, command string) (<-chan sshexec.Line, error) {
	ch := make(chan sshexec.Line, 8)
	go func() {
		defer close(ch)
		for _, l := range e.lines {
			ch <- l
		}
	}()
	return ch, nil
}

func newTestOrchestrator(t *testing.T) (*store.Store, *Orchestrator) {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	st, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return st, NewOrchestrator(st)
}

func TestStateMachine_ApproveExecuteFeedback(t *testing.T) {
	st, o := newTestOrchestrator(t)
	defer st.DB.Close()

	hostID, _ := st.SaveHost(store.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	o.LLM = &scriptLLM{
		scripts: [][]llm.Chunk{
			{llm.Chunk{Text: "我看看"}, llm.Chunk{Command: &llm.CommandSuggestion{Command: "top -bn1", Why: "查 CPU", Risk: "low"}}},
			{llm.Chunk{Text: "是 go 进程占满"}},
		},
	}
	o.ExecutorFor = func(hostID string) Exec {
		return &stubExecutor{lines: []sshexec.Line{{Stream: "stdout", Text: "go 99%"}}}
	}

	sid, err := o.NewSession(hostID, "cpu 高")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	coll := o.Collect(sid)

	// 1) 用户发消息 → AI 提议命令（AwaitingApproval）
	o.SendMessage(sid, "cpu 高")
	<-coll.Text // 消费 "我看看"
	cmd := <-coll.Command
	if cmd["command"] != "top -bn1" {
		t.Fatalf("期望提议 top -bn1，得到 %q", cmd["command"])
	}
	if <-coll.State != "AwaitingApproval" {
		t.Fatal("状态应为 AwaitingApproval")
	}

	// 2) 批准 → 执行 → 输出 → 回灌 → AI 给结论（Idle）
	o.ApproveCommand(sid, cmd["command"].(string))
	if <-coll.State != "Running" {
		t.Fatal("状态应为 Running")
	}
	line := <-coll.Line
	if line.Text != "go 99%" {
		t.Fatalf("执行输出 = %q", line.Text)
	}
	<-coll.Done
	if <-coll.State != "FeedingBack" {
		t.Fatal("状态应为 FeedingBack")
	}
	finalText := <-coll.Text
	if finalText != "是 go 进程占满" {
		t.Fatalf("结论 = %q", finalText)
	}
	if <-coll.State != "Idle" {
		t.Fatal("状态应收回 Idle")
	}
}

func TestStateMachine_RejectAsksAIForAlternative(t *testing.T) {
	st, o := newTestOrchestrator(t)
	defer st.DB.Close()
	hostID, _ := st.SaveHost(store.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	o.LLM = &scriptLLM{
		scripts: [][]llm.Chunk{
			{llm.Chunk{Command: &llm.CommandSuggestion{Command: "rm -rf /tmp/x", Why: "清理", Risk: "high"}}},
			{llm.Chunk{Command: &llm.CommandSuggestion{Command: "du -sh /tmp", Why: "看占用", Risk: "low"}}},
		},
	}
	o.ExecutorFor = func(hostID string) Exec { return &stubExecutor{} }

	sid, _ := o.NewSession(hostID, "清理")
	coll := o.Collect(sid)
	o.SendMessage(sid, "清理 tmp")
	cmd := <-coll.Command
	if cmd["command"] != "rm -rf /tmp/x" {
		t.Fatalf("首条命令 = %q", cmd["command"])
	}
	<-coll.State // AwaitingApproval
	o.RejectCommand(sid)
	<-coll.State // Idle（拒绝后）
	cmd2 := <-coll.Command
	if cmd2["command"] != "du -sh /tmp" {
		t.Fatalf("拒绝后应换方案，得到 %q", cmd2["command"])
	}
}

func TestStateMachine_DangerCommandFlagged(t *testing.T) {
	st, o := newTestOrchestrator(t)
	defer st.DB.Close()
	hostID, _ := st.SaveHost(store.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	o.LLM = &scriptLLM{
		scripts: [][]llm.Chunk{
			{llm.Chunk{Command: &llm.CommandSuggestion{Command: "rm -rf /", Why: "x", Risk: "high"}}},
		},
	}
	o.ExecutorFor = func(hostID string) Exec { return &stubExecutor{} }
	sid, _ := o.NewSession(hostID, "x")
	coll := o.Collect(sid)
	o.SendMessage(sid, "删根目录")
	cmd := <-coll.Command
	if cmd["assessedRisk"] != "high" {
		t.Fatalf("危险命令应标红，得到 %q", cmd["assessedRisk"])
	}
}
