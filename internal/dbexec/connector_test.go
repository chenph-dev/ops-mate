package dbexec

import (
	"context"
	"path/filepath"
	"testing"

	"ops-mate/internal/connector"
)

func hasCap(caps []string, want string) bool {
	for _, c := range caps {
		if c == want {
			return true
		}
	}
	return false
}

func TestMysqlPostgresRegistered(t *testing.T) {
	for _, proto := range []string{"mysql", "postgres"} {
		d := connector.Get(proto)
		if d == nil {
			t.Fatalf("driver %q 未注册", proto)
		}
		if !d.NeedsHost {
			t.Errorf("%q NeedsHost 应为 true", proto)
		}
		if !hasCap(d.Capabilities, "query") || !hasCap(d.Capabilities, "objectTree") {
			t.Errorf("%q 应声明 query/objectTree 能力, got %v", proto, d.Capabilities)
		}
		if d.SkillPack.Guardrail != "sql" {
			t.Errorf("%q guardrail 应为 sql, got %q", proto, d.SkillPack.Guardrail)
		}
	}
}

func TestMysqlNewReturnsCapabilities(t *testing.T) {
	cap, err := connector.New("mysql", connector.Config{
		Addr: "127.0.0.1", Port: 3306, User: "root", Password: "x",
		Params: map[string]any{"database": "app"},
	})
	if err != nil {
		t.Fatalf("New(mysql): %v", err)
	}
	if _, ok := cap.(connector.QueryRunner); !ok {
		t.Fatalf("mysql 应实现 QueryRunner, got %T", cap)
	}
	if _, ok := cap.(connector.ObjectBrowser); !ok {
		t.Fatalf("mysql 应实现 ObjectBrowser, got %T", cap)
	}
}

func TestPostgresNewReturnsCapabilities(t *testing.T) {
	cap, err := connector.New("postgres", connector.Config{
		Addr: "127.0.0.1", Port: 5432, User: "pg", Password: "x",
		Params: map[string]any{"database": "app"},
	})
	if err != nil {
		t.Fatalf("New(postgres): %v", err)
	}
	if _, ok := cap.(connector.QueryRunner); !ok {
		t.Fatalf("postgres 应实现 QueryRunner, got %T", cap)
	}
}

func TestParamStringHelper(t *testing.T) {
	cfg := connector.Config{Params: map[string]any{"database": "app"}}
	if got := paramString(cfg, "database"); got != "app" {
		t.Errorf("paramString(database) = %q, want app", got)
	}
	if got := paramString(cfg, "missing"); got != "" {
		t.Errorf("paramString(missing) = %q, want empty", got)
	}
}

func TestDBAdapterImplementsPingable(t *testing.T) {
	cap, err := connector.New("sqlite", connector.Config{
		Params: map[string]any{"filePath": filepath.Join(t.TempDir(), "x.db")},
	})
	if err != nil {
		t.Fatalf("New(sqlite): %v", err)
	}
	pingable, ok := cap.(connector.Pingable)
	if !ok {
		t.Fatalf("sqlite 能力应实现 Pingable, got %T", cap)
	}
	ctx := context.Background()
	if err := pingable.Ping(ctx); err != nil {
		t.Fatalf("Ping(sqlite): %v", err)
	}
}
