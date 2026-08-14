package terminal

import (
	"strings"
	"testing"
)

// 不涉及真实 SSH：直接调用记录方法与读取方法验证 hostID 聚合与清屏重置。
func TestTerminalHandler_TerminalContext(t *testing.T) {
	h := NewTerminalHandler(nil)

	// 无记录 → 空
	if got := h.TerminalContext("h1"); got != "" {
		t.Errorf("无记录应返回空串，got %q", got)
	}

	h.appendTermOutput("h1", []byte("root@h1:~$ df -h\r\n/dev/sda1  40G  20G  18G  52% /\r\n"))
	h.appendTermOutput("h1", []byte("\x1b[32mOK\x1b[0m\r\n"))

	got := h.TerminalContext("h1")
	if !strings.Contains(got, "/dev/sda1") {
		t.Errorf("清洗后应含命令输出: %q", got)
	}
	if !strings.Contains(got, "> df -h") {
		t.Errorf("输入的命令应以 > 前缀保留: %q", got)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("ANSI 应被去掉: %q", got)
	}

	// 不同 host 独立
	if got := h.TerminalContext("h2"); got != "" {
		t.Errorf("h2 无记录应返回空串，got %q", got)
	}
}

func TestTerminalHandler_TerminalContextClearScreen(t *testing.T) {
	h := NewTerminalHandler(nil)
	h.appendTermOutput("h1", []byte("before\r\n"))
	// 清屏转义 → 缓冲清空
	h.appendTermOutput("h1", []byte("\x1b[2J"))
	h.appendTermOutput("h1", []byte("after\r\n"))

	got := h.TerminalContext("h1")
	if strings.Contains(got, "before") {
		t.Errorf("清屏后旧内容应清空: %q", got)
	}
	if !strings.Contains(got, "after") {
		t.Errorf("清屏后新内容应保留: %q", got)
	}
}
