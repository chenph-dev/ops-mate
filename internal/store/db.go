package store

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	// 注册纯 Go SQLite 驱动（modernc.org/sqlite），驱动名为 "sqlite"。
	_ "modernc.org/sqlite"

	"ops-mate/internal/store/crypto"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB 持有 GORM 实例与主密钥，供各子包 store 共享。
type DB struct {
	gorm *gorm.DB
	key  []byte
}

// Open 打开（必要时创建）ops-mate.db 并执行建表迁移。
// 数据库文件位于用户数据目录（Windows: %APPDATA%/ops-mate）。
func Open() (*DB, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir data dir: %w", err)
	}
	gormDB, err := gorm.Open(sqlite.New(sqlite.Config{
		DSN:        filepath.Join(dir, "ops-mate.db"),
		DriverName: "sqlite", // 使用 modernc.org/sqlite（纯 Go，无需 CGO）
	}), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open gorm: %w", err)
	}
	if err := gormDB.Exec(`PRAGMA foreign_keys = ON;`).Error; err != nil {
		return nil, err
	}
	if err := runMigrations(gormDB); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	key, err := crypto.MasterKey(dir)
	if err != nil {
		return nil, fmt.Errorf("master key: %w", err)
	}
	return &DB{gorm: gormDB, key: key}, nil
}

// runMigrations 用 golang-migrate 执行版本化迁移（embedded SQL）。
// 迁移文件在 internal/store/migrations/，按 000001_init.up.sql 等版本号顺序执行。
func runMigrations(gormDB *gorm.DB) error {
	raw, err := gormDB.DB()
	if err != nil {
		return err
	}
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	driver, err := migratesqlite.WithInstance(raw, &migratesqlite.Config{})
	if err != nil {
		return fmt.Errorf("migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func dataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ops-mate"), nil
}

// GORM 返回底层 GORM 实例，供子包执行查询。
func (d *DB) GORM() *gorm.DB {
	return d.gorm
}

// Encrypt 委托 crypto 包加密。
func (d *DB) Encrypt(plaintext []byte) ([]byte, error) {
	return crypto.Encrypt(d.key, plaintext)
}

// Decrypt 委托 crypto 包解密。
func (d *DB) Decrypt(blob []byte) ([]byte, error) {
	return crypto.Decrypt(d.key, blob)
}
