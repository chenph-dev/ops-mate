package base

import (
	"context"
	"path/filepath"
	"testing"

	"ops-mate/internal/connector"
	"ops-mate/internal/store"
	hoststore "ops-mate/internal/store/hosts"
)

func openTestApp(t *testing.T) *store.DB {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	app, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := app.GORM().DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	return app
}

func TestConnFor_SQLite(t *testing.T) {
	app := openTestApp(t)
	s := hoststore.NewHostsStore(app)
	file := filepath.Join(t.TempDir(), "t.db")
	id, err := s.SaveHost(hoststore.HostInput{
		Name: "db", Protocol: "sqlite",
		Params: map[string]any{"filePath": file},
	})
	if err != nil {
		t.Fatalf("SaveHost: %v", err)
	}
	r := NewExecutorResolver(s)
	cap := r.ConnFor(id)
	qr, ok := cap.(connector.QueryRunner)
	if !ok {
		t.Fatalf("ConnFor(sqlite) 应返回 QueryRunner, got %T", cap)
	}
	res, err := qr.Query(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Errorf("SELECT 1 应返回 1 行，得到 %d", len(res.Rows))
	}
}

func TestConnFor_SSHProtocolReturnsNil(t *testing.T) {
	app := openTestApp(t)
	s := hoststore.NewHostsStore(app)
	id, err := s.SaveHost(hoststore.HostInput{
		Name: "h", Addr: "1.1.1.1", Port: 22, User: "u",
		AuthType: "password", Secret: "x",
	})
	if err != nil {
		t.Fatalf("SaveHost: %v", err)
	}
	r := NewExecutorResolver(s)
	if got := r.ConnFor(id); got != nil {
		t.Fatalf("ssh 协议 ConnFor 应为 nil, got %T", got)
	}
}

func TestDbFor_SQLite(t *testing.T) {
	app := openTestApp(t)
	s := hoststore.NewHostsStore(app)
	file := filepath.Join(t.TempDir(), "t.db")
	id, err := s.SaveHost(hoststore.HostInput{
		Name: "db", Protocol: "sqlite",
		Params: map[string]any{"filePath": file},
	})
	if err != nil {
		t.Fatalf("SaveHost: %v", err)
	}
	r := NewExecutorResolver(s)
	ex := r.DbFor(id)
	if ex == nil {
		t.Fatal("DbFor(sqlite) 应为非 nil 执行器")
	}
	res, err := ex.Query(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("DbFor 构造的执行器查询失败: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Errorf("SELECT 1 应返回 1 行，得到 %d", len(res.Rows))
	}
}

func TestDbFor_SSHReturnsNil(t *testing.T) {
	app := openTestApp(t)
	s := hoststore.NewHostsStore(app)
	id, err := s.SaveHost(hoststore.HostInput{
		Name: "h", Addr: "1.1.1.1", Port: 22, User: "u",
		AuthType: "password", Secret: "x",
	})
	if err != nil {
		t.Fatalf("SaveHost: %v", err)
	}
	r := NewExecutorResolver(s)
	if got := r.DbFor(id); got != nil {
		t.Fatalf("ssh 协议 DbFor 应为 nil, got %+v", got)
	}
}

func TestHostFor_RejectsDBProtocol(t *testing.T) {
	app := openTestApp(t)
	s := hoststore.NewHostsStore(app)
	file := filepath.Join(t.TempDir(), "t.db")
	id, err := s.SaveHost(hoststore.HostInput{
		Name: "db", Protocol: "sqlite",
		Params: map[string]any{"filePath": file},
	})
	if err != nil {
		t.Fatalf("SaveHost: %v", err)
	}
	r := NewExecutorResolver(s)
	if _, err := r.HostFor(id); err == nil {
		t.Fatal("HostFor 对数据库资产应返回错误（不支持交互式会话）")
	}
}
