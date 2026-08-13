package session

import (
	"context"
	"errors"
	"strings"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/einoagent/history"
	"ops-mate/internal/einoagent/testutil"
	"ops-mate/internal/skill"
	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"
	configstore "ops-mate/internal/store/config"
	convstore "ops-mate/internal/store/conversations"
	hoststore "ops-mate/internal/store/hosts"
)

// --- 测试基建 ---

type sessionFixture struct {
	t         *testing.T
	app       *store.DB
	convs     *convstore.ConvStore
	mgr       *SessionManager
	rec       *testutil.EmitRecorder
	modelImpl *testutil.ScriptedModel
	ex        *testutil.FakeExec
	hostID    string
}

func newSessionFixture(t *testing.T, responses []*schema.Message) *sessionFixture {
	t.Helper()
	app := testutil.OpenTempStore(t)
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

	rec := &testutil.EmitRecorder{}
	f := &sessionFixture{
		t: t, app: app, convs: convs, rec: rec,
		modelImpl: &testutil.ScriptedModel{Responses: responses},
		ex:        &testutil.FakeExec{Lines: []sshexec.Line{{Stream: "stdout", Text: "out"}}},
		hostID:    hostID,
	}
	f.mgr = NewSessionManager(app, cfg,
		func(hid string) sshexec.Exec { return f.ex },
		nil, // hostNameFor（测试不注入主机名）
		rec.Emit,
	)
	f.mgr.modelFactory = func(ctx context.Context, c configstore.AIConfig) (einomodel.ToolCallingChatModel, error) {
		return f.modelImpl, nil
	}
	return f
}

func (f *sessionFixture) waitState(sid, want string) {
	testutil.WaitFor(f.t, func() bool { return f.mgr.sessionState(sid) == want }, "state="+want)
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
		testutil.ToolCallResponse("top -bn1"),
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

	if cmds := f.ex.Commands(); len(cmds) != 1 || cmds[0] != "top -bn1" {
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
		testutil.ToolCallResponse("reboot"),
		schema.AssistantMessage("那换个方案", nil),
	})
	sid, _ := f.mgr.EnsureSession(f.hostID)
	_ = f.mgr.SendMessage(sid, "重启")
	f.waitState(sid, StateAwaitingApproval)

	if err := f.mgr.RejectCommand(sid); err != nil {
		t.Fatalf("RejectCommand: %v", err)
	}
	f.waitState(sid, StateIdle)

	if cmds := f.ex.Commands(); len(cmds) != 0 {
		t.Errorf("拒绝后不应执行命令: %v", cmds)
	}
	msgs, _ := f.convs.LoadMessages(sid)
	if msgs[len(msgs)-1].Content != "那换个方案" {
		t.Errorf("拒绝后未回灌换方案: %+v", msgs)
	}

	// 拒绝路径必须落库 tool 消息（配对 assistant tool_calls，否则历史回放被 API 拒绝）
	var toolMsg *convstore.Message
	for i := range msgs {
		if msgs[i].Role == history.RoleTool {
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
		testutil.ToolCallResponse("ls"),
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
	for _, e := range f.rec.SnapshotEvents() {
		if e == "ai:error" {
			found = true
		}
	}
	if !found {
		t.Error("期望推送 ai:error 事件")
	}
}

func TestSessionManager_ModelFactoryError(t *testing.T) {
	f := newSessionFixture(t, nil)
	f.mgr.modelFactory = func(ctx context.Context, c configstore.AIConfig) (einomodel.ToolCallingChatModel, error) {
		return nil, errors.New("unsupported provider")
	}
	sid, _ := f.mgr.EnsureSession(f.hostID)
	if err := f.mgr.SendMessage(sid, "hi"); err == nil {
		t.Error("模型构建失败应返回错误")
	}
}

func TestSessionManager_CancelRun(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{
		testutil.ToolCallResponse("sleep 30"),
		schema.AssistantMessage("done", nil),
	})
	// 阻塞式 executor：直到 ctx 取消
	blockEx := &testutil.BlockingExec{}
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

func TestBuildInput_TerminalContextInjected(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{schema.AssistantMessage("hi", nil)})
	f.mgr.SetTerminalContextResolver(func(hostID string) string {
		return "TERMINAL_CTX_" + hostID
	})
	sid, _ := f.mgr.EnsureSession(f.hostID)

	s, err := f.mgr.sessionFor(sid)
	if err != nil {
		t.Fatalf("sessionFor: %v", err)
	}
	input, err := f.mgr.buildInput(s, "继续")
	if err != nil {
		t.Fatalf("buildInput: %v", err)
	}
	// system 消息应包含终端上下文
	found := false
	for _, msg := range input {
		if msg.Role == schema.System && strings.Contains(msg.Content, "TERMINAL_CTX_"+f.hostID) {
			found = true
		}
	}
	if !found {
		t.Error("system 消息应包含终端上下文")
	}
}

func TestBuildInput_NoResolverNoContext(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{schema.AssistantMessage("hi", nil)})
	// 不注入 resolver
	sid, _ := f.mgr.EnsureSession(f.hostID)
	s, _ := f.mgr.sessionFor(sid)
	input, err := f.mgr.buildInput(s, "继续")
	if err != nil {
		t.Fatalf("buildInput: %v", err)
	}
	for _, msg := range input {
		if msg.Role == schema.System && strings.Contains(msg.Content, "终端最近输出") {
			t.Error("未注入 resolver 时不应输出终端上下文段落")
		}
	}
}

func TestBuildInput_SkillsCatalogInjected(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{schema.AssistantMessage("hi", nil)})
	f.mgr.SetSkillResolver(
		func() string { return "- nginx-check: 检查 Nginx" },
		func(name string) (*skill.Skill, error) { return nil, nil },
	)
	sid, _ := f.mgr.EnsureSession(f.hostID)
	s, _ := f.mgr.sessionFor(sid)
	input, err := f.mgr.buildInput(s, "继续")
	if err != nil {
		t.Fatalf("buildInput: %v", err)
	}
	found := false
	for _, msg := range input {
		if msg.Role == schema.System && strings.Contains(msg.Content, "nginx-check") {
			found = true
		}
	}
	if !found {
		t.Error("system 消息应含技能目录")
	}
}

func TestBuildInput_NoSkillsCatalog(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{schema.AssistantMessage("hi", nil)})
	// 不注入技能解析器
	sid, _ := f.mgr.EnsureSession(f.hostID)
	s, _ := f.mgr.sessionFor(sid)
	input, err := f.mgr.buildInput(s, "继续")
	if err != nil {
		t.Fatalf("buildInput: %v", err)
	}
	for _, msg := range input {
		if msg.Role == schema.System && strings.Contains(msg.Content, "已安装运维技能") {
			t.Error("未注入技能解析器时不应输出技能目录段落")
		}
	}
}

func TestSessionManager_ProtocolResolverInjected(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{schema.AssistantMessage("hi", nil)})
	resolved := ""
	f.mgr.SetProtocolResolver(func(hostID string) string {
		resolved = hostID
		return "winrm"
	})
	sid, _ := f.mgr.EnsureSession(f.hostID)
	if err := f.mgr.SendMessage(sid, "在吗"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	f.waitState(sid, StateIdle)
	if resolved != f.hostID {
		t.Errorf("ensureGraph 应调用协议解析器，得到 %q", resolved)
	}
}

func TestBuildInput_OSInjected(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{schema.AssistantMessage("hi", nil)})
	f.mgr.SetProtocolResolver(func(hostID string) string { return "winrm" })
	sid, _ := f.mgr.EnsureSession(f.hostID)
	s, err := f.mgr.sessionFor(sid)
	if err != nil { t.Fatalf("sessionFor: %v", err) }
	input, err := f.mgr.buildInput(s, "继续")
	if err != nil { t.Fatalf("buildInput: %v", err) }
	found := false
	for _, msg := range input {
		if msg.Role == schema.System && strings.Contains(msg.Content, "Windows 主机") {
			found = true
		}
	}
	if !found {
		t.Error("winrm 协议下 system 消息应含 Windows 主机描述")
	}
}

func TestSessionManager_PolicyResolverInjected(t *testing.T) {
	f := newSessionFixture(t, []*schema.Message{schema.AssistantMessage("hi", nil)})
	resolved := false
	f.mgr.SetApprovalPolicyResolver(func(hostID string) (bool, []string) {
		resolved = true
		return true, []string{"ls"}
	})
	sid, _ := f.mgr.EnsureSession(f.hostID)
	if err := f.mgr.SendMessage(sid, "在吗"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	f.waitState(sid, StateIdle)
	if !resolved {
		t.Error("ensureGraph 应调用策略解析器")
	}
}

func TestBuildInput_TruncationFirstNonSystemIsUser(t *testing.T) {
	// 回归：历史超 maxHistoryMessages 触发截断时，模型输入的第一条非 system 消息
	// 必须是 user（OpenAI/Anthropic 等 API 硬性校验，违反报 "first non-system
	// message should be user message"）。此前截断插入的占位消息是 assistant 角色
	// 且排在 user 之前，长会话时必现。
	f := newSessionFixture(t, []*schema.Message{schema.AssistantMessage("hi", nil)})
	sid, _ := f.mgr.EnsureSession(f.hostID)

	// 构造超过 maxHistoryMessages 条的历史（交错 user/assistant），且会话末尾
	// 是完整的 user → assistant(tool_call) → tool → assistant 轮次，贴近真实。
	tcJSON := `[{"id":"call_x","name":"execute_command","arguments":"{\"command\":\"ls\"}"}]`
	for i := 0; i < maxHistoryMessages+2; i++ {
		_ = f.convs.SaveMessage(convstore.Message{SessionID: sid, Role: history.RoleUser, Content: "查询"})
		_ = f.convs.SaveMessage(convstore.Message{SessionID: sid, Role: history.RoleAssistant, Content: "分析中"})
	}
	_ = f.convs.SaveMessage(convstore.Message{SessionID: sid, Role: history.RoleUser, Content: "看下目录"})
	_ = f.convs.SaveMessage(convstore.Message{SessionID: sid, Role: history.RoleAssistant, Content: "", ToolCalls: tcJSON})
	_ = f.convs.SaveMessage(convstore.Message{SessionID: sid, Role: history.RoleTool, Content: "file1", ToolCallID: "call_x", ToolName: "execute_command"})
	_ = f.convs.SaveMessage(convstore.Message{SessionID: sid, Role: history.RoleAssistant, Content: "完成"})

	s, err := f.mgr.sessionFor(sid)
	if err != nil {
		t.Fatalf("sessionFor: %v", err)
	}
	input, err := f.mgr.buildInput(s, "继续")
	if err != nil {
		t.Fatalf("buildInput: %v", err)
	}

	firstNonSystem := (*schema.Message)(nil)
	for _, msg := range input {
		if msg.Role != schema.System {
			firstNonSystem = msg
			break
		}
	}
	if firstNonSystem == nil {
		t.Fatal("输入中没有非 system 消息")
	}
	if firstNonSystem.Role != schema.User {
		t.Errorf("截断后第一条非 system 消息角色 = %v，want user（内容: %q）",
			firstNonSystem.Role, firstNonSystem.Content)
	}
}
