package hosts

import (
	"context"
	"path/filepath"
	"testing"

	"ops-mate/internal/handler/base"
	"ops-mate/internal/store"
	hoststore "ops-mate/internal/store/hosts"
)

func TestTestConnection_SQLite(t *testing.T) {
	base.SetCtx(context.Background()) // 模拟 Wails OnStartup
	t.Setenv("APPDATA", t.TempDir())
	app, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() {
		sqlDB, _ := app.GORM().DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	h := NewHostsHandler(hoststore.NewHostsStore(app), nil)
	ok, err := h.TestConnection(hoststore.HostInput{
		Protocol: "sqlite",
		Params:   map[string]any{"filePath": filepath.Join(t.TempDir(), "t.db")},
	})
	if err != nil {
		t.Fatalf("TestConnection(sqlite): %v", err)
	}
	if !ok {
		t.Fatal("sqlite 连接测试应成功")
	}
}
