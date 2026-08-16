// Package connector 提供连接类型注册表的只读 API（前端表单/面板驱动的单一事实来源）。
package connector

import (
	connector "ops-mate/internal/connector"
	_ "ops-mate/internal/register" // 登记内置命令型驱动（ssh/winrm），供 ListDrivers 输出
)

// ConnectorHandler 暴露连接类型元信息给前端（wails 绑定）。
type ConnectorHandler struct{}

// NewConnectorHandler 构造 ConnectorHandler。
func NewConnectorHandler() *ConnectorHandler {
	return &ConnectorHandler{}
}

// DriverMeta 前端可见的连接类型元信息（映射自 connector.Driver，去除 New 等内部字段）。
type DriverMeta struct {
	Protocol  string                  `json:"protocol"`
	Name      string                  `json:"name"`
	NeedsHost bool                    `json:"needsHost"`
	Params    []connector.ParamSchema `json:"params"`
}

// ListDrivers 返回注册表中全部连接类型元信息，供前端动态表单/面板按 protocol 复用。
func (h *ConnectorHandler) ListDrivers() []DriverMeta {
	list := connector.List()
	out := make([]DriverMeta, 0, len(list))
	for _, d := range list {
		out = append(out, DriverMeta{
			Protocol: d.Protocol, Name: d.Name, NeedsHost: d.NeedsHost,
			Params: d.Params,
		})
	}
	return out
}
