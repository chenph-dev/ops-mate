// Package es 注册 Elasticsearch 连接器（KindDB 型，实现 QueryRunner + Pingable）。
package es

import (
	"ops-mate/internal/connector"
	"ops-mate/internal/einoagent/prompt"
)

func init() {
	connector.Register(&connector.Connector{
		Protocol:  "elasticsearch",
		Name:      "Elasticsearch",
		Kind:      connector.KindDB,
		NeedsHost: true,
		// host/port/password 走通用区块（与 mysql/redis 一致）；
		// 此处仅登记 es 特有参数（默认索引 / API Key / 跳过证书校验）。
		Params: []connector.ParamSchema{
			{Key: "index", Label: "默认索引", Type: connector.ParamString, Required: false, Placeholder: "留空则查询时指定"},
			{Key: "apiKey", Label: "API Key", Type: connector.ParamString, Required: false, Placeholder: "留空则用用户名密码"},
			{Key: "skipVerify", Label: "跳过证书校验", Type: connector.ParamString, Required: false, Placeholder: "HTTPS 自签时填 true"},
		},
		SkillPack: connector.SkillPack{
			Prompt:    prompt.EsPrompt,
			Guardrail: connector.GuardrailES,
		},
		New: func(cfg connector.Config) (connector.Capability, error) {
			return NewExecutor(cfg)
		},
	})
}
