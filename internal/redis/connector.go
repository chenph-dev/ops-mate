// Package redis 注册 Redis 连接器（KindDB 型，实现 QueryRunner + Pingable）。
package redis

import (
	"ops-mate/internal/connector"
	"ops-mate/internal/einoagent/prompt"
)

func init() {
	connector.Register(&connector.Connector{
		Protocol:  "redis",
		Name:      "Redis",
		Kind:      connector.KindDB,
		NeedsHost: true,
		// host/port/password 走通用区块（NeedsHost=true 渲染 addr/port/secret），
		// 与 mysql/postgres 一致，避免前端出现重复的地址/端口/密码输入框；
		// 此处仅登记 redis 特有参数（数据库编号）。
		Params: []connector.ParamSchema{
			{Key: "db", Label: "数据库编号", Type: connector.ParamString, Required: false, Placeholder: "0-15，默认 0"},
		},
		SkillPack: connector.SkillPack{
			Prompt:    prompt.RedisPrompt,
			Guardrail: connector.GuardrailRedis,
		},
		New: func(cfg connector.Config) (connector.Capability, error) {
			return NewExecutor(cfg)
		},
	})
}
