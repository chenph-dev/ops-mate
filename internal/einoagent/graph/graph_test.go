package graph

import (
	"context"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/einoagent/checkpoint"
	"ops-mate/internal/einoagent/testutil"
)

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

func (t *echoTool) InvokableRun(ctx context.Context, argsJSON string, opts ...einotool.Option) (string, error) {
	t.lastArgs = argsJSON
	return "echo:" + argsJSON, nil
}

func buildTestGraph(t *testing.T, chatModel einomodel.ToolCallingChatModel, tools []einotool.BaseTool) (compose.Runnable[[]*schema.Message, []*schema.Message], *checkpoint.MemCheckpointStore) {
	t.Helper()
	ctx := context.Background()
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: tools, ExecuteSequentially: true,
	})
	if err != nil {
		t.Fatalf("NewToolNode: %v", err)
	}
	ckpt := checkpoint.NewMemCheckpointStore()
	g, err := BuildAgentGraph(ctx, chatModel, toolsNode, ckpt)
	if err != nil {
		t.Fatalf("BuildAgentGraph: %v", err)
	}
	return g, ckpt
}

func TestBuildAgentGraph_NoToolCalls_EndsAtModel(t *testing.T) {
	m := &testutil.ScriptedModel{Responses: []*schema.Message{
		schema.AssistantMessage("直接回答", nil),
	}}
	et := &echoTool{}
	g, _ := buildTestGraph(t, m, []einotool.BaseTool{et})

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
	if m.Calls != 1 {
		t.Errorf("模型应只被调用 1 次，实际 %d", m.Calls)
	}
}

func TestBuildAgentGraph_ToolCallRoutesToToolsAndLoops(t *testing.T) {
	m := &testutil.ScriptedModel{Responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "c1", Type: "function",
			Function: schema.FunctionCall{Name: "echo_tool", Arguments: `{"text":"hi"}`},
		}}),
		schema.AssistantMessage("最终结论", nil),
	}}
	et := &echoTool{}
	g, _ := buildTestGraph(t, m, []einotool.BaseTool{et})

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
	if m.Calls != 2 {
		t.Errorf("模型应被调用 2 次（提议+总结），实际 %d", m.Calls)
	}
	if len(m.Inputs) != 2 {
		t.Fatalf("期望记录 2 次模型输入，得到 %d", len(m.Inputs))
	}
	second := m.Inputs[1]
	if len(second) != 3 {
		t.Fatalf("第二次模型调用应收到完整历史 3 条消息，得到 %d", len(second))
	}
	if second[0].Role != schema.User || second[1].Role != schema.Assistant ||
		len(second[1].ToolCalls) != 1 || second[2].Role != schema.Tool {
		t.Errorf("第二次模型调用历史角色/配对错误: %v / %v / %v",
			second[0].Role, second[1].Role, second[2].Role)
	}
}

// interruptTool 首次调用即 einotool.Interrupt；恢复后返回结果。
type interruptTool struct{}

func (t *interruptTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "interrupt_tool",
		Desc: "测试中断工具",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"x": {Type: schema.String, Desc: "参数", Required: true},
		}),
	}, nil
}

func (t *interruptTool) InvokableRun(ctx context.Context, argsJSON string, opts ...einotool.Option) (string, error) {
	wasInterrupted, _, _ := einotool.GetInterruptState[string](ctx)
	if !wasInterrupted {
		return "", einotool.Interrupt(ctx, "awaiting-approval")
	}
	isTarget, hasData, data := einotool.GetResumeContext[string](ctx)
	if !isTarget || !hasData {
		return "", einotool.Interrupt(ctx, "awaiting-approval")
	}
	return "resumed:" + data, nil
}

func TestBuildAgentGraph_InterruptAndResumeCycle(t *testing.T) {
	m := &testutil.ScriptedModel{Responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "c1", Type: "function",
			Function: schema.FunctionCall{Name: "interrupt_tool", Arguments: `{"x":"1"}`},
		}}),
		schema.AssistantMessage("完成", nil),
	}}
	ckpt := checkpoint.NewMemCheckpointStore()
	toolsNode, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
		Tools: []einotool.BaseTool{&interruptTool{}}, ExecuteSequentially: true,
	})
	if err != nil {
		t.Fatalf("NewToolNode: %v", err)
	}
	g, err := BuildAgentGraph(context.Background(), m, toolsNode, ckpt)
	if err != nil {
		t.Fatalf("BuildAgentGraph: %v", err)
	}
	input := []*schema.Message{schema.UserMessage("开始")}

	// 首次 Invoke：中断
	_, err = g.Invoke(context.Background(), input, compose.WithCheckPointID("s-int"))
	info, ok := compose.ExtractInterruptInfo(err)
	if !ok || len(info.InterruptContexts) == 0 {
		t.Fatalf("期望中断错误，得到 %v", err)
	}
	// checkpoint 已持久化
	if _, exists, _ := ckpt.Get(context.Background(), "s-int"); !exists {
		t.Fatal("中断后 checkpoint 未持久化")
	}

	// Resume：完成
	resumeCtx := compose.ResumeWithData(context.Background(), info.InterruptContexts[0].ID, "approved")
	out, err := g.Invoke(resumeCtx, input, compose.WithCheckPointID("s-int"))
	if err != nil {
		t.Fatalf("Resume Invoke: %v", err)
	}
	if len(out) == 0 || out[len(out)-1].Content != "完成" {
		t.Errorf("Resume 后未得到最终回复: %+v", out)
	}
}
