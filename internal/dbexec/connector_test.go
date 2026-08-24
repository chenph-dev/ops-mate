package dbexec

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ops-mate/internal/connector"
)

func TestMysqlPostgresRegistered(t *testing.T) {
	for _, proto := range []string{"mysql", "postgres"} {
		d := connector.Get(proto)
		if d == nil {
			t.Fatalf("driver %q 未注册", proto)
		}
		if !d.NeedsHost {
			t.Errorf("%q NeedsHost 应为 true", proto)
		}
		if d.SkillPack.Guardrail != connector.GuardrailSQL {
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
	// sqlite 测试连接要求文件已存在，先创建
	path := filepath.Join(t.TempDir(), "x.db")
	if f, err := os.Create(path); err != nil {
		t.Fatalf("create: %v", err)
	} else {
		f.Close()
	}
	cap, err := connector.New("sqlite", connector.Config{
		Params: map[string]any{"filePath": path},
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
