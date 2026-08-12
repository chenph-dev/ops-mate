package prompt

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestBuildSystemMessages_TerminalContext(t *testing.T) {
	// 有终端上下文 → 段落输出且包含内容
	msgs, err := BuildSystemMessages(context.Background(), map[string]any{
		"HostName": "web01", "Memory": "", "TerminalContext": "/dev/sda1 52% /\n",
	})
	if err != nil {
		t.Fatalf("BuildSystemMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Role != schema.System {
		t.Fatalf("期望 1 条 system 消息，得到 %d", len(msgs))
	}
	content := msgs[0].Content
	if !strings.Contains(content, "终端最近输出") {
		t.Errorf("应含终端上下文引导文案: %q", content)
	}
	if !strings.Contains(content, "/dev/sda1") {
		t.Errorf("应含终端内容: %q", content)
	}
}

func TestBuildSystemMessages_NoTerminalContext(t *testing.T) {
	msgs, err := BuildSystemMessages(context.Background(), map[string]any{
		"HostName": "", "Memory": "", "TerminalContext": "",
	})
	if err != nil {
		t.Fatalf("BuildSystemMessages: %v", err)
	}
	content := msgs[0].Content
	if strings.Contains(content, "终端最近输出") {
		t.Errorf("无终端上下文时不应输出该段落: %q", content)
	}
}
