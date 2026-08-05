package model

import (
	"context"
	"errors"
	"sync"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/einoagent/testutil"
)

// fakeStreamModel 实现 einomodel.ToolCallingChatModel，Stream 返回预设 chunks。
type fakeStreamModel struct {
	mu     sync.Mutex
	chunks []*schema.Message
	err    error
	tools  []*schema.ToolInfo
}

func (f *fakeStreamModel) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	return nil, errors.New("测试中不应直接调用 fake 的 Generate")
}

func (f *fakeStreamModel) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return schema.StreamReaderFromArray(f.chunks), nil
}

func (f *fakeStreamModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tools = tools
	return f, nil
}

func TestStreamingChatModel_EmitsDeltasAndAccumulates(t *testing.T) {
	base := &fakeStreamModel{chunks: []*schema.Message{
		{Role: schema.Assistant, Content: "你"},
		{Role: schema.Assistant, Content: "好"},
	}}
	rec := &testutil.EmitRecorder{}
	var assistantGot *schema.Message
	w := NewStreamingChatModel(base, "s1", rec.Emit, func(m *schema.Message) { assistantGot = m })

	got, err := w.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got.Content != "你好" {
		t.Errorf("累积内容 = %q，want 你好", got.Content)
	}
	if len(rec.Deltas) != 2 || rec.Deltas[0] != "你" || rec.Deltas[1] != "好" {
		t.Errorf("ai:text 增量序列错误: %v", rec.Deltas)
	}
	if assistantGot == nil || assistantGot.Content != "你好" {
		t.Errorf("onAssistant 回调未收到完整消息: %+v", assistantGot)
	}
}

func TestStreamingChatModel_ToolCallChunksNotEmitted(t *testing.T) {
	base := &fakeStreamModel{chunks: []*schema.Message{
		{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{
			ID: "c1", Type: "function",
			Function: schema.FunctionCall{Name: "execute_command", Arguments: `{"command":"ls"}`},
		}}},
	}}
	rec := &testutil.EmitRecorder{}
	w := NewStreamingChatModel(base, "s1", rec.Emit, nil)

	got, err := w.Generate(context.Background(), nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].ID != "c1" {
		t.Errorf("tool calls 累积错误: %+v", got.ToolCalls)
	}
	if len(rec.Deltas) != 0 {
		t.Errorf("tool call chunks 不应发 ai:text，得到 %v", rec.Deltas)
	}
}

func TestStreamingChatModel_NilChunkSkipped(t *testing.T) {
	base := &fakeStreamModel{chunks: []*schema.Message{
		{Role: schema.Assistant, Content: "a"},
		nil,
		{Role: schema.Assistant, Content: "b"},
	}}
	rec := &testutil.EmitRecorder{}
	w := NewStreamingChatModel(base, "s1", rec.Emit, nil)
	got, err := w.Generate(context.Background(), nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got.Content != "ab" {
		t.Errorf("nil chunk 应被跳过且内容正常累积，得到 %q", got.Content)
	}
}

func TestStreamingChatModel_StreamPassthrough(t *testing.T) {
	base := &fakeStreamModel{chunks: []*schema.Message{{Role: schema.Assistant, Content: "x"}}}
	w := NewStreamingChatModel(base, "s1", nil, nil)
	sr, err := w.Stream(context.Background(), nil)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer sr.Close()
	m, err := sr.Recv()
	if err != nil || m.Content != "x" {
		t.Errorf("Stream 透传错误: %+v, %v", m, err)
	}
}

func TestStreamingChatModel_WithToolsDelegates(t *testing.T) {
	base := &fakeStreamModel{}
	w := NewStreamingChatModel(base, "s1", nil, nil)
	infos := []*schema.ToolInfo{{Name: "execute_command"}}
	w2, err := w.WithTools(infos)
	if err != nil {
		t.Fatalf("WithTools: %v", err)
	}
	if w2 == w {
		t.Error("WithTools 应返回新的包装实例")
	}
	if len(base.tools) != 1 || base.tools[0].Name != "execute_command" {
		t.Errorf("WithTools 未委托给底层模型: %+v", base.tools)
	}
}

func TestStreamingChatModel_StreamErrorPropagates(t *testing.T) {
	base := &fakeStreamModel{err: errors.New("401 unauthorized")}
	w := NewStreamingChatModel(base, "s1", nil, nil)
	if _, err := w.Generate(context.Background(), nil); err == nil {
		t.Error("期望底层 Stream 错误向上传播")
	}
}
