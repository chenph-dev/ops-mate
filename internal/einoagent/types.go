package einoagent

import (
	"context"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// Role 消息角色。
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message 一条对话消息。
type Message struct {
	Role       Role   `json:"role"`
	Content    string `json:"content"`
	ToolResult string `json:"toolResult"`
}

// Chunk 流式返回片段。Text 为文本增量；
// 若 Command != nil，表示 AI 提议了一条命令。
type Chunk struct {
	Text    string             `json:"text"`
	Command *CommandSuggestion `json:"command"`
}

// CommandSuggestion AI 提议的命令（结构化）。
type CommandSuggestion struct {
	Command string `json:"command"`
	Why     string `json:"why"`
	Risk    string `json:"risk"` // "low" | "medium" | "high"
}

// LLMClient 统一 AI 后端抽象。
type LLMClient interface {
	Chat(ctx context.Context, msgs []Message) (<-chan Chunk, error)
}

// SystemPrompt 约束 AI 只返回：普通文本，或 JSON 命令块。
const SystemPrompt = `你是一个 SSH 运维助手。回答用户关于远程 Linux 主机的问题。
你可以：
1) 用普通文本解释分析；
2) 提议一条要在目标主机执行的命令，输出严格的 JSON：{"command":"...","why":"...","risk":"low|medium|high"}
不要把命令写在普通文本里；要执行就只用 JSON 命令块。
每次最多提议一条命令。无命令时输出普通文本即可。`

// ============================================================
// 消息类型转换（agent.Message ↔ eino schema.Message）
// ============================================================

// ToEinoMessages 将 agent.Message 列表转换为 eino 的 schema.Message 列表。
// 角色映射：user→user, assistant→assistant, tool→tool。
// tool 消息的 ToolResult 拼入 Content，供模型回灌。
func ToEinoMessages(msgs []Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case RoleUser:
			out = append(out, &schema.Message{
				Role:    schema.User,
				Content: m.Content,
			})
		case RoleAssistant:
			out = append(out, &schema.Message{
				Role:    schema.Assistant,
				Content: m.Content,
			})
		case RoleTool:
			out = append(out, &schema.Message{
				Role:    schema.Tool,
				Content: "[执行结果]\n" + m.ToolResult,
			})
		}
	}
	return out
}

// FromEinoMessage 将 eino 的 schema.Message 转换为 agent.Message。
func FromEinoMessage(m *schema.Message) Message {
	switch m.Role {
	case schema.User:
		return Message{Role: RoleUser, Content: m.Content}
	case schema.Assistant:
		return Message{Role: RoleAssistant, Content: m.Content}
	case schema.Tool:
		return Message{Role: RoleTool, Content: m.Content}
	default:
		return Message{Role: RoleAssistant, Content: m.Content}
	}
}

// SystemMessage 构造 eino 系统消息，使用 agent 的 SystemPrompt。
func SystemMessage() *schema.Message {
	return &schema.Message{
		Role:    schema.System,
		Content: SystemPrompt,
	}
}

// ============================================================
// 危险命令护栏
// ============================================================

// dangerPatterns 匹配即返回 "high" 风险等级。
var dangerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+-[a-zA-Z]*r[a-zA-Z]*f?.*\s/\s`),  // rm -rf ... /
	regexp.MustCompile(`rm\s+-[a-zA-Z]*r[a-zA-Z]*f?\s+/\s*$`), // rm -rf /
	regexp.MustCompile(`\bmkfs\b`),
	regexp.MustCompile(`\bdd\b.*\bof=/dev/`),
	regexp.MustCompile(`\b(shutdown|poweroff|halt|reboot)\b`),
	regexp.MustCompile(`>\s*/dev/(sd|nvme|hd|vd)`), // 重定向到块设备
	regexp.MustCompile(`:\(\)\s*\{\s*:\|:&\s*\}\s*;`), // fork bomb
}

// AssessRisk 返回命令风险等级："" 无风险，"high" 危险。
// 引擎层永不硬拒——只用于前端标红与二次确认。
func AssessRisk(command string) string {
	c := strings.TrimSpace(command)
	for _, p := range dangerPatterns {
		if p.MatchString(c) {
			return "high"
		}
	}
	return ""
}
