package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClaude_StreamsSSE(t *testing.T) {
	sse := strings.Join([]string{
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"我先\"}}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"看看\"}}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"{\\\"command\\\":\\\"top -bn1\\\",\\\"why\\\":\\\"查 CPU\\\",\\\"risk\\\":\\\"low\\\"}\"}}\n\n",
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
	}, "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-test" {
			t.Errorf("缺 x-api-key")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sse))
	}))
	defer srv.Close()

	c := NewClaude(srv.URL, "sk-test", "claude-sonnet-5")
	ch, err := c.Chat(context.Background(), []Message{{Role: RoleUser, Content: "cpu 高"}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	var text strings.Builder
	var cmd *CommandSuggestion
	for ck := range ch {
		if ck.Command != nil {
			cmd = ck.Command
		}
		text.WriteString(ck.Text)
	}
	if text.String() != "我先看看" {
		t.Fatalf("文本 = %q", text.String())
	}
	if cmd == nil || cmd.Command != "top -bn1" {
		t.Fatalf("未解析命令: %+v", cmd)
	}
}
