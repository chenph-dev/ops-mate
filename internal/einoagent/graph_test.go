package einoagent

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// scriptedModel 按顺序返回预设回复，实现 model.ToolCallingChatModel。
type scriptedModel struct {
	responses []*schema.Message
	calls     int
}

func (m *scriptedModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if m.calls >= len(m.responses) {
		return schema.AssistantMessage("（无更多预设回复）", nil), nil
	}
	r := m.responses[m.calls]
	m.calls++
	return r, nil
}

func (m *scriptedModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, _ := m.Generate(ctx, input, opts...)
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *scriptedModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// echoTool 测试用普通工具（不中断），记录收到的参数。
type echoTool struct {
	lastArgs string
}

func (t *echoTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "echo_tool",
		Desc: "回声工具",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"text": {Type: schema.String, Desc: "文本", Required: true},
		}),
	}, nil
}

func (t *echoTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	t.lastArgs = argsJSON
	return "echo:" + argsJSON, nil
}

func buildTestGraph(t *testing.T, chatModel model.ToolCallingChatModel, tools []tool.BaseTool) (compose.Runnable[[]*schema.Message, []*schema.Message], *memCheckpointStore) {
	t.Helper()
	ctx := context.Background()
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: tools, ExecuteSequentially: true,
	})
	if err != nil {
		t.Fatalf("NewToolNode: %v", err)
	}
	ckpt := newMemCheckpointStore()
	g, err := BuildAgentGraph(ctx, chatModel, toolsNode, ckpt)
	if err != nil {
		t.Fatalf("BuildAgentGraph: %v", err)
	}
	return g, ckpt
}

func TestBuildAgentGraph_NoToolCalls_EndsAtModel(t *testing.T) {
	m := &scriptedModel{responses: []*schema.Message{
		schema.AssistantMessage("直接回答", nil),
	}}
	et := &echoTool{}
	g, _ := buildTestGraph(t, m, []tool.BaseTool{et})

	out, err := g.Invoke(context.Background(),
		[]*schema.Message{schema.UserMessage("你好")},
		compose.WithCheckPointID("t1"))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("期望非空输出")
	}
	last := out[len(out)-1]
	if last.Content != "直接回答" {
		t.Errorf("末条消息 = %q，want 直接回答", last.Content)
	}
	if et.lastArgs != "" {
		t.Errorf("无 tool call 时不应执行工具，得到 %q", et.lastArgs)
	}
	if m.calls != 1 {
		t.Errorf("模型应只被调用 1 次，实际 %d", m.calls)
	}
}

func TestBuildAgentGraph_ToolCallRoutesToToolsAndLoops(t *testing.T) {
	m := &scriptedModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "c1", Type: "function",
			Function: schema.FunctionCall{Name: "echo_tool", Arguments: `{"text":"hi"}`},
		}}),
		schema.AssistantMessage("最终结论", nil),
	}}
	et := &echoTool{}
	g, _ := buildTestGraph(t, m, []tool.BaseTool{et})

	out, err := g.Invoke(context.Background(),
		[]*schema.Message{schema.UserMessage("跑一下")},
		compose.WithCheckPointID("t2"))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if et.lastArgs != `{"text":"hi"}` {
		t.Errorf("工具未收到参数: %q", et.lastArgs)
	}
	last := out[len(out)-1]
	if last.Content != "最终结论" {
		t.Errorf("回灌后末条消息 = %q，want 最终结论", last.Content)
	}
	if m.calls != 2 {
		t.Errorf("模型应被调用 2 次（提议+总结），实际 %d", m.calls)
	}
}
