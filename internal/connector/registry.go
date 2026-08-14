package connector

import (
	"fmt"
	"sort"
)

// drivers 全局驱动注册表，key = protocol。
var drivers = make(map[string]*Driver)

// Register 登记一个 Driver。Protocol 必须非空，否则 panic。
func Register(d *Driver) {
	if d == nil || d.Protocol == "" {
		panic("connector: Driver 必须提供非空 Protocol")
	}
	drivers[d.Protocol] = d
}

// Get 按 protocol 查询 Driver；未注册返回 nil。
func Get(protocol string) *Driver {
	return drivers[protocol]
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
