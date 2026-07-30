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

// Claude 调用 Anthropic Messages API（SSE 流式）。
type Claude struct {
	baseURL string
	apiKey  string
	model   string
}

func NewClaude(baseURL, apiKey, model string) *Claude {
	return &Claude{baseURL: baseURL, apiKey: apiKey, model: model}
}

type claudeReq struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Stream    bool            `json:"stream"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (c *Claude) Chat(ctx context.Context, msgs []Message) (<-chan Chunk, error) {
	var cm []claudeMessage
	for _, m := range msgs {
		role := string(m.Role)
		content := m.Content
		if m.Role == RoleTool {
			role = "user"
			content = "[执行结果]\n" + m.ToolResult
		}
		cm = append(cm, claudeMessage{Role: role, Content: content})
	}
	payload := claudeReq{
		Model: c.model, MaxTokens: 2048, Stream: true,
		System: SystemPrompt, Messages: cm,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("claude request: %w", err)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("claude status %d", resp.StatusCode)
	}
	out := make(chan Chunk, 16)
	go func() {
		defer resp.Body.Close()
		defer close(out)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			var evt struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue
			}
			if evt.Type != "content_block_delta" || evt.Delta.Text == "" {
				continue
			}
			text := evt.Delta.Text
			if cmd, ok := tryParseCommand(text); ok {
				select {
				case out <- Chunk{Command: cmd}:
				case <-ctx.Done():
					return
				}
				continue
			}
			select {
			case out <- Chunk{Text: text}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}
