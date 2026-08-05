package einoagent

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/sshexec"
)

// approvalFixture 组装：scripted 模型 + SSHTool(fake executor) + Graph + checkpoint。
func approvalFixture(t *testing.T, modelResponses []*schema.Message, execLines []sshexec.Line) (
	compose.Runnable[[]*schema.Message, []*schema.Message],
	*fakeExec, *emitRecorder,
) {
	t.Helper()
	ctx := context.Background()
	ex := &fakeExec{lines: execLines}
	rec := &emitRecorder{}
	holder := newToolCallHolder()
	sshTool := NewSSHTool("s1", ex, rec.emit, nil, holder)

	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{sshTool}, ExecuteSequentially: true,
	})
	if err != nil {
		t.Fatalf("NewToolNode: %v", err)
	}
	m := &scriptedModel{responses: modelResponses}
	// 模拟 SessionManager 的 onAssistant：记录 holder（集成测试只关心命令执行）
	wrapped := NewStreamingChatModel(m, "s1", rec.emit, func(msg *schema.Message) {
		if len(msg.ToolCalls) > 0 {
			holder.Set(&msg.ToolCalls[0])
		}
	})
	g, err := BuildAgentGraph(ctx, wrapped, toolsNode, newMemCheckpointStore())
	if err != nil {
		t.Fatalf("BuildAgentGraph: %v", err)
	}
	return g, ex, rec
}

func toolCallResponse(cmd string) *schema.Message {
	return schema.AssistantMessage("", []schema.ToolCall{{
		ID: "call_1", Type: "function",
		Function: schema.FunctionCall{
			Name:      "execute_command",
			Arguments: `{"command":"` + cmd + `","why":"诊断"}`,
		},
	}})
}

func TestApprovalFlow_ApproveWithEditedCommand(t *testing.T) {
	g, ex, _ := approvalFixture(t,
		[]*schema.Message{
			toolCallResponse("ls"),
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

	cmds := ex.commands()
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
			toolCallResponse("reboot"),
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
	if len(ex.commands()) != 0 {
		t.Error("拒绝后不应执行任何命令")
	}
	if len(out) == 0 || out[len(out)-1].Content != "好的，换个方案" {
		t.Errorf("拒绝后模型应重新回复: %+v", out)
	}
}

func TestApprovalFlow_ResumeDoesNotRecallModelBeforeInterrupt(t *testing.T) {
	// checkpoint 的核心价值：Resume 不重复中断点之前的 LLM 调用。
	// 用 scriptedModel 的调用次数断言：整个 approve 流程模型只被调用 2 次
	// （首次提议 1 次 + 回灌总结 1 次），Resume 不触发第 3 次。
	ctx := context.Background()
	ex := &fakeExec{lines: []sshexec.Line{{Stream: "stdout", Text: "ok"}}}
	rec := &emitRecorder{}
	holder := newToolCallHolder()
	sshTool := NewSSHTool("s1", ex, rec.emit, nil, holder)
	toolsNode, _ := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{sshTool}, ExecuteSequentially: true,
	})
	m := &scriptedModel{responses: []*schema.Message{
		toolCallResponse("uptime"),
		schema.AssistantMessage("运行正常", nil),
	}}
	wrapped := NewStreamingChatModel(m, "s1", rec.emit, func(msg *schema.Message) {
		if len(msg.ToolCalls) > 0 {
			holder.Set(&msg.ToolCalls[0])
		}
	})
	g, err := BuildAgentGraph(ctx, wrapped, toolsNode, newMemCheckpointStore())
	if err != nil {
		t.Fatalf("BuildAgentGraph: %v", err)
	}
	input := []*schema.Message{schema.UserMessage("状态如何")}

	_, err = g.Invoke(context.Background(), input, compose.WithCheckPointID("s3"))
	info, ok := compose.ExtractInterruptInfo(err)
	if !ok {
		t.Fatalf("期望中断: %v", err)
	}
	callsBeforeResume := m.calls

	resumeCtx := compose.ResumeWithData(context.Background(), info.InterruptContexts[0].ID, "uptime")
	if _, err := g.Invoke(resumeCtx, input, compose.WithCheckPointID("s3")); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if m.calls != callsBeforeResume+1 {
		t.Errorf("Resume 不应重放中断点前的模型调用：之前 %d 次，之后共 %d 次（期望 +1）",
			callsBeforeResume, m.calls)
	}
}

func TestApprovalFlow_ApproveWithEmptyCommandGuarded(t *testing.T) {
	g, ex, _ := approvalFixture(t,
		[]*schema.Message{
			toolCallResponse("ls"),
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
	if len(ex.commands()) != 0 {
		t.Errorf("空批准数据不应执行任何命令: %v", ex.commands())
	}
	if len(out) == 0 || out[len(out)-1].Content != "重新提议" {
		t.Errorf("空命令守卫应回灌模型重新提议: %+v", out)
	}
}
