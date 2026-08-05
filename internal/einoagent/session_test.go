package einoagent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	configstore "ops-mate/internal/store/config"
	convstore "ops-mate/internal/store/conversations"
	hoststore "ops-mate/internal/store/hosts"
	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"
)

// --- 测试基建 ---

type sessionFixture struct {
	t         *testing.T
	app       *store.DB
	convs     *convstore.ConvStore
	mgr       *SessionManager
	rec       *emitRecorder
	modelImpl *scriptedModel
	ex        *fakeExec
	hostID    string
}

func newSessionFixture(t *testing.T, responses []*schema.Message) *sessionFixture {
	t.Helper()
	app := openTempStore(t)
	hosts := hoststore.NewHostsStore(app)
	convs := convstore.NewConvStore(app)
	cfg := configstore.NewConfigStore(app)
	// ensureGraph 要求 Provider 非空；modelFactory 已被测试覆写，provider 取值无关紧要。
	if err := cfg.SaveAIConfig(configstore.AIConfig{Provider: "openai", Model: "test-model"}); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	hostID, err := hosts.SaveHost(hoststore.HostInput{
		Name: "h", Addr: "1.1.1.1", Port: 22, User: "u",
		AuthType: "password", Secret: "x",
	})
	if err != nil {
		t.Fatalf("SaveHost: %v", err)
	}

	rec := &emitRecorder{}
	f := &sessionFixture{
		t: t, app: app, convs: convs, rec: rec,
		modelImpl: &scriptedModel{responses: responses},
		ex:        &fakeExec{lines: []sshexec.Line{{Stream: "stdout", Text: "out"}}},
		hostID:    hostID,
	}
	f.mgr = NewSessionManager(app, cfg,
		func(hid string) sshexec.Exec { return f.ex },
		rec.emit,
	)
	f.mgr.modelFactory = func(ctx context.Context, c configstore.AIConfig) (model.ToolCallingChatModel, error) {
		return f.modelImpl, nil
	}
	return f
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等待超时: %s", what)
}

func (f *sessionFixture) waitState(sid, want string) {
	waitFor(f.t, func() bool { return f.mgr.sessionState(sid) == want }, "state="+want)
}

// --- 用例 ---

func TestSessionManager_EnsureSessionLazyCreate(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{schema.AssistantMessage("hi", nil)})
	sid1, err := f.mgr.EnsureSession(f.hostID)
	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	sid2, _ := f.mgr.EnsureSession(f.hostID)
	if sid1 != sid2 {
		t.Error("同一主机重复 EnsureSession 应返回同一会话")
	}
}

func TestSessionManager_TextOnlyTurnPersists(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{schema.AssistantMessage("你好，有什么可以帮你", nil)})
	sid, _ := f.mgr.EnsureSession(f.hostID)

	if err := f.mgr.SendMessage(sid, "在吗"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	f.waitState(sid, StateIdle)

	msgs, _ := f.convs.LoadMessages(sid)
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[0].Content != "在吗" ||
		msgs[1].Role != "assistant" || msgs[1].Content != "你好，有什么可以帮你" {
		t.Errorf("消息落库错误: %+v", msgs)
	}
}

func TestSessionManager_ApprovalRoundTrip(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{
		toolCallResponse("top -bn1"),
		schema.AssistantMessage("是 go 进程占满", nil),
	})
	sid, _ := f.mgr.EnsureSession(f.hostID)

	if err := f.mgr.SendMessage(sid, "cpu 为什么高"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	f.waitState(sid, StateAwaitingApproval)

	if err := f.mgr.ApproveCommand(sid, "top -bn1"); err != nil {
		t.Fatalf("ApproveCommand: %v", err)
	}
	f.waitState(sid, StateIdle)

	if cmds := f.ex.commands(); len(cmds) != 1 || cmds[0] != "top -bn1" {
		t.Fatalf("未执行批准的命令: %v", cmds)
	}
	msgs, _ := f.convs.LoadMessages(sid)
	// user / assistant(tool_calls) / tool / assistant(final)
	if len(msgs) != 4 {
		t.Fatalf("期望 4 条消息，得到 %d: %+v", len(msgs), msgs)
	}
	if msgs[1].ToolCalls == "" {
		t.Error("assistant tool_calls 未落库")
	}
	if msgs[2].Role != "tool" || msgs[2].ToolCallID == "" {
		t.Errorf("tool 消息缺 tool_call_id: %+v", msgs[2])
	}
}

func TestSessionManager_RejectFlow(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{
		toolCallResponse("reboot"),
		schema.AssistantMessage("那换个方案", nil),
	})
	sid, _ := f.mgr.EnsureSession(f.hostID)
	_ = f.mgr.SendMessage(sid, "重启")
	f.waitState(sid, StateAwaitingApproval)

	if err := f.mgr.RejectCommand(sid); err != nil {
		t.Fatalf("RejectCommand: %v", err)
	}
	f.waitState(sid, StateIdle)

	if cmds := f.ex.commands(); len(cmds) != 0 {
		t.Errorf("拒绝后不应执行命令: %v", cmds)
	}
	msgs, _ := f.convs.LoadMessages(sid)
	if msgs[len(msgs)-1].Content != "那换个方案" {
		t.Errorf("拒绝后未回灌换方案: %+v", msgs)
	}

	// 拒绝路径必须落库 tool 消息（配对 assistant tool_calls，否则历史回放被 API 拒绝）
	var toolMsg *convstore.Message
	for i := range msgs {
		if msgs[i].Role == RoleTool {
			toolMsg = &msgs[i]
		}
	}
	if toolMsg == nil {
		t.Fatal("拒绝后应有 tool 消息落库")
	}
	if toolMsg.ToolCallID == "" || !strings.Contains(toolMsg.Content, "拒绝") {
		t.Errorf("拒绝 tool 消息缺配对或内容错误: %+v", *toolMsg)
	}
	if toolMsg.ApprovalStatus != "rejected" {
		t.Errorf("拒绝 tool 消息 approval_status 应为 rejected: %+v", *toolMsg)
	}
}

func TestSessionManager_SendDuringApprovalRejected(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{
		toolCallResponse("ls"),
		schema.AssistantMessage("ok", nil),
	})
	sid, _ := f.mgr.EnsureSession(f.hostID)
	_ = f.mgr.SendMessage(sid, "看看")
	f.waitState(sid, StateAwaitingApproval)

	if err := f.mgr.SendMessage(sid, "插一句"); err == nil {
		t.Error("审批中发送新消息应被拒绝")
	}
}

func TestSessionManager_MissingExecutor(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{schema.AssistantMessage("x", nil)})
	f.mgr.executorFor = func(hid string) sshexec.Exec { return nil }
	sid, _ := f.mgr.EnsureSession(f.hostID)

	err := f.mgr.SendMessage(sid, "hi")
	if err == nil {
		t.Error("执行器不可用应返回错误")
	}
	found := false
	f.rec.mu.Lock()
	for _, e := range f.rec.events {
		if e == "ai:error" {
			found = true
		}
	}
	f.rec.mu.Unlock()
	if !found {
		t.Error("期望推送 ai:error 事件")
	}
}

func TestSessionManager_ModelFactoryError(t *testing.T) {
	f := newSessionFixture(t, nil)
	f.mgr.modelFactory = func(ctx context.Context, c configstore.AIConfig) (model.ToolCallingChatModel, error) {
		return nil, errors.New("unsupported provider")
	}
	sid, _ := f.mgr.EnsureSession(f.hostID)
	if err := f.mgr.SendMessage(sid, "hi"); err == nil {
		t.Error("模型构建失败应返回错误")
	}
}

func TestSessionManager_CancelRun(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{
		toolCallResponse("sleep 30"),
		schema.AssistantMessage("done", nil),
	})
	// 阻塞式 executor：直到 ctx 取消
	blockEx := &blockingExec{}
	f.mgr.executorFor = func(hid string) sshexec.Exec { return blockEx }
	sid, _ := f.mgr.EnsureSession(f.hostID)
	_ = f.mgr.SendMessage(sid, "跑个长命令")
	f.waitState(sid, StateAwaitingApproval)
	_ = f.mgr.ApproveCommand(sid, "sleep 30")
	f.waitState(sid, StateRunning)

	if err := f.mgr.CancelRun(sid); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}
	f.waitState(sid, StateIdle)
}

// blockingExec 阻塞到 ctx 取消。
type blockingExec struct{}

func (b *blockingExec) Exec(ctx context.Context, command string) (<-chan sshexec.Line, error) {
	ch := make(chan sshexec.Line)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}
