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
