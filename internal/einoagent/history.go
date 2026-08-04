package einoagent

import (
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/schema"

	convstore "ops-mate/internal/store/conversations"
)

// 角色常量（与 convstore.Message.Role 一致）。
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// toolCallDTO 是 assistant 消息 tool_calls 的落库结构。
type toolCallDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCallsToJSON 把模型返回的 tool calls 序列化为落库 JSON。
// 空列表返回空串（数据库存 NULL）。
func ToolCallsToJSON(tcs []schema.ToolCall) string {
	if len(tcs) == 0 {
		return ""
	}
	dtos := make([]toolCallDTO, 0, len(tcs))
	for _, tc := range tcs {
		dtos = append(dtos, toolCallDTO{
			ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments,
		})
	}
	b, err := json.Marshal(dtos)
	if err != nil {
		return ""
	}
	return string(b)
}

// parseToolCallsJSON 反序列化 tool_calls JSON 为 eino ToolCall。
func parseToolCallsJSON(js string) ([]schema.ToolCall, error) {
	var dtos []toolCallDTO
	if err := json.Unmarshal([]byte(js), &dtos); err != nil {
		return nil, fmt.Errorf("parse tool_calls: %w", err)
	}
	out := make([]schema.ToolCall, 0, len(dtos))
	for _, d := range dtos {
		out = append(out, schema.ToolCall{
			ID: d.ID, Type: "function",
			Function: schema.FunctionCall{Name: d.Name, Arguments: d.Arguments},
		})
	}
	return out, nil
}

// HistoryToEino 把持久化的消息历史转换为模型输入。
// assistant tool_calls 与 tool 消息按 tool_call_id 忠实还原，
// 保证 Claude/OpenAI 对 tool_use/tool_result 配对的严格要求。
func HistoryToEino(msgs []convstore.Message) ([]*schema.Message, error) {
	out := make([]*schema.Message, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case RoleUser:
			out = append(out, schema.UserMessage(m.Content))
		case RoleAssistant:
			msg := schema.AssistantMessage(m.Content, nil)
			if m.ToolCalls != "" {
				tcs, err := parseToolCallsJSON(m.ToolCalls)
				if err != nil {
					return nil, err
				}
				msg.ToolCalls = tcs
			}
			out = append(out, msg)
		case RoleTool:
			out = append(out, &schema.Message{
				Role: schema.Tool, Content: m.Content,
				ToolCallID: m.ToolCallID, ToolName: m.ToolName,
			})
		}
	}
	return out, nil
}
