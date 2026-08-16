package connector

import (
	"fmt"
	"sort"
	"strings"
)

// drivers 全局驱动注册表，key = protocol。
var drivers = make(map[string]*Driver)

// Register 登记一个 Driver。Protocol 必须非空，否则 panic。注册 key 统一小写，
// 使 Get 大小写不敏感（资产录入/前端可能传混合大小写协议名）。
// SkillPack.Guardrail 必须为合法枚举值（空 = 默认 linux），否则 panic——
// 防止自由字符串与 guardrail 包封闭协议集漂移（写错字静默降级为 shell 语义）。
func Register(d *Driver) {
	if d == nil || d.Protocol == "" {
		panic("connector: Driver 必须提供非空 Protocol")
	}
	g := d.SkillPack.Guardrail
	if g != "" && g != GuardrailSQL && g != GuardrailLinux &&
		g != GuardrailWindows && g != GuardrailRedis {
		panic(fmt.Sprintf("connector: 驱动 %q 的 Guardrail %q 非法", d.Protocol, g))
	}
	drivers[strings.ToLower(d.Protocol)] = d
}

// Get 按 protocol 查询 Driver（大小写不敏感）；未注册返回 nil。
func Get(protocol string) *Driver {
	return drivers[strings.ToLower(protocol)]
}

// List 返回所有已注册 Driver，按 Name 排序（供前端 ListDrivers）。
func List() []*Driver {
	out := make([]*Driver, 0, len(drivers))
	for _, d := range drivers {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// New 按 protocol 构造能力对象。
func New(protocol string, cfg Config) (Capability, error) {
	d := Get(protocol)
	if d == nil {
		return nil, fmt.Errorf("未知的连接类型: %q", protocol)
	}
	if d.New == nil {
		return nil, fmt.Errorf("连接类型 %q 未实现构造器", protocol)
	}
	return d.New(cfg)
}
