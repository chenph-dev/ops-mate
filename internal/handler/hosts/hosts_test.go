package hosts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	path := filepath.Join(t.TempDir(), "t.db")
	// sqlite 测试连接要求文件已存在（避免填错路径误建空库），先创建
	if f, err := os.Create(path); err != nil {
		t.Fatalf("create: %v", err)
	} else {
		f.Close()
	}
	ok, err := h.TestConnection(hoststore.HostInput{
		Protocol: "sqlite",
		Params:   map[string]any{"filePath": path},
	})
	if err != nil {
		t.Fatalf("TestConnection(sqlite): %v", err)
	}
	if !ok {
		t.Fatal("sqlite 连接测试应成功")
	}
}

func TestTestConnection_SQLiteMissingFile(t *testing.T) {
	base.SetCtx(context.Background())
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
	_, err = h.TestConnection(hoststore.HostInput{
		Protocol: "sqlite",
		Params:   map[string]any{"filePath": filepath.Join(t.TempDir(), "missing.db")},
	})
	if err == nil || !strings.Contains(err.Error(), "文件不存在") {
		t.Fatalf("sqlite 文件不存在应报错，got %v", err)
	}
}
