package dbexec

import (
	"context"
	"path/filepath"
	"testing"

	"ops-mate/internal/connector"
)

// TestSQLiteRealQuery 用临时文件真实读写，验证 sqlite 执行链路。
func TestSQLiteRealQuery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	ex := NewExecutor(Host{Driver: "sqlite", Database: dbPath})
	ctx := context.Background()

	if _, err := ex.Exec(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := ex.Exec(ctx, "INSERT INTO users (name) VALUES ('alice')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	res, err := ex.Query(ctx, "SELECT * FROM users")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Columns) != 2 || len(res.Rows) != 1 {
		t.Fatalf("result 异常: cols=%v rows=%v", res.Columns, res.Rows)
	}
}

// TestSQLiteSchema 验证 sqlite 的 schema 树（表 + 列）。
func TestSQLiteSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	ex := NewExecutor(Host{Driver: "sqlite", Database: dbPath})
	ctx := context.Background()

	if _, err := ex.Exec(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	s, err := ex.Schema(ctx)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	if len(s.Tables) != 1 || s.Tables[0].Name != "users" {
		t.Fatalf("schema 异常: %+v", s.Tables)
	}
	if len(s.Tables[0].Columns) != 2 {
		t.Fatalf("users 应有 2 列, got %d", len(s.Tables[0].Columns))
	}
}

// TestSQLiteDriverRegistered 验证 driver 注册：NeedsHost=false + filePath 参数 + QueryRunner。
func TestSQLiteDriverRegistered(t *testing.T) {
	d := connector.Get("sqlite")
	if d == nil {
		t.Fatal("sqlite driver 未注册")
	}
	if d.NeedsHost {
		t.Error("sqlite NeedsHost 应为 false（本地文件）")
	}
	foundFileParam := false
	for _, p := range d.Params {
		if p.Key == "filePath" {
			foundFileParam = true
		}
	}
	if !foundFileParam {
		t.Errorf("sqlite Params 应含 filePath, got %+v", d.Params)
	}

	cap, err := connector.New("sqlite", connector.Config{
		Params: map[string]any{"filePath": filepath.Join(t.TempDir(), "x.db")},
	})
	if err != nil {
		t.Fatalf("New(sqlite): %v", err)
	}
	if _, ok := cap.(connector.QueryRunner); !ok {
		t.Fatalf("sqlite 应实现 QueryRunner, got %T", cap)
	}
}
