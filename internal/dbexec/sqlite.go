package dbexec

import (
	_ "modernc.org/sqlite" // 注册 sqlite 驱动到 database/sql

	"ops-mate/internal/connector"
)

// init 注册 SQLite driver：本地文件类型，无 host/port/user。
func init() {
	connector.Register(&connector.Driver{
		Protocol:  "sqlite",
		Name:      "SQLite",
		NeedsHost: false,
		Params: []connector.ParamSchema{
			{Key: "filePath", Label: "数据库文件路径", Type: connector.ParamFile, Required: true,
				Placeholder: "C:\\data\\app.db"},
		},
		SkillPack: connector.SkillPack{Prompt: dbSkillPrompt, Guardrail: connector.GuardrailSQL},
		New: func(cfg connector.Config) (connector.Capability, error) {
			ex := NewExecutor(Host{Driver: "sqlite", Database: paramString(cfg, "filePath")})
			return &connectorQueryRunner{e: ex}, nil
		},
	})
}
