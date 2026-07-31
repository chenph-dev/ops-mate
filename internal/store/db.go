package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"ops-mate/internal/store/crypto"
)

// DB 持有数据库连接与主密钥，供各子包 store 共享。
type DB struct {
	db  *sql.DB
	key []byte
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
	db, err := sql.Open("sqlite", filepath.Join(dir, "ops-mate.db"))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	key, err := crypto.MasterKey(dir)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("master key: %w", err)
	}
	return &DB{db: db, key: key}, nil
}

func dataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ops-mate"), nil
}

// DB 返回底层数据库连接，供子包执行 SQL。
func (d *DB) DB() *sql.DB {
	return d.db
}

// Encrypt 委托 crypto 包加密。
func (d *DB) Encrypt(plaintext []byte) ([]byte, error) {
	return crypto.Encrypt(d.key, plaintext)
}

// Decrypt 委托 crypto 包解密。
func (d *DB) Decrypt(blob []byte) ([]byte, error) {
	return crypto.Decrypt(d.key, blob)
}
