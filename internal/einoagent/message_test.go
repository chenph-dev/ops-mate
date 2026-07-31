package einoagent

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestToEinoMessages_RoleMapping(t *testing.T) {
	msgs := []Message{
		{Role: RoleUser, Content: "cpu 高"},
		{Role: RoleAssistant, Content: "我看看"},
		{Role: RoleTool, Content: "top -bn1", ToolResult: "go 99%"},
	}

	out := ToEinoMessages(msgs)
	if len(out) != 3 {
		t.Fatalf("期望 3 条消息，得到 %d", len(out))
	}
	if out[0].Role != schema.User || out[0].Content != "cpu 高" {
		t.Errorf("第一条消息 role/content 错误: %+v", out[0])
	}
	if out[1].Role != schema.Assistant || out[1].Content != "我看看" {
		t.Errorf("第二条消息 role/content 错误: %+v", out[1])
	}
	if out[2].Role != schema.Tool {
		t.Errorf("第三条消息 role 错误: %s", out[2].Role)
	}
	if out[2].Content != "[执行结果]\ngo 99%" {
		t.Errorf("第三条消息 content 错误: %q", out[2].Content)
	}
}

func TestFromEinoMessage_RoleMapping(t *testing.T) {
	cases := []struct {
		in       *schema.Message
		wantRole Role
	}{
		{&schema.Message{Role: schema.User, Content: "hi"}, RoleUser},
		{&schema.Message{Role: schema.Assistant, Content: "ok"}, RoleAssistant},
		{&schema.Message{Role: schema.Tool, Content: "result"}, RoleTool},
		{&schema.Message{Role: schema.System, Content: "sys"}, RoleAssistant},
	}
	for _, c := range cases {
		got := FromEinoMessage(c.in)
		if got.Role != c.wantRole {
			t.Errorf("FromEinoMessage(%s) role = %s, want %s", c.in.Role, got.Role, c.wantRole)
		}
		if got.Content != c.in.Content {
			t.Errorf("FromEinoMessage content = %q, want %q", got.Content, c.in.Content)
		}
	}
}

func TestSystemMessage_UsesAgentPrompt(t *testing.T) {
	m := SystemMessage()
	if m.Role != schema.System {
		t.Errorf("SystemMessage role = %s, want system", m.Role)
	}
	if m.Content != SystemPrompt {
		t.Errorf("SystemMessage content 不等于 SystemPrompt")
	}
}
