package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllama_StreamsTextAndCommand(t *testing.T) {
	body := strings.Join([]string{
		`{"message":{"role":"assistant","content":"我先"},"done":false}`,
		`{"message":{"role":"assistant","content":"看看进程"},"done":false}`,
		`{"message":{"role":"assistant","content":"{\"command\":\"top -bn1\",\"why\":\"查 CPU\",\"risk\":\"low\"}"},"done":false}`,
		`{"message":{"role":"assistant","content":""},"done":true}`,
	}, "\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewOllama(srv.URL, "llama3")
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
	if text.String() != "我先看看进程" {
		t.Fatalf("文本拼接 = %q", text.String())
	}
	if cmd == nil || cmd.Command != "top -bn1" {
		t.Fatalf("未解析出命令: %+v", cmd)
	}
}
