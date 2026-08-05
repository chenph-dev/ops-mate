package einoagent

import (
	"testing"

	"github.com/cloudwego/eino/schema"

	convstore "ops-mate/internal/store/conversations"
)

func TestToolCallsToJSON_RoundTrip(t *testing.T) {
	tcs := []schema.ToolCall{{
		ID: "call_1", Type: "function",
		Function: schema.FunctionCall{Name: "execute_command", Arguments: `{"command":"ls","why":"看看"}`},
	}}
	js := ToolCallsToJSON(tcs)
	if js == "" {
		t.Fatal("期望非空 JSON")
	}
	back, err := parseToolCallsJSON(js)
	if err != nil {
		t.Fatalf("parseToolCallsJSON: %v", err)
	}
	if len(back) != 1 || back[0].ID != "call_1" ||
		back[0].Function.Name != "execute_command" ||
		back[0].Function.Arguments != `{"command":"ls","why":"看看"}` {
		t.Errorf("往返失败: %+v", back)
	}
}

func TestToolCallsToJSON_Empty(t *testing.T) {
	if got := ToolCallsToJSON(nil); got != "" {
		t.Errorf("空 tool calls 期望空串，得到 %q", got)
	}
}

func TestHistoryToEino_AllRoles(t *testing.T) {
	msgs := []convstore.Message{
		{Role: RoleUser, Content: "cpu 高"},
		{Role: RoleAssistant, Content: "", ToolCalls: `[{"id":"call_1","name":"execute_command","arguments":"{\"command\":\"top -bn1\"}"}]`},
		{Role: RoleTool, Content: "go 99%", ToolCallID: "call_1", ToolName: "execute_command"},
		{Role: RoleAssistant, Content: "是 go 进程占满"},
	}
	out, err := HistoryToEino(msgs)
	if err != nil {
		t.Fatalf("HistoryToEino: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("期望 4 条，得到 %d", len(out))
	}
	if out[0].Role != schema.User || out[0].Content != "cpu 高" {
		t.Errorf("user 消息错误: %+v", out[0])
	}
	if out[1].Role != schema.Assistant || len(out[1].ToolCalls) != 1 ||
		out[1].ToolCalls[0].ID != "call_1" ||
		out[1].ToolCalls[0].Function.Name != "execute_command" {
		t.Errorf("assistant tool_calls 还原错误: %+v", out[1])
	}
	if out[2].Role != schema.Tool || out[2].ToolCallID != "call_1" ||
		out[2].ToolName != "execute_command" || out[2].Content != "go 99%" {
		t.Errorf("tool 消息还原错误: %+v", out[2])
	}
	if out[3].Role != schema.Assistant || out[3].Content != "是 go 进程占满" {
		t.Errorf("纯文本 assistant 错误: %+v", out[3])
	}
}

func TestHistoryToEino_BadToolCallsJSON(t *testing.T) {
	msgs := []convstore.Message{
		{Role: RoleAssistant, ToolCalls: "{bad json"},
	}
	if _, err := HistoryToEino(msgs); err == nil {
		t.Error("期望损坏的 tool_calls JSON 返回错误")
	}
}

func TestHistoryToEino_OrphanToolCallsStripped(t *testing.T) {
	// 审批中断后会话被放弃：assistant 提议命令但未执行，历史遗留孤立 tool_use。
	// 必须剥离，否则模型端以 "tool_use without tool_result" 拒绝整个请求（400）。
	msgs := []convstore.Message{
		{Role: RoleUser, Content: "看看"},
		{Role: RoleAssistant, Content: "", ToolCalls: `[{"id":"call_orphan","name":"execute_command","arguments":"{\"command\":\"top -bn1\"}"}]`},
		{Role: RoleUser, Content: "算了"},
	}
	out, err := HistoryToEino(msgs)
	if err != nil {
		t.Fatalf("HistoryToEino: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("孤立 tool_use 应被剥离（期望 2 条），得到 %d: %+v", len(out), out)
	}
	for _, m := range out {
		if len(m.ToolCalls) > 0 {
			t.Errorf("不应存在任何 tool_calls: %+v", m.ToolCalls)
		}
	}
}

func TestHistoryToEino_PartialToolCallsMatched(t *testing.T) {
	// 多 tool_calls 只执行了第一个：未配对的第二个 tool_call 必须被剥离。
	msgs := []convstore.Message{
		{Role: RoleUser, Content: "查一下"},
		{Role: RoleAssistant, Content: "", ToolCalls: `[{"id":"c1","name":"execute_command","arguments":"{\"command\":\"ls\"}"},{"id":"c2","name":"execute_command","arguments":"{\"command\":\"pwd\"}"}]`},
		{Role: RoleTool, Content: "file1", ToolCallID: "c1", ToolName: "execute_command"},
		{Role: RoleAssistant, Content: "总结"},
	}
	out, err := HistoryToEino(msgs)
	if err != nil {
		t.Fatalf("HistoryToEino: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("期望 4 条，得到 %d: %+v", len(out), out)
	}
	if len(out[1].ToolCalls) != 1 || out[1].ToolCalls[0].ID != "c1" {
		t.Errorf("未配对的 tool_call 应被剥离，assistant 只保留 c1: %+v", out[1].ToolCalls)
	}
	if out[2].Role != schema.Tool || out[2].ToolCallID != "c1" {
		t.Errorf("tool 消息配对错误: %+v", out[2])
	}
}

func TestHistoryToEino_OrphanToolResultDropped(t *testing.T) {
	// 无 assistant tool_use 的孤立 tool_result：模型不接受，应丢弃。
	msgs := []convstore.Message{
		{Role: RoleUser, Content: "hi"},
		{Role: RoleTool, Content: "孤立结果", ToolCallID: "ghost", ToolName: "execute_command"},
	}
	out, err := HistoryToEino(msgs)
	if err != nil {
		t.Fatalf("HistoryToEino: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("孤立 tool_result 应被丢弃（期望 1 条），得到 %d: %+v", len(out), out)
	}
}
