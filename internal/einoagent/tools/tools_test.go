package tools

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/einoagent/testutil"
	"ops-mate/internal/sshexec"
	convstore "ops-mate/internal/store/conversations"
	hoststore "ops-mate/internal/store/hosts"
)

func TestTruncateForModel(t *testing.T) {
	small := strings.Repeat("a", 100)
	if got := truncateForModel(small); got != small {
		t.Error("小输出不应被截断")
	}
	big := strings.Repeat("b", 20*1024)
	got := truncateForModel(big)
	if len(got) > modelOutputLimit+200 {
		t.Errorf("截断后仍过长: %d", len(got))
	}
	if !strings.Contains(got, "省略") {
		t.Error("截断应包含省略标记")
	}
	if !strings.HasPrefix(got, "bbb") || !strings.HasSuffix(got, "bbb") {
		t.Error("截断应保留头尾")
	}
}

func TestTruncateForDisplay(t *testing.T) {
	big := strings.Repeat("c", 100*1024)
	got := truncateForDisplay(big)
	if len(got) > displayOutputLimit+100 {
		t.Errorf("展示截断后过长: %d", len(got))
	}
	if !strings.Contains(got, "截断") {
		t.Error("展示截断应包含标记")
	}
}

func TestSSHTool_FirstCallAlwaysInterrupts(t *testing.T) {
	rec := &testutil.EmitRecorder{}
	ex := &testutil.FakeExec{Lines: []sshexec.Line{{Stream: "stdout", Text: "out"}}}
	tool := NewSSHTool("s1", ex, rec.Emit, nil, NewToolCallHolder())

	// 低风险命令也必须中断（全量审批）
	_, err := tool.InvokableRun(context.Background(), `{"command":"ls -la","why":"看看文件"}`)
	if err == nil {
		t.Fatal("期望首次调用返回中断错误")
	}
	if len(ex.Commands()) != 0 {
		t.Error("中断前不应执行命令")
	}
	// 应已推送 ai:command 事件
	found := false
	for _, e := range rec.SnapshotEvents() {
		if e == "ai:command" {
			found = true
		}
	}
	if !found {
		t.Error("期望推送 ai:command 事件")
	}
}

func TestSSHTool_HighRiskMarked(t *testing.T) {
	var gotInfo commandInfo
	rec := &testutil.EmitRecorder{}
	emit := func(sid, event string, data any) {
		rec.Emit(sid, event, data)
		if event == "ai:command" {
			gotInfo, _ = data.(commandInfo)
		}
	}
	tool := NewSSHTool("s1", &testutil.FakeExec{}, emit, nil, NewToolCallHolder())
	_, _ = tool.InvokableRun(context.Background(), `{"command":"rm -rf /","why":"清理"}`)
	if gotInfo.AssessedRisk != "high" {
		t.Errorf("高风险命令 assessedRisk = %q，want high", gotInfo.AssessedRisk)
	}
}

func TestSSHTool_ExecutePersistsAndEmits(t *testing.T) {
	app := testutil.OpenTempStore(t)
	hosts := hoststore.NewHostsStore(app)
	convs := convstore.NewConvStore(app)
	hostID, _ := hosts.SaveHost(hoststore.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	sid, _ := convs.NewConversation(hostID, "t")

	ex := &testutil.FakeExec{Lines: []sshexec.Line{
		{Stream: "stdout", Text: "file1"},
		{Stream: "stdout", Text: "file2"},
		{Stream: "exit", Text: "exit_code=2"},
	}}
	rec := &testutil.EmitRecorder{}
	holder := NewToolCallHolder()
	holder.Add(&schema.ToolCall{ID: "call_9", Function: schema.FunctionCall{Name: "execute_command"}})
	tool := NewSSHTool(sid, ex, rec.Emit, convs, holder)

	result, err := tool.execute(context.Background(), "ls")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(result, "file1") || !strings.Contains(result, "exit_code=2") {
		t.Errorf("模型回灌结果缺内容: %q", result)
	}

	msgs, _ := convs.LoadMessages(sid)
	if len(msgs) != 1 || msgs[0].Role != "tool" ||
		msgs[0].ToolCallID != "call_9" || msgs[0].ToolName != "execute_command" ||
		msgs[0].ApprovalStatus != "approved" ||
		!strings.Contains(msgs[0].Content, "file1") {
		t.Errorf("tool 消息落库错误: %+v", msgs)
	}

	// 执行前后应分别推送 run:start 与 run:result 事件
	events := rec.SnapshotEvents()
	var sawStart, sawResult bool
	for _, e := range events {
		switch e {
		case "run:start":
			sawStart = true
		case "run:result":
			sawResult = true
		}
	}
	if !sawStart || !sawResult {
		t.Errorf("期望 run:start 与 run:result 事件，得到 %v", events)
	}
}

func TestSSHTool_ExecErrorFedBackAsText(t *testing.T) {
	ex := &testutil.FakeExec{Err: context.DeadlineExceeded}
	tool := NewSSHTool("s1", ex, (&testutil.EmitRecorder{}).Emit, nil, NewToolCallHolder())
	result, err := tool.execute(context.Background(), "sleep 999")
	if err != nil {
		t.Fatalf("execute 不应返回 error（应回灌文本）: %v", err)
	}
	if !strings.Contains(result, "执行失败") {
		t.Errorf("执行失败应回灌提示文本: %q", result)
	}
}

func TestSSHTool_BadArgsReturnsTextNotError(t *testing.T) {
	tool := NewSSHTool("s1", &testutil.FakeExec{}, (&testutil.EmitRecorder{}).Emit, nil, NewToolCallHolder())
	got, err := tool.InvokableRun(context.Background(), "{bad json")
	if err != nil {
		t.Fatalf("坏 JSON 不应返回 error: %v", err)
	}
	if !strings.Contains(got, "参数解析失败") {
		t.Errorf("期望参数解析失败提示，得到 %q", got)
	}
}

func TestTruncateForModel_RuneSafe(t *testing.T) {
	big := strings.Repeat("中", 10000) // 30000 bytes
	got := truncateForModel(big)
	if !utf8.ValidString(got) {
		t.Error("截断结果不是合法 UTF-8（切断了多字节字符）")
	}
	if !strings.Contains(got, "省略") {
		t.Error("应包含省略标记")
	}
}

func TestTruncateForDisplay_RuneSafe(t *testing.T) {
	big := strings.Repeat("文", 40000) // 120000 bytes > 64KB
	got := truncateForDisplay(big)
	if !utf8.ValidString(got) {
		t.Error("展示截断结果不是合法 UTF-8")
	}
}

// 回归：run:result emit 时，该命令的 tool 消息必须已落库。
// 修复前 run:result 在 saveToolMessage 之前 emit，前端收到事件触发 resync 时
// tool 消息还不在库中，assistant 提议与其结果的相邻配对缺失，
// 审批状态会被误判回"待审批"。
func TestSSHTool_RunResultEmittedAfterToolMessagePersisted(t *testing.T) {
	app := testutil.OpenTempStore(t)
	hosts := hoststore.NewHostsStore(app)
	convs := convstore.NewConvStore(app)
	hostID, _ := hosts.SaveHost(hoststore.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	sid, _ := convs.NewConversation(hostID, "t")

	ex := &testutil.FakeExec{Lines: []sshexec.Line{{Stream: "stdout", Text: "file1"}}}
	holder := NewToolCallHolder()
	holder.Add(&schema.ToolCall{ID: "call_1", Function: schema.FunctionCall{Name: "execute_command"}})

	persistedAtResult := false
	emit := func(_ string, event string, _ any) {
		if event == "run:result" {
			msgs, err := convs.LoadMessages(sid)
			if err != nil {
				t.Errorf("LoadMessages: %v", err)
				return
			}
			for _, m := range msgs {
				if m.Role == "tool" && m.ToolCallID == "call_1" {
					persistedAtResult = true
				}
			}
		}
	}
	tool := NewSSHTool(sid, ex, emit, convs, holder)
	if _, err := tool.execute(context.Background(), "ls"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !persistedAtResult {
		t.Error("run:result emit 时 tool 消息应已落库（修复前在落库之前 emit）")
	}
}

// 注：rejected 落库分支与空批准数据守卫都需要图执行上下文（einotool.Resume）才能触发，
// 工具层无法直接测试。空批准数据守卫由 graph 包测试
// （TestApprovalFlow_ApproveWithEmptyCommandGuarded）覆盖；
// rejected 路径的 DB 落库由 session 包测试（TestSessionManager_RejectFlow）覆盖。
