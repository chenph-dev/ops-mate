// Package connector 提供连接类型注册表的只读 API（前端表单/面板驱动的单一事实来源）。
package connector

import (
	connector "ops-mate/internal/connector"
	_ "ops-mate/internal/es"       // 登记 Elasticsearch 连接器
	_ "ops-mate/internal/register" // 登记内置命令型驱动（ssh/winrm），供 ListConnectors 输出
	_ "ops-mate/internal/redis"    // 登记 Redis 连接器
)

// ConnectorHandler 暴露连接类型元信息给前端（wails 绑定）。
type ConnectorHandler struct{}

// NewConnectorHandler 构造 ConnectorHandler。
func NewConnectorHandler() *ConnectorHandler {
	return &ConnectorHandler{}
}

// ConnectorMeta 前端可见的连接类型元信息（映射自 connector.Connector，去除 New 等内部字段）。
type ConnectorMeta struct {
	Protocol    string                  `json:"protocol"`
	Name        string                  `json:"name"`
	NeedsHost   bool                    `json:"needsHost"`
	Params      []connector.ParamSchema `json:"params"`
	Kind        string                  `json:"kind"` // 恒为 "db"/"command"（归一化，永不为空）
	CommandKind string                  `json:"commandKind,omitempty"`
}

// ListConnectors 返回注册表中全部连接类型元信息，供前端动态表单/面板按 protocol 复用。
func (h *ConnectorHandler) ListConnectors() []ConnectorMeta {
	list := connector.List()
	out := make([]ConnectorMeta, 0, len(list))
	for _, c := range list {
		kind := "db"
		if c.IsCommand() {
			kind = "command"
		}
		out = append(out, ConnectorMeta{
			Protocol: c.Protocol, Name: c.Name, NeedsHost: c.NeedsHost,
			Params: c.Params, Kind: kind, CommandKind: c.CommandKind,
		})
	}
	return out
}
