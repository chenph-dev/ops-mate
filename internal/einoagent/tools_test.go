package einoagent

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"
	convstore "ops-mate/internal/store/conversations"
	hoststore "ops-mate/internal/store/hosts"
)

// fakeExec 实现 sshexec.Exec。
type fakeExec struct {
	mu      sync.Mutex
	lines   []sshexec.Line
	err     error
	gotCmds []string
}

func (f *fakeExec) Exec(ctx context.Context, command string) (<-chan sshexec.Line, error) {
	f.mu.Lock()
	f.gotCmds = append(f.gotCmds, command)
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	ch := make(chan sshexec.Line, len(f.lines))
	for _, ln := range f.lines {
		ch <- ln
	}
	close(ch)
	return ch, nil
}

func (f *fakeExec) commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.gotCmds...)
}

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
	rec := &emitRecorder{}
	ex := &fakeExec{lines: []sshexec.Line{{Stream: "stdout", Text: "out"}}}
	tool := NewSSHTool("s1", ex, rec.emit, nil, newToolCallHolder())

	// 低风险命令也必须中断（全量审批）
	_, err := tool.InvokableRun(context.Background(), `{"command":"ls -la","why":"看看文件"}`)
	if err == nil {
		t.Fatal("期望首次调用返回中断错误")
	}
	if len(ex.commands()) != 0 {
		t.Error("中断前不应执行命令")
	}
	// 应已推送 ai:command 事件
	found := false
	rec.mu.Lock()
	for _, e := range rec.events {
		if e == "ai:command" {
			found = true
		}
	}
	rec.mu.Unlock()
	if !found {
		t.Error("期望推送 ai:command 事件")
	}
}

func TestSSHTool_HighRiskMarked(t *testing.T) {
	var gotInfo commandInfo
	rec := &emitRecorder{}
	emit := func(sid, event string, data any) {
		rec.emit(sid, event, data)
		if event == "ai:command" {
			gotInfo, _ = data.(commandInfo)
		}
	}
	tool := NewSSHTool("s1", &fakeExec{}, emit, nil, newToolCallHolder())
	_, _ = tool.InvokableRun(context.Background(), `{"command":"rm -rf /","why":"清理"}`)
	if gotInfo.AssessedRisk != "high" {
		t.Errorf("高风险命令 assessedRisk = %q，want high", gotInfo.AssessedRisk)
	}
}

func openTempStore(t *testing.T) *store.DB {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	app, err := store.Open()
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := app.GORM().DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	return app
}

func TestSSHTool_ExecutePersistsAndEmits(t *testing.T) {
	app := openTempStore(t)
	hosts := hoststore.NewHostsStore(app)
	convs := convstore.NewConvStore(app)
	hostID, _ := hosts.SaveHost(hoststore.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	sid, _ := convs.NewConversation(hostID, "t")

	ex := &fakeExec{lines: []sshexec.Line{
		{Stream: "stdout", Text: "file1"},
		{Stream: "stdout", Text: "file2"},
		{Stream: "exit", Text: "exit_code=2"},
	}}
	rec := &emitRecorder{}
	holder := newToolCallHolder()
	holder.Set(&schema.ToolCall{ID: "call_9", Function: schema.FunctionCall{Name: "execute_command"}})
	tool := NewSSHTool(sid, ex, rec.emit, convs, holder)

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
		!strings.Contains(msgs[0].Content, "file1") {
		t.Errorf("tool 消息落库错误: %+v", msgs)
	}
}

func TestSSHTool_ExecErrorFedBackAsText(t *testing.T) {
	ex := &fakeExec{err: context.DeadlineExceeded}
	tool := NewSSHTool("s1", ex, (&emitRecorder{}).emit, nil, newToolCallHolder())
	result, err := tool.execute(context.Background(), "sleep 999")
	if err != nil {
		t.Fatalf("execute 不应返回 error（应回灌文本）: %v", err)
	}
	if !strings.Contains(result, "执行失败") {
		t.Errorf("执行失败应回灌提示文本: %q", result)
	}
}
