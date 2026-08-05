package graph

import (
	"context"
	"testing"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	agentmodel "ops-mate/internal/einoagent/model"
	agenttools "ops-mate/internal/einoagent/tools"
	"ops-mate/internal/einoagent/testutil"
	"ops-mate/internal/sshexec"
	"ops-mate/internal/einoagent/checkpoint"
)

// approvalFixture 组装：ScriptedModel + SSHTool(fake executor) + Graph + checkpoint。
func approvalFixture(t *testing.T, modelResponses []*schema.Message, execLines []sshexec.Line) (
	compose.Runnable[[]*schema.Message, []*schema.Message],
	*testutil.FakeExec, *testutil.EmitRecorder,
) {
	t.Helper()
	ctx := context.Background()
	ex := &testutil.FakeExec{Lines: execLines}
	rec := &testutil.EmitRecorder{}
	holder := agenttools.NewToolCallHolder()
	sshTool := agenttools.NewSSHTool("s1", ex, rec.Emit, nil, holder)

	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []einotool.BaseTool{sshTool}, ExecuteSequentially: true,
	})
	if err != nil {
		t.Fatalf("NewToolNode: %v", err)
	}
	m := &testutil.ScriptedModel{Responses: modelResponses}
	// 模拟 SessionManager 的 onAssistant：记录 holder（集成测试只关心命令执行）
	wrapped := agentmodel.NewStreamingChatModel(m, "s1", rec.Emit, func(msg *schema.Message) {
		if len(msg.ToolCalls) > 0 {
			holder.Add(&msg.ToolCalls[0])
		}
	})
	g, err := BuildAgentGraph(ctx, wrapped, toolsNode, checkpoint.NewMemCheckpointStore())
	if err != nil {
		t.Fatalf("BuildAgentGraph: %v", err)
	}
	return g, ex, rec
}

func TestApprovalFlow_ApproveWithEditedCommand(t *testing.T) {
	g, ex, _ := approvalFixture(t,
		[]*schema.Message{
			testutil.ToolCallResponse("ls"),
			schema.AssistantMessage("文件已列出", nil),
		},
		[]sshexec.Line{{Stream: "stdout", Text: "file1"}},
	)
	input := []*schema.Message{schema.UserMessage("看看文件")}

	// 第一次 Invoke：应中断等审批
	_, err := g.Invoke(context.Background(), input, compose.WithCheckPointID("s1"))
	if err == nil {
		t.Fatal("期望中断错误")
	}
	info, ok := compose.ExtractInterruptInfo(err)
	if !ok || len(info.InterruptContexts) == 0 {
		t.Fatalf("期望 InterruptInfo，得到 %v", err)
	}
	interruptID := info.InterruptContexts[0].ID

	// 批准（用户把命令改成了 ls -la）
	resumeCtx := compose.ResumeWithData(context.Background(), interruptID, "ls -la")
	out, err := g.Invoke(resumeCtx, input, compose.WithCheckPointID("s1"))
	if err != nil {
		t.Fatalf("Resume Invoke: %v", err)
	}

	cmds := ex.Commands()
	if len(cmds) != 1 || cmds[0] != "ls -la" {
		t.Fatalf("应执行用户编辑后的命令，得到 %v", cmds)
	}
	if len(out) == 0 || out[len(out)-1].Content != "文件已列出" {
		t.Errorf("回灌后未得到最终回复: %+v", out)
	}
}

func TestApprovalFlow_RejectFeedsBack(t *testing.T) {
	g, ex, _ := approvalFixture(t,
		[]*schema.Message{
			testutil.ToolCallResponse("reboot"),
			schema.AssistantMessage("好的，换个方案", nil),
		},
		nil,
	)
	input := []*schema.Message{schema.UserMessage("重启一下")}

	_, err := g.Invoke(context.Background(), input, compose.WithCheckPointID("s2"))
	info, ok := compose.ExtractInterruptInfo(err)
	if !ok || len(info.InterruptContexts) == 0 {
		t.Fatalf("期望中断: %v", err)
	}

	resumeCtx := compose.ResumeWithData(context.Background(), info.InterruptContexts[0].ID, "rejected")
	out, err := g.Invoke(resumeCtx, input, compose.WithCheckPointID("s2"))
	if err != nil {
		t.Fatalf("Resume(rejected): %v", err)
	}
	if len(ex.Commands()) != 0 {
		t.Error("拒绝后不应执行任何命令")
	}
	if len(out) == 0 || out[len(out)-1].Content != "好的，换个方案" {
		t.Errorf("拒绝后模型应重新回复: %+v", out)
	}
}

func TestApprovalFlow_ResumeDoesNotRecallModelBeforeInterrupt(t *testing.T) {
	// checkpoint 的核心价值：Resume 不重复中断点之前的 LLM 调用。
	// 用 ScriptedModel 的调用次数断言：整个 approve 流程模型只被调用 2 次
	// （首次提议 1 次 + 回灌总结 1 次），Resume 不触发第 3 次。
	ctx := context.Background()
	ex := &testutil.FakeExec{Lines: []sshexec.Line{{Stream: "stdout", Text: "ok"}}}
	rec := &testutil.EmitRecorder{}
	holder := agenttools.NewToolCallHolder()
	sshTool := agenttools.NewSSHTool("s1", ex, rec.Emit, nil, holder)
	toolsNode, _ := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []einotool.BaseTool{sshTool}, ExecuteSequentially: true,
	})
	m := &testutil.ScriptedModel{Responses: []*schema.Message{
		testutil.ToolCallResponse("uptime"),
		schema.AssistantMessage("运行正常", nil),
	}}
	wrapped := agentmodel.NewStreamingChatModel(m, "s1", rec.Emit, func(msg *schema.Message) {
		if len(msg.ToolCalls) > 0 {
			holder.Add(&msg.ToolCalls[0])
		}
	})
	g, err := BuildAgentGraph(ctx, wrapped, toolsNode, checkpoint.NewMemCheckpointStore())
	if err != nil {
		t.Fatalf("BuildAgentGraph: %v", err)
	}
	input := []*schema.Message{schema.UserMessage("状态如何")}

	_, err = g.Invoke(context.Background(), input, compose.WithCheckPointID("s3"))
	info, ok := compose.ExtractInterruptInfo(err)
	if !ok {
		t.Fatalf("期望中断: %v", err)
	}
	callsBeforeResume := m.Calls
	if callsBeforeResume != 1 {
		t.Fatalf("中断前模型应恰好被调用 1 次，实际 %d", callsBeforeResume)
	}

	resumeCtx := compose.ResumeWithData(context.Background(), info.InterruptContexts[0].ID, "uptime")
	if _, err := g.Invoke(resumeCtx, input, compose.WithCheckPointID("s3")); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if m.Calls != callsBeforeResume+1 {
		t.Errorf("Resume 不应重放中断点前的模型调用：之前 %d 次，之后共 %d 次（期望 +1）",
			callsBeforeResume, m.Calls)
	}
}

func TestApprovalFlow_ApproveWithEmptyCommandGuarded(t *testing.T) {
	g, ex, _ := approvalFixture(t,
		[]*schema.Message{
			testutil.ToolCallResponse("ls"),
			schema.AssistantMessage("重新提议", nil),
		},
		nil,
	)
	input := []*schema.Message{schema.UserMessage("看看")}

	_, err := g.Invoke(context.Background(), input, compose.WithCheckPointID("s-empty"))
	info, ok := compose.ExtractInterruptInfo(err)
	if !ok || len(info.InterruptContexts) == 0 {
		t.Fatalf("期望中断: %v", err)
	}

	// 批准数据为空（前端清空编辑框后点批准的边界情况）
	resumeCtx := compose.ResumeWithData(context.Background(), info.InterruptContexts[0].ID, "")
	out, err := g.Invoke(resumeCtx, input, compose.WithCheckPointID("s-empty"))
	if err != nil {
		t.Fatalf("Resume(空命令): %v", err)
	}
	if len(ex.Commands()) != 0 {
		t.Errorf("空批准数据不应执行任何命令: %v", ex.Commands())
	}
	if len(out) == 0 || out[len(out)-1].Content != "重新提议" {
		t.Errorf("空命令守卫应回灌模型重新提议: %+v", out)
	}
}
