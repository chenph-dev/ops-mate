package connector

import (
	"testing"

	_ "ops-mate/internal/dbexec" // 注册 mysql/postgres/sqlite driver
)

func TestListDrivers_ContainsRegistered(t *testing.T) {
	h := NewConnectorHandler()
	list := h.ListDrivers()
	byProtocol := map[string]DriverMeta{}
	for _, d := range list {
		byProtocol[d.Protocol] = d
	}
	for _, proto := range []string{"mysql", "postgres", "sqlite"} {
		d, ok := byProtocol[proto]
		if !ok {
			t.Fatalf("ListDrivers 应包含 %q", proto)
		}
		if d.Name == "" || len(d.Params) == 0 {
			t.Errorf("%q 元信息不完整: %+v", proto, d)
		}
	}
	sqlite := byProtocol["sqlite"]
	if sqlite.NeedsHost {
		t.Error("sqlite NeedsHost 应为 false（本地文件）")
	}
	if len(sqlite.Params) != 1 || sqlite.Params[0].Key != "filePath" {
		t.Errorf("sqlite Params schema 应为 filePath: %+v", sqlite.Params)
	}
	mysql := byProtocol["mysql"]
	if !mysql.NeedsHost {
		t.Error("mysql NeedsHost 应为 true")
	}
	if len(mysql.Params) != 1 || mysql.Params[0].Key != "database" {
		t.Errorf("mysql Params schema 应为 database: %+v", mysql.Params)
	}
}
