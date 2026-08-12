package termctx

import (
	"strings"
	"testing"
)

func TestRingBuffer_AppendWithinLimit(t *testing.T) {
	b := NewRingBuffer(100)
	b.Append([]byte("hello"))
	b.Append([]byte(" world"))
	if got := string(b.Bytes()); got != "hello world" {
		t.Errorf("Bytes = %q, want %q", got, "hello world")
	}
}

func TestRingBuffer_TruncatesOldest(t *testing.T) {
	b := NewRingBuffer(10)
	b.Append([]byte("0123456789ABCDEF")) // 16 > 10，应只保留后 10
	if got := string(b.Bytes()); got != "6789ABCDEF" {
		t.Errorf("Bytes = %q, want %q", got, "6789ABCDEF")
	}
}

func TestRingBuffer_Reset(t *testing.T) {
	b := NewRingBuffer(100)
	b.Append([]byte("x"))
	b.Reset()
	if got := b.Bytes(); len(got) != 0 {
		t.Errorf("Reset 后 Bytes = %q, want 空", got)
	}
}

func TestRingBuffer_ZeroMaxNoOp(t *testing.T) {
	b := NewRingBuffer(0)
	b.Append([]byte("x"))
	if got := b.Bytes(); len(got) != 0 {
		t.Errorf("max=0 应不缓存，got %q", got)
	}
}

func TestClean_StripsANSI(t *testing.T) {
	raw := []byte("\x1b[32mOK\x1b[0m\n\x1b[1m done\x1b[0m")
	got := Clean(raw)
	if strings.Contains(got, "\x1b") {
		t.Errorf("清洗后仍含 ANSI 转义: %q", got)
	}
	if !strings.Contains(got, "OK") || !strings.Contains(got, "done") {
		t.Errorf("清洗后缺内容: %q", got)
	}
}

func TestClean_StripsCommandEcho(t *testing.T) {
	raw := []byte("root@web01:~$ df -h\r\n/dev/sda1  40G  20G  18G  52% /\r\n")
	got := Clean(raw)
	if strings.Contains(got, "df -h") {
		t.Errorf("命令行回显应被去掉: %q", got)
	}
	if !strings.Contains(got, "/dev/sda1") {
		t.Errorf("命令输出应保留: %q", got)
	}
}

func TestClean_StripsShellPromptPrefix(t *testing.T) {
	raw := []byte("$ uptime\r\n 10:00 up 3 days\r\n# whoami\r\nroot\r\n")
	got := Clean(raw)
	if strings.Contains(got, "uptime") || strings.Contains(got, "whoami") {
		t.Errorf("提示符行应去掉: %q", got)
	}
	if !strings.Contains(got, "up 3 days") || !strings.Contains(got, "root") {
		t.Errorf("输出应保留: %q", got)
	}
}

func TestClean_DeduplicatesConsecutiveLines(t *testing.T) {
	raw := []byte("100%\r\n100%\r\n100%\r\ndone\r\n")
	got := Clean(raw)
	if strings.Count(got, "100%") != 1 {
		t.Errorf("连续重复行应合并为一行: %q", got)
	}
}

func TestClean_TruncatesLongLine(t *testing.T) {
	long := strings.Repeat("a", MaxLineBytes+500)
	got := Clean([]byte(long + "\n"))
	if len(got) > MaxLineBytes+10 {
		t.Errorf("超长行应截断到 MaxLineBytes 附近，got %d", len(got))
	}
}

func TestClean_TruncatesTotal(t *testing.T) {
	// 多行累计超过 MaxTotalBytes → 只保留头部
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString(strings.Repeat("x", 300))
		sb.WriteString("\n")
	}
	got := Clean([]byte(sb.String()))
	if len(got) > MaxTotalBytes+10 {
		t.Errorf("总长应截断到 MaxTotalBytes 附近，got %d", len(got))
	}
}

func TestClean_EmptyReturnsEmpty(t *testing.T) {
	if got := Clean(nil); got != "" {
		t.Errorf("空输入应返回空串，got %q", got)
	}
	if got := Clean([]byte("\x1b[0m")); got != "" {
		t.Errorf("清洗后为空应返回空串，got %q", got)
	}
}
