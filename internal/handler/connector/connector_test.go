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
	if mysql.Kind != "db" {
		t.Errorf("mysql Kind 应为 db, got %q", mysql.Kind)
	}

	// 命令型驱动（ssh/winrm）也应暴露，Kind 归一化为 command
	for _, proto := range []string{"ssh", "winrm"} {
		d, ok := byProtocol[proto]
		if !ok {
			t.Fatalf("ListDrivers 应包含命令型 %q", proto)
		}
		if d.Kind != "command" || d.CommandKind != proto {
			t.Errorf("%q 应为 command 型且 CommandKind=%q, got %+v", proto, proto, d)
		}
	}
}
