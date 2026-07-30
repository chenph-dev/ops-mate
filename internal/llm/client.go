package llm

import "context"

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
