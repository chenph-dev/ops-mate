package store

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// migrateAt 构造迁移实例（复用 embedded migrationsFS）。
func migrateAt(t *testing.T, gormDB *gorm.DB) *migrate.Migrate {
	t.Helper()
	raw, err := gormDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	driver, err := migratesqlite.WithInstance(raw, &migratesqlite.Config{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestMigrate010_JdbcToSingleProtocol 验证：存量 jdbc 资产迁移后 protocol 单层 + params_json 含 database。
func TestMigrate010_JdbcToSingleProtocol(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.New(sqlite.Config{
		DSN:        filepath.Join(t.TempDir(), "m.db"),
		DriverName: "sqlite",
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open gorm: %v", err)
	}
	defer func() {
		sqlDB, _ := gormDB.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	m := migrateAt(t, gormDB)
	if err := m.Migrate(9); err != nil {
		t.Fatalf("migrate to 000009: %v", err)
	}

	// 插入存量 jdbc 资产（000009 的 schema 有 protocol/driver/database 列）
	if err := gormDB.Exec(`INSERT INTO hosts (id, name, node_type, protocol, driver, database, addr, port, user, auth_type, created_at)
VALUES ('h1', 'db1', 'host', 'jdbc', 'mysql', 'app', '10.0.0.5', 3306, 'root', 'password', 1)`).Error; err != nil {
		t.Fatalf("insert jdbc host: %v", err)
	}

	if err := m.Migrate(10); err != nil {
		t.Fatalf("migrate to 000010: %v", err)
	}

	var h struct {
		Protocol   string
		ParamsJSON string
	}
	if err := gormDB.Raw("SELECT protocol, params_json FROM hosts WHERE id = 'h1'").Scan(&h).Error; err != nil {
		t.Fatalf("query migrated host: %v", err)
	}
	if h.Protocol != "mysql" {
		t.Errorf("protocol = %q, want mysql", h.Protocol)
	}
	if !strings.Contains(h.ParamsJSON, "app") {
		t.Errorf("params_json 应含 database=app, got %q", h.ParamsJSON)
	}
}

// TestMigrate010_Down 验证：000010 down 可回滚到 000009。
// mysql 资产应回退 protocol='jdbc'、database 从 params_json 回填、params_json 列被删除。
func TestMigrate010_Down(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.New(sqlite.Config{
		DSN:        filepath.Join(t.TempDir(), "m.db"),
		DriverName: "sqlite",
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open gorm: %v", err)
	}
	defer func() {
		sqlDB, _ := gormDB.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	m := migrateAt(t, gormDB)
	if err := m.Migrate(10); err != nil {
		t.Fatalf("migrate to 000010: %v", err)
	}

	// 插入 mysql 资产（000010 schema：protocol=mysql + params_json 含 database）
	if err := gormDB.Exec(`INSERT INTO hosts (id, name, node_type, protocol, driver, database, params_json, addr, port, user, auth_type, created_at)
VALUES ('h1', 'db1', 'host', 'mysql', 'mysql', '', '{"database":"app"}', '10.0.0.5', 3306, 'root', 'password', 1)`).Error; err != nil {
		t.Fatalf("insert mysql host: %v", err)
	}

	if err := m.Migrate(9); err != nil {
		t.Fatalf("migrate down to 000009: %v", err)
	}

	var h struct {
		Protocol string
		Driver   string
		Database string
	}
	if err := gormDB.Raw("SELECT protocol, driver, database FROM hosts WHERE id = 'h1'").Scan(&h).Error; err != nil {
		t.Fatalf("query rolled-back host: %v", err)
	}
	if h.Protocol != "jdbc" {
		t.Errorf("protocol = %q, want jdbc", h.Protocol)
	}
	if h.Driver != "mysql" {
		t.Errorf("driver = %q, want mysql", h.Driver)
	}
	if h.Database != "app" {
		t.Errorf("database = %q, want app", h.Database)
	}

	// params_json 列应已删除：SELECT 应报 no such column
	var got struct{ ParamsJSON string }
	if err := gormDB.Raw("SELECT params_json FROM hosts WHERE id = 'h1'").Scan(&got).Error; err == nil {
		t.Errorf("params_json 列应已删除，但查询未报错")
	}
}
