package store

import (
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	// 注册纯 Go SQLite 驱动（modernc.org/sqlite），驱动名为 "sqlite"。
	_ "modernc.org/sqlite"

	"ops-mate/internal/store/crypto"
)

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
	if err := gormDB.Exec(schemaSQL).Error; err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	key, err := crypto.MasterKey(dir)
	if err != nil {
		return nil, fmt.Errorf("master key: %w", err)
	}
	return &DB{gorm: gormDB, key: key}, nil
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
