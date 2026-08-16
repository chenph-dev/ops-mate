// Package connector 提供连接类型（Driver）注册表与能力接口抽象。
// 一种连接类型 = 一个 Driver 声明（参数 schema + 能力 + SkillPack），
// 注册后即可被资产录入表单、执行器解析、AI 工具装配按 protocol 复用。
package connector

// ParamType 参数控件类型，供前端动态表单渲染。
type ParamType string

const (
	ParamString ParamType = "string"
	ParamFile   ParamType = "file"
)

// ParamOption select 参数的候选项。
type ParamOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ParamSchema 描述一个 driver 专属连接参数。
type ParamSchema struct {
	Key         string        `json:"key"`
	Label       string        `json:"label"`
	Type        ParamType     `json:"type"`
	Required    bool          `json:"required"`
	Default     any           `json:"default,omitempty"`
	Placeholder string        `json:"placeholder,omitempty"`
	Options     []ParamOption `json:"options,omitempty"`
}
