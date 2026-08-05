// Package testutil 提供 einoagent 子包测试共享的 fixture。
package testutil

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"
)

// EmitRecorder 按序记录 emit 事件（ai:text 增量提取到 Deltas，ai:command 提取到 Commands）。
type EmitRecorder struct {
	mu       sync.Mutex
	Events   []string
	Deltas   []string
	Commands []string
}

// Emit 实现 emit 回调签名。
func (r *EmitRecorder) Emit(_sessionID, event string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Events = append(r.Events, event)
	if event == "ai:text" {
		if m, ok := data.(map[string]any); ok {
			if d, ok := m["delta"].(string); ok {
				r.Deltas = append(r.Deltas, d)
			}
		}
	}
	if event == "ai:command" {
		// data 为 tools.commandInfo（未导出类型），用反射提取 Command 字段。
		if v := reflect.ValueOf(data); v.IsValid() && v.Kind() == reflect.Struct {
			if f := v.FieldByName("Command"); f.IsValid() && f.Kind() == reflect.String {
				r.Commands = append(r.Commands, f.String())
			}
		}
	}
}

// SnapshotEvents 返回事件名列表副本（线程安全）。
func (r *EmitRecorder) SnapshotEvents() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.Events...)
}

// SnapshotCommands 返回 ai:command 的 command 列表副本（线程安全）。
func (r *EmitRecorder) SnapshotCommands() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.Commands...)
}

// FakeExec 实现 sshexec.Exec，返回预设行。
type FakeExec struct {
	mu      sync.Mutex
	Lines   []sshexec.Line
	Err     error
	gotCmds []string
}

func (f *FakeExec) Exec(_ context.Context, command string) (<-chan sshexec.Line, error) {
	f.mu.Lock()
	f.gotCmds = append(f.gotCmds, command)
	f.mu.Unlock()
	if f.Err != nil {
		return nil, f.Err
	}
	ch := make(chan sshexec.Line, len(f.Lines))
	for _, ln := range f.Lines {
		ch <- ln
	}
	close(ch)
	return ch, nil
}

// Commands 返回已收到的命令列表副本。
func (f *FakeExec) Commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.gotCmds...)
}

// ScriptedModel 按顺序返回预设回复，实现 einomodel.ToolCallingChatModel。
type ScriptedModel struct {
	Responses []*schema.Message
	Calls     int
	Inputs    [][]*schema.Message
}

func (m *ScriptedModel) Generate(_ context.Context, input []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	if m.Calls >= len(m.Responses) {
		return schema.AssistantMessage("（无更多预设回复）", nil), nil
	}
	m.Inputs = append(m.Inputs, input)
	r := m.Responses[m.Calls]
	m.Calls++
	return r, nil
}

func (m *ScriptedModel) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, _ := m.Generate(ctx, input, opts...)
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *ScriptedModel) WithTools(_ []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	return m, nil
}

// OpenTempStore 打开一个临时 DB（自动清理）。
func OpenTempStore(t *testing.T) *store.DB {
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

// ToolCallResponse 构造一条带 execute_command tool_call 的 assistant 回复。
func ToolCallResponse(cmd string) *schema.Message {
	return schema.AssistantMessage("", []schema.ToolCall{{
		ID: "call_1", Type: "function",
		Function: schema.FunctionCall{
			Name:      "execute_command",
			Arguments: `{"command":"` + cmd + `","why":"诊断"}`,
		},
	}})
}

// WaitFor 轮询等待条件成立（超时则 Fail）。
func WaitFor(t *testing.T, cond func() bool, what string) {
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

// BlockingExec 阻塞到 ctx 取消。
type BlockingExec struct{}

func (b *BlockingExec) Exec(ctx context.Context, _ string) (<-chan sshexec.Line, error) {
	ch := make(chan sshexec.Line)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}
