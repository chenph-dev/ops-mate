package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/einoagent/history"
	"ops-mate/internal/einoagent/prompt"
)

// buildInput 组装模型输入：系统提示（模板） + 记忆注入 + DB 历史。
// 会话历史超上限时截取最近 N 条并提示模型早期已省略（控制上下文防爆炸）。
func (m *SessionManager) buildInput(s *agentSession, userText string) ([]*schema.Message, error) {
	hist, err := m.convs.LoadMessages(s.id)
	if err != nil {
		return nil, fmt.Errorf("加载历史失败: %w", err)
	}
	// 上下文控制：超过上限时保留最近 maxHistoryMessages 条（早期省略，
	// HistoryToEino 会丢弃截断点处孤立的 tool 配对，不会导致模型端 400）
	var omitted *schema.Message
	if len(hist) > maxHistoryMessages {
		truncated := len(hist) - maxHistoryMessages
		hist = hist[len(hist)-maxHistoryMessages:]
		omitted = schema.AssistantMessage(
			fmt.Sprintf("[已省略较早的 %d 条对话以控制上下文]", truncated), nil)
	}
	msgs, err := history.HistoryToEino(hist)
	if err != nil {
		return nil, err
	}

	// 模板参数：主机名 + 记忆（失败不阻断主流程）
	params := map[string]any{"HostName": "", "Memory": ""}
	if m.hostNameFor != nil {
		if name, err := m.hostNameFor(s.hostID); err == nil {
			params["HostName"] = name
		}
	}
	if recall, err := m.mem.Recall(s.hostID, userText); err == nil && len(recall.PastCommands) > 0 {
		var note strings.Builder
		for _, c := range recall.PastCommands {
			note.WriteString("- ")
			note.WriteString(c.Command)
			note.WriteString("\n")
		}
		params["Memory"] = strings.TrimSuffix(note.String(), "\n")
	}
	sysMsgs, err := prompt.BuildSystemMessages(context.Background(), params)
	if err != nil {
		return nil, fmt.Errorf("渲染系统提示词: %w", err)
	}

	input := make([]*schema.Message, 0, len(msgs)+len(sysMsgs)+2)
	input = append(input, sysMsgs...)
	if omitted != nil {
		input = append(input, omitted)
	}
	input = append(input, msgs...)
	return input, nil
}
