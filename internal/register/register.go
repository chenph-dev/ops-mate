// Package register 登记内置命令型连接驱动（ssh/winrm）。
// 由 handler/base、einoagent/session、handler/connector blank import，
// 使注册成为显式的单一事实来源，且各包测试二进制也能拿到完整注册表。
package register

import (
	"ops-mate/internal/connector"
	"ops-mate/internal/einoagent/prompt"
)

func init() {
	connector.Register(&connector.Driver{
		Protocol:    "ssh",
		Name:        "SSH",
		Kind:        connector.KindCommand,
		CommandKind: connector.CommandSSH,
		NeedsHost:   true,
		SkillPack: connector.SkillPack{
			Prompt:    prompt.SshPrompt,
			Guardrail: connector.GuardrailLinux,
		},
	})
	connector.Register(&connector.Driver{
		Protocol:    "winrm",
		Name:        "WinRM",
		Kind:        connector.KindCommand,
		CommandKind: connector.CommandWinRM,
		NeedsHost:   true,
		SkillPack: connector.SkillPack{
			Prompt:    prompt.WinrmPrompt,
			Guardrail: connector.GuardrailWindows,
		},
	})
}
