package connector

import (
	"fmt"
	"sort"
	"strings"
)

// connectors 全局驱动注册表，key = protocol。
var connectors = make(map[string]*Connector)

// Register 登记一个 Connector。Protocol 必须非空，否则 panic。注册 key 统一小写，
// 使 Get 大小写不敏感（资产录入/前端可能传混合大小写协议名）。
// Kind 必须显式声明（db/command），命令型必须提供 CommandKind——
// 防止协议分派静默退化；SkillPack.Guardrail 必须为合法枚举值（空 = 默认 linux），
// 防止自由字符串与 guardrail 包封闭协议集漂移。
func Register(c *Connector) {
	if c == nil || c.Protocol == "" {
		panic("connector: Connector 必须提供非空 Protocol")
	}
	switch c.Kind {
	case KindDB:
	case KindCommand:
		if c.CommandKind == "" {
			panic(fmt.Sprintf("connector: 命令型驱动 %q 必须提供 CommandKind", c.Protocol))
		}
	default:
		panic(fmt.Sprintf("connector: 驱动 %q 的 Kind %q 非法（必须显式声明 db 或 command）", c.Protocol, c.Kind))
	}
	g := c.SkillPack.Guardrail
	if g != "" && g != GuardrailSQL && g != GuardrailLinux &&
		g != GuardrailWindows && g != GuardrailRedis {
		panic(fmt.Sprintf("connector: 驱动 %q 的 Guardrail %q 非法", c.Protocol, g))
	}
	connectors[strings.ToLower(c.Protocol)] = c
}

// Get 按 protocol 查询 Connector（大小写不敏感）；未注册返回 nil。
func Get(protocol string) *Connector {
	return connectors[strings.ToLower(protocol)]
}

// List 返回所有已注册 Connector，按 Name 排序（供前端 ListConnectors）。
func List() []*Connector {
	out := make([]*Connector, 0, len(connectors))
	for _, c := range connectors {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// New 按 protocol 构造能力对象。
func New(protocol string, cfg Config) (Capability, error) {
	c := Get(protocol)
	if c == nil {
		return nil, fmt.Errorf("未知的连接类型: %q", protocol)
	}
	if c.New == nil {
		return nil, fmt.Errorf("连接类型 %q 未实现构造器", protocol)
	}
	return c.New(cfg)
}
