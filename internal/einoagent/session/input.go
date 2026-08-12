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
	// HistoryToEino 会丢弃截断点处孤立的 tool 配对，不会导致模型端 400）。
	var omitted *schema.Message
	if len(hist) > maxHistoryMessages {
		// 截断窗口必须从 user 消息开始：OpenAI/Anthropic 等 API 硬性要求
		// 「第一条非 system 消息是 user」，窗口若以 assistant/tool 开头，
		// HistoryToEino 还原后序列第一条非 system 可能不是 user（报
		// "first non-system message should be user message"）。
		start := len(hist) - maxHistoryMessages
		for start < len(hist) && hist[start].Role != history.RoleUser {
			start++
		}
		if start >= len(hist) {
			// 兜底：窗口内没有 user（理论上每轮都有 user 消息，不会走到），
			// 退回到原始截断点。
			start = len(hist) - maxHistoryMessages
		}
		truncated := start
		hist = hist[start:]
		// 占位提示用 system 角色（语义是系统说明），且位于所有 system 之后、
		// user 之前，不破坏「第一条非 system 是 user」约束。
		omitted = schema.SystemMessage(
			fmt.Sprintf("较早的 %d 条对话已省略，请基于现有上下文继续。", truncated))
	}
	msgs, err := history.HistoryToEino(hist)
	if err != nil {
		return nil, err
	}

	// 模板参数：主机名 + 记忆 + 终端上下文 + 技能目录（失败不阻断主流程）
	params := map[string]any{"HostName": "", "Memory": "", "TerminalContext": "", "SkillsCatalog": ""}
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
	if m.terminalContextFor != nil {
		if ctx := m.terminalContextFor(s.hostID); ctx != "" {
			params["TerminalContext"] = ctx
		}
	}
	if m.skillCatalogFor != nil {
		if c := m.skillCatalogFor(); c != "" {
			params["SkillsCatalog"] = c
		}
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
