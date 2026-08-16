package prompt

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	_ "ops-mate/internal/dbexec" // 注册 mysql/postgres/sqlite driver（PromptForProtocol 取 SkillPack）
)

func TestPromptForProtocol_SshDefault(t *testing.T) {
	if got := PromptForProtocol("ssh"); !strings.Contains(got, "execute_command") {
		t.Errorf("ssh 片段应含 execute_command，得到 %q", got)
	}
	if got := PromptForProtocol(""); !strings.Contains(got, "execute_command") {
		t.Errorf("空协议应回退 ssh 片段，得到 %q", got)
	}
}

func TestPromptForProtocol_Winrm(t *testing.T) {
	got := PromptForProtocol("winrm")
	if !strings.Contains(got, "PowerShell 命令") {
		t.Errorf("winrm 片段应含 PowerShell 语义，得到 %q", got)
	}
	if !strings.Contains(got, "diskpart clean") {
		t.Errorf("winrm 片段应含 Windows 危险操作提示，得到 %q", got)
	}
}

func TestPromptForProtocol_DbDriver(t *testing.T) {
	got := PromptForProtocol("mysql")
	if !strings.Contains(got, "execute_sql") {
		t.Errorf("mysql 应取注册表 SkillPack.Prompt（含 execute_sql），得到 %q", got)
	}
}

func TestBuildSystemMessages_TerminalContext(t *testing.T) {
	msgs, err := BuildSystemMessages(context.Background(), map[string]any{
		"HostName": "web01", "Memory": "", "TerminalContext": "/dev/sda1 52% /\n", "SkillsCatalog": "",
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
	if !strings.Contains(content, "不可信数据") {
		t.Errorf("终端上下文应标注为不可信数据: %q", content)
	}
	if !strings.Contains(content, "/dev/sda1") {
		t.Errorf("应含终端内容: %q", content)
	}
}

func TestBuildSystemMessages_NoTerminalContext(t *testing.T) {
	msgs, err := BuildSystemMessages(context.Background(), map[string]any{
		"HostName": "", "Memory": "", "TerminalContext": "", "SkillsCatalog": "",
	})
	if err != nil {
		t.Fatalf("BuildSystemMessages: %v", err)
	}
	content := msgs[0].Content
	if strings.Contains(content, "终端最近输出") {
		t.Errorf("无终端上下文时不应输出该段落: %q", content)
	}
}

func TestBuildSystemMessages_DefaultSshPrompt(t *testing.T) {
	msgs, err := BuildSystemMessages(context.Background(), map[string]any{
		"HostName": "", "Memory": "", "TerminalContext": "", "SkillsCatalog": "",
	})
	if err != nil {
		t.Fatalf("BuildSystemMessages: %v", err)
	}
	content := msgs[0].Content
	if !strings.Contains(content, "Linux 系统") {
		t.Errorf("默认应含 Linux 语义（ssh 片段），得到 %q", content)
	}
	if strings.Contains(content, "PowerShell 命令") {
		t.Errorf("默认不应含 PowerShell 语义: %q", content)
	}
	if strings.Contains(content, "diskpart clean") {
		t.Errorf("默认不应含 Windows 危险操作提示: %q", content)
	}
}

func TestBuildSystemMessages_WinrmPromptInjected(t *testing.T) {
	msgs, err := BuildSystemMessages(context.Background(), map[string]any{
		"HostName": "", "Memory": "", "TerminalContext": "", "SkillsCatalog": "",
		"Prompt": WinrmPrompt,
	})
	if err != nil {
		t.Fatalf("BuildSystemMessages: %v", err)
	}
	content := msgs[0].Content
	if !strings.Contains(content, "Windows 系统") {
		t.Errorf("winrm Prompt 应含 Windows 语义: %q", content)
	}
	if !strings.Contains(content, "PowerShell 命令") {
		t.Errorf("winrm Prompt 应含 PowerShell 语义: %q", content)
	}
	if strings.Contains(content, "Linux 系统") {
		t.Errorf("winrm Prompt 不应含 Linux 语义: %q", content)
	}
	if !strings.Contains(content, "diskpart clean") {
		t.Errorf("winrm Prompt 应含 Windows 危险操作提示: %q", content)
	}
}

func TestBuildSystemMessages_DbPromptInjected(t *testing.T) {
	msgs, err := BuildSystemMessages(context.Background(), map[string]any{
		"HostName": "", "Memory": "", "TerminalContext": "", "SkillsCatalog": "",
		"Prompt": PromptForProtocol("mysql"),
	})
	if err != nil {
		t.Fatalf("BuildSystemMessages: %v", err)
	}
	content := msgs[0].Content
	if !strings.Contains(content, "execute_sql") {
		t.Errorf("db Prompt 应含 execute_sql 语义: %q", content)
	}
}

func TestBuildSystemMessages_HostnameKeepsSemantics(t *testing.T) {
	msgs, err := BuildSystemMessages(context.Background(), map[string]any{
		"HostName": "win01", "Memory": "", "TerminalContext": "", "SkillsCatalog": "",
		"Prompt": WinrmPrompt,
	})
	if err != nil {
		t.Fatalf("BuildSystemMessages: %v", err)
	}
	content := msgs[0].Content
	if !strings.Contains(content, "PowerShell 命令") {
		t.Errorf("有资产名时也应含 PowerShell 语义: %q", content)
	}
	if !strings.Contains(content, "目标资产 win01") {
		t.Errorf("应含资产名: %q", content)
	}
}

func TestBuildSystemMessages_PromptLiteralNotReParsed(t *testing.T) {
	// 片段中的 {{ }} 应原样输出（模板不二次解析变量值）
	msgs, err := BuildSystemMessages(context.Background(), map[string]any{
		"HostName": "", "Memory": "", "TerminalContext": "", "SkillsCatalog": "",
		"Prompt": "字面量 {{.HostName}} 不变",
	})
	if err != nil {
		t.Fatalf("BuildSystemMessages: %v", err)
	}
	if !strings.Contains(msgs[0].Content, "字面量 {{.HostName}} 不变") {
		t.Errorf("片段应原样渲染（不二次解析），得到 %q", msgs[0].Content)
	}
}

func TestPromptForProtocol_CaseInsensitive(t *testing.T) {
	if got := PromptForProtocol("MYSQL"); !strings.Contains(got, "execute_sql") {
		t.Errorf("大写 MYSQL 应命中注册表 db 片段，得到 %q", got)
	}
	if got := PromptForProtocol("  WinRM  "); !strings.Contains(got, "PowerShell 命令") {
		t.Errorf("带空白混合大小写 WinRM 应返回 winrm 片段，得到 %q", got)
	}
}
