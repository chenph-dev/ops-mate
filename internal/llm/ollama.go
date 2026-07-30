package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Ollama 调用本地 Ollama HTTP /api/chat（流式 NDJSON）。
type Ollama struct {
	baseURL string
	model   string
}

func NewOllama(baseURL, model string) *Ollama {
	return &Ollama{baseURL: baseURL, model: model}
}

type ollamaChatReq struct {
	Model    string      `json:"model"`
	Messages []ollamaMsg `json:"messages"`
	Stream   bool        `json:"stream"`
}

type ollamaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChunk struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

func (o *Ollama) Chat(ctx context.Context, msgs []Message) (<-chan Chunk, error) {
	payload := ollamaChatReq{Model: o.model, Stream: true}
	payload.Messages = append(payload.Messages, ollamaMsg{Role: "system", Content: SystemPrompt})
	for _, m := range msgs {
		role := string(m.Role)
		content := m.Content
		if m.Role == RoleTool {
			content = "[执行结果]\n" + m.ToolResult
			role = "user"
		}
		payload.Messages = append(payload.Messages, ollamaMsg{Role: role, Content: content})
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("ollama status %d", resp.StatusCode)
	}
	out := make(chan Chunk, 16)
	go func() {
		defer resp.Body.Close()
		defer close(out)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Bytes()
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var ch ollamaChunk
			if err := json.Unmarshal(line, &ch); err != nil {
				continue
			}
			content := ch.Message.Content
			if cmd, ok := tryParseCommand(content); ok {
				select {
				case out <- Chunk{Command: cmd}:
				case <-ctx.Done():
					return
				}
				continue
			}
			select {
			case out <- Chunk{Text: content}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// tryParseCommand 尝试从一段文本里提取 JSON 命令块。
// 接受裸 JSON，或被 ```json ... ``` 包裹的。
func tryParseCommand(s string) (*CommandSuggestion, bool) {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, "```json")
	t = strings.TrimPrefix(t, "```")
	t = strings.TrimSuffix(t, "```")
	t = strings.TrimSpace(t)
	if !strings.HasPrefix(t, "{") {
		return nil, false
	}
	var c CommandSuggestion
	if err := json.Unmarshal([]byte(t), &c); err != nil {
		return nil, false
	}
	if c.Command == "" {
		return nil, false
	}
	return &c, true
}
