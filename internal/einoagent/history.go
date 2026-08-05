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
//
// 配对防御：若历史中存在孤立的 tool_calls（如审批中断后会话被放弃、
// 或多 tool_calls 只部分执行导致配对错位），剥离未配对的 tool_calls——
// 否则模型端会以 "tool_use without tool_result" 拒绝整个请求（HTTP 400）。
func HistoryToEino(msgs []convstore.Message) ([]*schema.Message, error) {
	out := make([]*schema.Message, 0, len(msgs))
	var pending *schema.Message // 待配对的 assistant（带 tool_calls）
	for _, m := range msgs {
		switch m.Role {
		case RoleUser:
			flushPendingToolCalls(&out, &pending)
			out = append(out, schema.UserMessage(m.Content))
		case RoleAssistant:
			flushPendingToolCalls(&out, &pending)
			if m.ToolCalls != "" {
				tcs, err := parseToolCallsJSON(m.ToolCalls)
				if err != nil {
					return nil, fmt.Errorf("message %s: %w", m.ID, err)
				}
				pending = schema.AssistantMessage(m.Content, tcs)
			} else {
				out = append(out, schema.AssistantMessage(m.Content, nil))
			}
		case RoleTool:
			if pending != nil && len(pending.ToolCalls) > 0 &&
				pending.ToolCalls[0].ID == m.ToolCallID {
				// 配对成功：assistant 只保留已配对的 tool_calls，紧跟 tool 结果。
				assistant := *pending
				assistant.ToolCalls = pending.ToolCalls[:1]
				out = append(out, &assistant)
				pending = nil
				out = append(out, &schema.Message{
					Role: schema.Tool, Content: m.Content,
					ToolCallID: m.ToolCallID, ToolName: m.ToolName,
				})
			} else {
				// 无配对的 tool 消息：丢弃（模型不接受无 tool_use 的 tool_result）。
				flushPendingToolCalls(&out, &pending)
			}
		}
	}
	flushPendingToolCalls(&out, &pending)
	return out, nil
}

// flushPendingToolCalls 处理未配对的 pending assistant：
// 剥离其 tool_calls；若仍有文本内容则保留为纯文本 assistant，否则丢弃整条。
func flushPendingToolCalls(out *[]*schema.Message, pending **schema.Message) {
	p := *pending
	if p == nil {
		return
	}
	*pending = nil
	if p.Content != "" {
		p.ToolCalls = nil
		*out = append(*out, p)
	}
}
