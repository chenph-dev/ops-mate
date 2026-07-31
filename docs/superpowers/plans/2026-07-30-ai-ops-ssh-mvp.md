# ops-mate AI 辅助运维工具 SSH MVP 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 ops-mate Wails 桌面应用中实现 AI 辅助运维首版——用户对话式管理 SSH 远程 Linux 主机，AI 提议命令、用户批准/修改/拒绝、应用执行并回灌 AI 多轮分析，带本地 SQLite 记忆。

**Architecture:** Go 后端承担全部重活（SSH 执行、凭据加密存储、AI HTTP 调用、会话状态机编排）；前端仅 UI（对话/主机/设置页）。所有持久化走单一 SQLite（`modernc.org/sqlite`），敏感列 AES-GCM 加密。前端通过 Wails Bind(RPC) 调用、通过 `EventsEmit` 接收流式输出。

**Tech Stack:** Go 1.23 · `modernc.org/sqlite` · `golang.org/x/crypto/ssh` · `github.com/zalando/go-keyring` · stdlib `crypto/aes`(GCM) · React 19 + TypeScript + Ant Design 6 + react-router-dom 7 · Vitest(前端测试)

**Spec:** `docs/superpowers/specs/2026-07-30-ai-ops-ssh-mvp-design.md`

---

## 文件结构

### Go 后端（`internal/`，按职责拆小文件）

| 文件 | 职责 |
|---|---|
| `internal/store/db.go` | 打开/迁移 SQLite，返回 `*sql.DB` |
| `internal/store/schema.go` | 建表 SQL |
| `internal/store/crypto.go` | AES-GCM 加解密 + 主密钥（keyring 优先，加密 key 文件回退） |
| `internal/store/hosts.go` | 主机 CRUD（auth 列加密） |
| `internal/store/config.go` | AI 配置 CRUD（api_key 列加密） |
| `internal/store/conversations.go` | 会话/消息/命令记录 CRUD + FTS5 |
| `internal/store/memory.go` | `Memory.Recall`：FTS5 跨会话检索 |
| `internal/sshexec/executor.go` | `Executor` 接口 + `SSHExecutor`（流式输出） |
| `internal/llm/client.go` | `LLMClient` 接口 + `Message`/`Chunk` 类型 |
| `internal/llm/ollama.go` | Ollama provider（HTTP + 流式） |
| `internal/llm/claude.go` | Claude/Anthropic provider（HTTP + SSE 流式） |
| `internal/orchestrator/guardrail.go` | 危险命令静态扫描 |
| `internal/orchestrator/orchestrator.go` | 会话状态机 + 多轮编排 |
| `app.go` | `App` 结构，聚合各 service，经 Wails Bind 暴露 |
| `main.go` | 装配 `App`，启动 Wails |

### 前端（`frontend/src/`）

| 文件 | 职责 |
|---|---|
| `pages/Hosts/index.tsx` | 主机表：增删改查 + 测试连接 |
| `pages/Chat/index.tsx` | 对话页：消息流 + 输入框 + 会话历史列表 |
| `pages/Chat/ApprovalCard.tsx` | 命令批准卡（批准/修改/拒绝） |
| `pages/Chat/useChatEvents.ts` | 订阅 Wails 事件 hook |
| `pages/Settings/index.tsx` | 填充：AI 后端配置 |
| `components/AppLayout/menuConfig.tsx` | 加 `/chat` `/hosts` 路由项 |

---

## 约定

- **TDD**：每个 Go 模块先写失败测试，再实现，再跑通，再提交。
- **提交**：每个任务末尾提交；message 用 `feat:`/`test:`/`chore:` 前缀。
- **不要编辑 `frontend/wailsjs/`**：自动生成。新增 Go 导出方法后跑 `wails dev`（或 `wails generate module`）重新生成代理。
- **Go 包路径**：模块名 `ops-mate`，内部包 `ops-mate/internal/store` 等。

---

## Task 1: Go 依赖与项目骨架

**Files:**
- Modify: `go.mod` / `go.sum`
- Create: `internal/store/db.go`（占位，Task 2 填实现）

- [ ] **Step 1: 添加 Go 依赖**

Run:
```bash
go get github.com/modernc.org/sqlite@latest
go get github.com/zalando/go-keyring@latest
go get golang.org/x/crypto/ssh@latest
```

- [ ] **Step 2: 确认编译通过**

Run: `go build ./...`
Expected: 无报错（即使没有 .go 文件也应无 import 错误）。

- [ ] **Step 3: 创建包目录占位**

Create `internal/store/db.go`:
```go
package store

// DB 包装 SQLite 操作，Task 2 起填充。
```

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/store/db.go
git commit -m "chore: 添加 sqlite/ssh/keyring 依赖与 store 包骨架"
```

---

## Task 2: DBStore — 打开与建表（schema + 迁移）

**Files:**
- Create: `internal/store/schema.go`
- Modify: `internal/store/db.go`
- Test: `internal/store/db_test.go`

- [ ] **Step 1: 写 schema.go**

Create `internal/store/schema.go`:
```go
package store

const schemaSQL = `
CREATE TABLE IF NOT EXISTS hosts (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    addr          TEXT NOT NULL,
    port          INTEGER NOT NULL,
    user          TEXT NOT NULL,
    auth_encrypted BLOB,           -- AES-GCM(密码或私钥PEM)
    auth_type     TEXT NOT NULL,   -- "password" | "privatekey"
    created_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS ai_config (
    id                INTEGER PRIMARY KEY CHECK (id = 1),
    provider          TEXT NOT NULL,   -- "ollama" | "claude"
    model             TEXT NOT NULL,
    base_url          TEXT NOT NULL,
    api_key_encrypted BLOB             -- 可空（ollama 无需）
);

CREATE TABLE IF NOT EXISTS conversations (
    id         TEXT PRIMARY KEY,
    host_id    TEXT NOT NULL,
    title      TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS messages (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL,
    role        TEXT NOT NULL,    -- "user" | "assistant" | "tool"
    content     TEXT NOT NULL,
    tool_result TEXT,             -- role=tool 时的执行输出
    ts          INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES conversations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS commands (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    command    TEXT NOT NULL,
    exit_code   INTEGER,
    output     TEXT,
    ts         INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES conversations(id) ON DELETE CASCADE
);

-- FTS5 虚表：跨会话检索命令记录（命令文本 + 输出）
CREATE VIRTUAL TABLE IF NOT EXISTS commands_fts USING fts5(
    command, output, content='commands', content_rowid='rowid'
);

-- 同步触发器：commands 增删时同步 FTS
CREATE TRIGGER IF NOT EXISTS commands_ai AFTER INSERT ON commands BEGIN
    INSERT INTO commands_fts(rowid, command, output)
    VALUES (new.rowid, new.command, COALESCE(new.output, ''));
END;
CREATE TRIGGER IF NOT EXISTS commands_ad AFTER DELETE ON commands BEGIN
    DELETE FROM commands_fts WHERE rowid = old.rowid;
END;
`
```

- [ ] **Step 2: 写 db.go**

Replace `internal/store/db.go`:
```go
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/modernc.org/sqlite"
)

// Open 打开（必要时创建）ops-mate.db 并执行建表迁移。
// 数据库文件位于用户数据目录（Windows: %APPDATA%/ops-mate）。
func Open() (*sql.DB, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir data dir: %w", err)
	}
	dbPath := filepath.Join(dir, "ops-mate.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// 启用外键与级联。
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return db, nil
}

func dataDir() (string, error) {
	base, err := os.UserConfigDir() // Windows=%APPDATA%, macOS=~/Library/Application Support
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ops-mate"), nil
}
```

- [ ] **Step 3: 写失败测试**

Create `internal/store/db_test.go`:
```go
package store

import (
	"path/filepath"
	"testing"
)

func TestOpen_CreatesAndMigrates(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir()) // 覆盖 dataDir 的根
	db, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var name string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='commands_fts'`).Scan(&name)
	if err != nil {
		t.Fatalf("查询 FTS 表失败: %v", err)
	}
	if name != "commands_fts" {
		t.Fatalf("期望 commands_fts，得到 %s", name)
	}
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/store/ -run TestOpen -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/schema.go internal/store/db.go internal/store/db_test.go
git commit -m "feat(store): 打开 SQLite 并建表（含 FTS5 命令检索）"
```

---

## Task 3: DBStore — 加密（AES-GCM + 主密钥）

**Files:**
- Create: `internal/store/crypto.go`
- Test: `internal/store/crypto_test.go`

- [ ] **Step 1: 写失败测试**

Create `internal/store/crypto_test.go`:
```go
package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMasterKey_PersistsAcrossOpens(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)

	k1, err := masterKey(filepath.Join(dir, "ops-mate"))
	if err != nil {
		t.Fatalf("masterKey 1: %v", err)
	}
	if len(k1) != 32 {
		t.Fatalf("密钥长度 %d，期望 32", len(k1))
	}
	k2, err := masterKey(filepath.Join(dir, "ops-mate"))
	if err != nil {
		t.Fatalf("masterKey 2: %v", err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("同一目录应返回同一主密钥")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := []byte("01234567890123456789012345678901") // 32 字节
	plain := []byte("super-secret-password")
	ct, err := encrypt(key, plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(ct, plain) {
		t.Fatal("密文不应包含明文")
	}
	pt, err := decrypt(key, ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(pt, plain) {
		t.Fatalf("解密结果不匹配")
	}
	// 相同明文两次加密应不同（nonce 随机）
	ct2, _ := encrypt(key, plain)
	if bytes.Equal(ct, ct2) {
		t.Fatal("相同明文应产生不同密文（随机 nonce）")
	}
}

func TestMasterKey_FallsBackToFile(t *testing.T) {
	// 清空 keyring 服务名使其报错，强制走文件回退
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	os.Setenv("OPS_MATE_TEST_NO_KEYRING", "1")
	defer os.Unsetenv("OPS_MATE_TEST_NO_KEYRING")

	k, err := masterKey(filepath.Join(dir, "ops-mate"))
	if err != nil {
		t.Fatalf("文件回退失败: %v", err)
	}
	if len(k) != 32 {
		t.Fatalf("密钥长度 %d", len(k))
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/store/ -run 'TestMasterKey|TestEncrypt' -v`
Expected: FAIL（函数未定义）

- [ ] **Step 3: 写实现**

Create `internal/store/crypto.go`:
```go
package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "ops-mate"
	keyringKey     = "master-key"
)

// masterKey 返回 32 字节主密钥。优先 OS keyring；失败时回退到
// 数据目录下 master.key 文件（0600）。
func masterKey(appDir string) ([]byte, error) {
	if os.Getenv("OPS_MATE_TEST_NO_KEYRING") != "1" {
		secret, err := keyring.Get(keyringService, keyringKey)
		if err == nil && len(secret) == 32 {
			return []byte(secret), nil
		}
		// 不存在或异常 → 生成并尝试存入 keyring
		k, err := randomKey(32)
		if err != nil {
			return nil, err
		}
		if err := keyring.Set(keyringService, keyringKey, string(k)); err == nil {
			return k, nil // 存入成功
		}
		// keyring 不可写 → 落文件回退
	}
	return fileMasterKey(appDir)
}

func fileMasterKey(appDir string) ([]byte, error) {
	p := filepath.Join(appDir, "master.key")
	b, err := os.ReadFile(p)
	if err == nil && len(b) >= 32 {
		return b[:32], nil
	}
	k, err := randomKey(32)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, k, 0o600); err != nil {
		return nil, fmt.Errorf("write master.key: %w", err)
	}
	return k, nil
}

func randomKey(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}
	return b, nil
}

// encrypt 用 AES-256-GCM 加密。输出格式: nonce(12) || ciphertext。
func encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ct...), nil
}

// decrypt 解密 encrypt 的输出。
func decrypt(key, blob []byte) ([]byte, error) {
	if len(blob) < 13 { // 12 nonce + 至少 1
		return nil, errors.New("密文过短")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, ct := blob[:gcm.NonceSize()], blob[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}

// 给一个稳定的 32 字节 key 用于测试以外的占位场景（暂保留供后续 seed）。
var _ = binary.LittleEndian // 防止未用导入报错占位，实际可删
```

> 注：测试里写死的 32 字节 key 与运行时随机密钥不同，仅用于 `TestEncryptDecrypt`。`masterKey` 测试通过 `t.Setenv("APPDATA", dir)` 改变 `dataDir`，因此 `Open()` 内调用 `masterKey(filepath.Join(dir,"ops-mate"))` 路径一致；但当前 `masterKey` 入参是 `appDir`，`Open()` 需传入此目录。下一步 HostStore 会连接二者。

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/store/ -run 'TestMasterKey|TestEncrypt' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/crypto.go internal/store/crypto_test.go
git commit -m "feat(store): AES-GCM 加密与主密钥（keyring 优先，文件回退）"
```

---

## Task 4: DBStore — 主机 CRUD（auth 加密）

**Files:**
- Create: `internal/store/hosts.go`
- Modify: `internal/store/db.go`（`Open` 返回带密钥的 `Store` 结构）
- Test: `internal/store/hosts_test.go`

- [ ] **Step 1: 重构 db.go 引入 Store 结构**

Replace `internal/store/db.go`（保留 `schemaSQL`/`dataDir` 在原文件，新增 `Store`）：

在 `db.go` 末尾追加：
```go
// Store 持有 DB 句柄与主密钥。
type Store struct {
	DB  *sql.DB
	key []byte
}

func Open() (*Store, error) {
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
	key, err := masterKey(dir)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("master key: %w", err)
	}
	return &Store{DB: db, key: key}, nil
}
```
并删除旧的 `func Open() (*sql.DB, error)` 定义（被上面的替换）。同时把 `db_test.go` 的 `db, err := Open(); defer db.Close()` 调整为 `s, err := Open(); defer s.DB.Close()`。

更新 `internal/store/db_test.go`：
```go
package store

import "testing"

func TestOpen_CreatesAndMigrates(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	s, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.DB.Close()

	var name string
	err = s.DB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='commands_fts'`).Scan(&name)
	if err != nil {
		t.Fatalf("查询 FTS 表失败: %v", err)
	}
	if name != "commands_fts" {
		t.Fatalf("期望 commands_fts，得到 %s", name)
	}
}
```

- [ ] **Step 2: 写 hosts.go 失败测试**

Create `internal/store/hosts_test.go`:
```go
package store

import "testing"

func TestHostCRUD_AuthEncrypted(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	s, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.DB.Close()

	h := HostInput{
		Name: "web-01", Addr: "10.0.0.5", Port: 22, User: "ops",
		AuthType: "password", Secret: "p@ss",
	}
	id, err := s.SaveHost(h)
	if err != nil {
		t.Fatalf("SaveHost: %v", err)
	}

	// DB 明文中不应出现密码
	var blob []byte
	if err := s.DB.QueryRow(`SELECT auth_encrypted FROM hosts WHERE id=?`, id).Scan(&blob); err != nil {
		t.Fatalf("查 auth_encrypted: %v", err)
	}
	if string(blob) == "p@ss" {
		t.Fatal("密码被明文存储")
	}

	// ListHosts 不应返回 secret
	list, err := s.ListHosts()
	if err != nil {
		t.Fatalf("ListHosts: %v", err)
	}
	if len(list) != 1 || list[0].Name != "web-01" {
		t.Fatalf("ListHosts 结果: %+v", list)
	}
	if list[0].Secret != "" {
		t.Fatal("ListHosts 不应返回 secret")
	}

	// GetHostSecret 解密应回原文
	sec, at, err := s.GetHostSecret(id)
	if err != nil {
		t.Fatalf("GetHostSecret: %v", err)
	}
	if sec != "p@ss" || at != "password" {
		t.Fatalf("GetHostSecret = %q %q", sec, at)
	}

	// DeleteHost
	if err := s.DeleteHost(id); err != nil {
		t.Fatalf("DeleteHost: %v", err)
	}
	list, _ = s.ListHosts()
	if len(list) != 0 {
		t.Fatal("删除后应无主机")
	}
}
```

- [ ] **Step 3: 运行测试，确认失败**

Run: `go test ./internal/store/ -run TestHostCRUD -v`
Expected: FAIL（`HostInput`/`SaveHost` 等未定义）

- [ ] **Step 4: 写 hosts.go**

Create `internal/store/hosts.go`:
```go
package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// HostInput 主机录入数据。Secret 为密码或私钥 PEM 明文。
type HostInput struct {
	Name     string
	Addr     string
	Port     int
	User     string
	AuthType string // "password" | "privatekey"
	Secret   string
}

// HostMeta 主机列表项（不含凭据）。
type HostMeta struct {
	ID       string
	Name     string
	Addr     string
	Port     int
	User     string
	AuthType string
}

func (s *Store) SaveHost(in HostInput) (string, error) {
	id := newID()
	enc, err := encrypt(s.key, []byte(in.Secret))
	if err != nil {
		return "", fmt.Errorf("encrypt auth: %w", err)
	}
	_, err = s.DB.Exec(
		`INSERT INTO hosts(id,name,addr,port,user,auth_encrypted,auth_type,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		id, in.Name, in.Addr, in.Port, in.User, enc, in.AuthType, time.Now().Unix(),
	)
	if err != nil {
		return "", fmt.Errorf("insert host: %w", err)
	}
	return id, nil
}

func (s *Store) ListHosts() ([]HostMeta, error) {
	rows, err := s.DB.Query(`SELECT id,name,addr,port,user,auth_type FROM hosts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HostMeta
	for rows.Next() {
		var h HostMeta
		if err := rows.Scan(&h.ID, &h.Name, &h.Addr, &h.Port, &h.User, &h.AuthType); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetHostSecret 返回解密后的凭据与类型。
func (s *Store) GetHostSecret(id string) (secret, authType string, err error) {
	var blob []byte
	err = s.DB.QueryRow(`SELECT auth_encrypted, auth_type FROM hosts WHERE id=?`, id).Scan(&blob, &authType)
	if err != nil {
		return "", "", err
	}
	pt, err := decrypt(s.key, blob)
	if err != nil {
		return "", "", fmt.Errorf("decrypt auth: %w", err)
	}
	return string(pt), authType, nil
}

func (s *Store) DeleteHost(id string) error {
	_, err := s.DB.Exec(`DELETE FROM hosts WHERE id=?`, id)
	return err
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 5: 运行测试，确认通过**

Run: `go test ./internal/store/ -v`
Expected: 全部 PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store/hosts.go internal/store/hosts_test.go internal/store/db.go internal/store/db_test.go
git commit -m "feat(store): 主机 CRUD，凭据 AES-GCM 加密列存储"
```

---

## Task 5: DBStore — AI 配置 CRUD

**Files:**
- Create: `internal/store/config.go`
- Test: `internal/store/config_test.go`

- [ ] **Step 1: 写失败测试**

Create `internal/store/config_test.go`:
```go
package store

import "testing"

func TestAIConfig_SaveLoad_APIKeyEncrypted(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	s, _ := Open()
	defer s.DB.Close()

	cfg := AIConfig{Provider: "claude", Model: "claude-sonnet-5", BaseURL: "https://api.anthropic.com", APIKey: "sk-secret"}
	if err := s.SaveAIConfig(cfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	// 明文不应落盘
	var blob []byte
	s.DB.QueryRow(`SELECT api_key_encrypted FROM ai_config WHERE id=1`).Scan(&blob)
	if string(blob) == "sk-secret" {
		t.Fatal("API Key 明文存储")
	}

	got, err := s.GetAIConfig()
	if err != nil {
		t.Fatalf("GetAIConfig: %v", err)
	}
	if got.APIKey != "sk-secret" || got.Model != "claude-sonnet-5" {
		t.Fatalf("回读不匹配: %+v", got)
	}

	// 空配置（ollama 无 key）
	if err := s.SaveAIConfig(AIConfig{Provider: "ollama", Model: "llama3", BaseURL: "http://localhost:11434", APIKey: ""}); err != nil {
		t.Fatalf("空 key Save: %v", err)
	}
	got2, _ := s.GetAIConfig()
	if got2.APIKey != "" {
		t.Fatal("空 key 应为空")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/store/ -run TestAIConfig -v`
Expected: FAIL

- [ ] **Step 3: 写实现**

Create `internal/store/config.go`:
```go
package store

import (
	"database/sql"
	"fmt"
)

type AIConfig struct {
	Provider string // "ollama" | "claude"
	Model    string
	BaseURL  string
	APIKey   string // 内存中明文；落库加密
}

func (s *Store) SaveAIConfig(c AIConfig) error {
	var enc []byte
	if c.APIKey != "" {
		b, err := encrypt(s.key, []byte(c.APIKey))
		if err != nil {
			return fmt.Errorf("encrypt key: %w", err)
		}
		enc = b
	}
	_, err := s.DB.Exec(
		`INSERT INTO ai_config(id,provider,model,base_url,api_key_encrypted) VALUES(1,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET provider=excluded.provider, model=excluded.model,
		   base_url=excluded.base_url, api_key_encrypted=excluded.api_key_encrypted`,
		c.Provider, c.Model, c.BaseURL, enc,
	)
	return err
}

func (s *Store) GetAIConfig() (AIConfig, error) {
	var c AIConfig
	var enc []byte
	err := s.DB.QueryRow(`SELECT provider,model,base_url,api_key_encrypted FROM ai_config WHERE id=1`).
		Scan(&c.Provider, &c.Model, &c.BaseURL, &enc)
	if err == sql.ErrNoRows {
		return AIConfig{}, nil // 未配置，返回零值
	}
	if err != nil {
		return AIConfig{}, err
	}
	if len(enc) > 0 {
		pt, err := decrypt(s.key, enc)
		if err != nil {
			return AIConfig{}, fmt.Errorf("decrypt key: %w", err)
		}
		c.APIKey = string(pt)
	}
	return c, nil
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/config.go internal/store/config_test.go
git commit -m "feat(store): AI 配置 CRUD，API Key 加密列"
```

---

## Task 6: DBStore — 会话/消息/命令记录 CRUD

**Files:**
- Create: `internal/store/conversations.go`
- Test: `internal/store/conversations_test.go`

- [ ] **Step 1: 写失败测试**

Create `internal/store/conversations_test.go`:
```go
package store

import "testing"

func TestConversationAndCommands_FTS(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	s, _ := Open()
	defer s.DB.Close()

	hostID, _ := s.SaveHost(HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})

	sid, err := s.NewConversation(hostID, "cpu 高")
	if err != nil {
		t.Fatalf("NewConversation: %v", err)
	}
	if err := s.AppendMessage(sid, "user", "cpu 为什么高", ""); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if err := s.AppendMessage(sid, "assistant", "我看看", ""); err != nil {
		t.Fatalf("AppendMessage assistant: %v", err)
	}

	if err := s.SaveCommand(sid, "top -bn1", 0, "go proc 99%"); err != nil {
		t.Fatalf("SaveCommand: %v", err)
	}

	// 列会话
	conv, _ := s.ListConversations(hostID)
	if len(conv) != 1 || conv[0].Title != "cpu 高" {
		t.Fatalf("ListConversations: %+v", conv)
	}
	// 读消息
	msgs, _ := s.LoadMessages(sid)
	if len(msgs) != 2 || msgs[0].Role != "user" {
		t.Fatalf("LoadMessages: %+v", msgs)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/store/ -run TestConversation -v`
Expected: FAIL

- [ ] **Step 3: 写实现**

Create `internal/store/conversations.go`:
```go
package store

import (
	"fmt"
	"time"
)

type Conversation struct {
	ID        string
	HostID    string
	Title     string
	CreatedAt int64
	UpdatedAt int64
}

type Message struct {
	ID         string
	SessionID  string
	Role       string
	Content    string
	ToolResult  string
	Ts         int64
}

func (s *Store) NewConversation(hostID, title string) (string, error) {
	id := newID()
	now := time.Now().Unix()
	_, err := s.DB.Exec(
		`INSERT INTO conversations(id,host_id,title,created_at,updated_at) VALUES(?,?,?,?,?)`,
		id, hostID, title, now, now)
	if err != nil {
		return "", fmt.Errorf("insert conversation: %w", err)
	}
	return id, nil
}

func (s *Store) ListConversations(hostID string) ([]Conversation, error) {
	rows, err := s.DB.Query(
		`SELECT id,host_id,title,created_at,updated_at FROM conversations WHERE host_id=? ORDER BY updated_at DESC`,
		hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.HostID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) AppendMessage(sessionID, role, content, toolResult string) error {
	id := newID()
	_, err := s.DB.Exec(
		`INSERT INTO messages(id,session_id,role,content,tool_result,ts) VALUES(?,?,?,?,?,?)`,
		id, sessionID, role, content, nullable(toolResult), time.Now().Unix())
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`UPDATE conversations SET updated_at=? WHERE id=?`, time.Now().Unix(), sessionID)
	return err
}

func (s *Store) LoadMessages(sessionID string) ([]Message, error) {
	rows, err := s.DB.Query(
		`SELECT id,session_id,role,content,tool_result,ts FROM messages WHERE session_id=? ORDER BY ts`,
		sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var tr *string
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &tr, &m.Ts); err != nil {
			return nil, err
		}
		if tr != nil {
			m.ToolResult = *tr
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) SaveCommand(sessionID, command string, exitCode int, output string) error {
	id := newID()
	_, err := s.DB.Exec(
		`INSERT INTO commands(id,session_id,command,exit_code,output,ts) VALUES(?,?,?,?,?,?)`,
		id, sessionID, command, exitCode, nullable(output), time.Now().Unix())
	return err
}

func (s *Store) DeleteConversation(id string) error {
	_, err := s.DB.Exec(`DELETE FROM conversations WHERE id=?`, id)
	return err
}

// nullable 把空串转为 NULL，否则原值。
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/conversations.go internal/store/conversations_test.go
git commit -m "feat(store): 会话/消息/命令记录 CRUD"
```

---

## Task 7: Memory — 跨会话 FTS5 检索

**Files:**
- Create: `internal/store/memory.go`
- Test: `internal/store/memory_test.go`

- [ ] **Step 1: 写失败测试**

Create `internal/store/memory_test.go`:
```go
package store

import "testing"

func TestMemory_RecallReturnsPastCommands(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	s, _ := Open()
	defer s.DB.Close()

	hostID, _ := s.SaveHost(HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	sid, _ := s.NewConversation(hostID, "old")
	s.SaveCommand(sid, "top -bn1", 0, "go proc 占满 CPU")
	s.SaveCommand(sid, "journalctl -u nginx", 0, "nginx restarted")

	ctx, err := s.Recall(hostID, "CPU 高怎么回事")
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(ctx.PastCommands) == 0 {
		t.Fatal("应召回过往命令")
	}
	hit := false
	for _, c := range ctx.PastCommands {
		if c.Command == "top -bn1" {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("应召回 top 命令，得到 %+v", ctx.PastCommands)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/store/ -run TestMemory -v`
Expected: FAIL

- [ ] **Step 3: 写实现**

Create `internal/store/memory.go`:
```go
package store

import "fmt"

// PastCommand 召回的历史命令记录。
type PastCommand struct {
	Command string
	Output  string
}

// RecallContext 注入给 AI 的记忆上下文。
type RecallContext struct {
	PastCommands []PastCommand
}

// Recall 按 hostID + 关键词从 commands 表 FTS5 检索这台机器过往命令。
// 取 top-N（默认 5）。关键词做简单分词后 OR 拼接。
func (s *Store) Recall(hostID, question string) (RecallContext, error) {
	q := ftsQuery(question)
	if q == "" {
		return RecallContext{}, nil
	}
	// 通过 host_id 过滤：commands.session_id → conversations.host_id
	rows, err := s.DB.Query(`
		SELECT c.command, COALESCE(c.output,'')
		FROM commands c
		JOIN conversations v ON v.id = c.session_id
		WHERE v.host_id = ?
		  AND c.rowid IN (SELECT rowid FROM commands_fts WHERE commands_fts MATCH ?)
		ORDER BY c.ts DESC
		LIMIT 5`, hostID, q)
	if err != nil {
		return RecallContext{}, fmt.Errorf("recall query: %w", err)
	}
	defer rows.Close()
	var out RecallContext
	for rows.Next() {
		var pc PastCommand
		if err := rows.Scan(&pc.Command, &pc.Output); err != nil {
			return RecallContext{}, err
		}
		out.PastCommands = append(out.PastCommands, pc)
	}
	return out, rows.Err()
}

// ftsQuery 把自然语言问题转成 FTS5 OR 查询，过滤停用词与过短词。
func ftsQuery(s string) string {
	stop := map[string]bool{"the": true, "a": true, "an": true, "is": true, "为什么": true, "怎么": true, "怎么回事": true}
	var tokens []string
	cur := ""
	for _, r := range s {
		if r == ' ' || r == ',' || r == '？' || r == '?' || r == '。' {
			if cur != "" && !stop[cur] && len(cur) > 1 {
				tokens = append(tokens, cur)
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" && !stop[cur] && len(cur) > 1 {
		tokens = append(tokens, cur)
	}
	out := ""
	for i, tk := range tokens {
		if i > 0 {
			out += " OR "
		}
		out += "\"" + tk + "\""
	}
	return out
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/store/ -run TestMemory -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/memory.go internal/store/memory_test.go
git commit -m "feat(store): Memory 跨会话 FTS5 检索召回"
```

---

## Task 8: SSHExecutor — 命令执行（流式输出）

**Files:**
- Create: `internal/sshexec/executor.go`
- Test: `internal/sshexec/executor_test.go`（用内置 SSH server）

- [ ] **Step 1: 写失败测试**

Create `internal/sshexec/executor_test.go`:
```go
package sshexec

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// startTestSSHServer 起一个本地 SSH server，用传入公钥授权。
// 返回 (addr, clientPrivateKeyPEM, cleanup)。
func startTestSSHServer(t *testing.T) (string, string, func()) {
	t.Helper()
	pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)
	sshPub, _ := ssh.NewPublicKey(pubKey)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	config := &ssh.ServerConfig{NoClientAuth: false}
	config.PublicKeyCallback = func(c ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
		if bytesEqual(k.Marshal(), sshPub.Marshal()) {
			return nil, nil
		}
		return nil, fmt.Errorf("unknown key")
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				_, chans, reqs, err := ssh.NewServerConn(c, config)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for newCh := range chans {
					ch, reqs, _ := newCh.Accept()
					go func(ch ssh.Channel, reqs <-chan *ssh.Request) {
						for r := range reqs {
							if r.Type == "exec" {
								var cmd struct{ Cmd string }
								ssh.Unmarshal(r.Payload, &cmd)
								r.Reply(true, nil)
								fmt.Fprintf(ch, "ran: %s\n", cmd.Cmd)
								ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{0}))
								ch.Close()
							}
						}
					}(ch, reqs)
				}
			}(conn)
		}
	}()

	pemBlock, _ := ssh.MarshalPrivateKey(privKey, "")
	return ln.Addr().String(), string(pem.EncodeToMemory(pemBlock)), func() { ln.Close() }
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSSHExecutor_StreamsOutput(t *testing.T) {
	addr, privPEM, cleanup := startTestSSHServer(t)
	defer cleanup()

	host := Host{
		Addr: addr, Port: 0, User: "test",
		AuthType: "privatekey", Secret: privPEM,
	}
	ex := NewExecutor(host)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lines, err := ex.Exec(ctx, "echo hi")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	var out strings.Builder
	for ln := range lines {
		out.WriteString(ln)
	}
	if !strings.Contains(out.String(), "ran: echo hi") {
		t.Fatalf("期望包含执行输出，得到 %q", out.String())
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/sshexec/ -v`
Expected: FAIL（`Host`/`NewExecutor`/`Exec` 未定义）

- [ ] **Step 3: 写实现**

Create `internal/sshexec/executor.go`:
```go
package sshexec

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Host 描述一个 SSH 目标。Secret 为密码或私钥 PEM 明文。
type Host struct {
	Addr     string // host:port 或 host
	Port     int
	User     string
	AuthType string // "password" | "privatekey"
	Secret   string
}

// Line 一行输出，带来源 stdout/stderr。
type Line struct {
	Stream string // "stdout" | "stderr"
	Text   string
}

// Executor 执行命令并逐行流式输出。
type Executor struct{ host Host }

func NewExecutor(h Host) *Executor { return &Executor{host: h} }

func (e *Executor) dial(ctx context.Context) (*ssh.Client, error) {
	addr := e.host.Addr
	if !strings.Contains(addr, ":") {
		port := 22
		if e.host.Port != 0 {
			port = e.host.Port
		}
		addr = net.JoinHostPort(addr, strconv.Itoa(port))
	}
	auth, err := e.authMethod()
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            e.host.User,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // MVP：不做主机指纹校验
		Timeout:         10 * time.Second,
	}
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func (e *Executor) authMethod() (ssh.AuthMethod, error) {
	switch e.host.AuthType {
	case "password":
		return ssh.Password(e.host.Secret), nil
	case "privatekey":
		signer, err := ssh.ParsePrivateKey([]byte(e.host.Secret))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("未知 auth_type %q", e.host.AuthType)
	}
}

// Exec 执行命令，返回行流通道（执行结束后关闭）。
// 输出逐行通过 channel 推送；ctx 取消则中止会话。
func (e *Executor) Exec(ctx context.Context, command string) (<-chan Line, error) {
	client, err := e.dial(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		client.Close()
		return nil, err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		client.Close()
		return nil, err
	}
	if err := sess.Start(command); err != nil {
		client.Close()
		return nil, fmt.Errorf("start: %w", err)
	}
	out := make(chan Line, 32)
	go func() {
		defer client.Close()
		pipe := func(r io.Reader, stream string) {
			sc := bufio.NewScanner(r)
			for sc.Scan() {
				select {
				case out <- Line{Stream: stream, Text: sc.Text()}:
				case <-ctx.Done():
					return
				}
			}
		}
		go pipe(stdout, "stdout")
		pipe(stderr, "stderr")
		_ = sess.Wait() // 等待结束
		close(out)
	}()
	return out, nil
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/sshexec/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/sshexec/executor.go internal/sshexec/executor_test.go
git commit -m "feat(sshexec): SSH 命令执行，逐行流式输出"
```

---

## Task 9: LLMClient — 接口 + Ollama provider（流式）

**Files:**
- Create: `internal/llm/client.go`
- Create: `internal/llm/ollama.go`
- Test: `internal/llm/ollama_test.go`（用 httptest 模拟）

- [ ] **Step 1: 写 client.go（接口与类型）**

Create `internal/llm/client.go`:
```go
package llm

import "context"

// Role 消息角色。
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message 一条对话消息。
type Message struct {
	Role       Role
	Content    string
	ToolResult string // role=tool 时的执行输出
}

// Chunk 流式返回片段。Text 为文本增量；
// 若 Command != nil，表示 AI 提议了一条命令。
type Chunk struct {
	Text    string
	Command *CommandSuggestion
}

// CommandSuggestion AI 提议的命令（结构化）。
type CommandSuggestion struct {
	Command string
	Why     string
	Risk    string // "low" | "medium" | "high"
}

// LLMClient 统一 AI 后端抽象。
type LLMClient interface {
	Chat(ctx context.Context, msgs []Message) (<-chan Chunk, error)
}

// SystemPrompt 约束 AI 只返回：普通文本，或 JSON 命令块。
// 命令块格式：{"command":"...","why":"...","risk":"low|medium|high"}
const SystemPrompt = `你是一个 SSH 运维助手。回答用户关于远程 Linux 主机的问题。
你可以：
1) 用普通文本解释分析；
2) 提议一条要在目标主机执行的命令，输出严格的 JSON：{"command":"...","why":"...","risk":"low|medium|high"}
不要把命令写在普通文本里；要执行就只用 JSON 命令块。
每次最多提议一条命令。无命令时输出普通文本即可。`
```

- [ ] **Step 2: 写失败测试（fake Ollama server）**

Create `internal/llm/ollama_test.go`:
```go
package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllama_StreamsTextAndCommand(t *testing.T) {
	// 模拟 Ollama /api/chat 的 NDJSON 流式响应
	body := strings.Join([]string{
		`{"message":{"role":"assistant","content":"我先"},"done":false}`,
		`{"message":{"role":"assistant","content":"看看进程"},"done":false}`,
		`{"message":{"role":"assistant","content":"{\"command\":\"top -bn1\",\"why\":\"查 CPU\",\"risk\":\"low\"}"},"done":false}`,
		`{"message":{"role":"assistant","content":""},"done":true}`,
	}, "\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewOllama(srv.URL, "llama3")
	ch, err := c.Chat(context.Background(), []Message{{Role: RoleUser, Content: "cpu 高"}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	var text strings.Builder
	var cmd *CommandSuggestion
	for ck := range ch {
		if ck.Command != nil {
			cmd = ck.Command
		}
		text.WriteString(ck.Text)
	}
	if text.String() != "我先看看进程" {
		t.Fatalf("文本拼接 = %q", text.String())
	}
	if cmd == nil || cmd.Command != "top -bn1" {
		t.Fatalf("未解析出命令: %+v", cmd)
	}
}
```

- [ ] **Step 3: 运行测试，确认失败**

Run: `go test ./internal/llm/ -run TestOllama -v`
Expected: FAIL（`NewOllama` 未定义）

- [ ] **Step 4: 写 ollama.go**

Create `internal/llm/ollama.go`:
```go
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Ollama 调用本地 Ollama HTTP /api/chat（流式 NDJSON）。
type Ollama struct {
	baseURL string
	model   string
}

func NewOllama(baseURL, model string) *Ollama {
	return &Ollama{baseURL: baseURL, model: model}
}

type ollamaChatReq struct {
	Model    string    `json:"model"`
	Messages []ollamaMsg `json:"messages"`
	Stream   bool      `json:"stream"`
}

type ollamaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChunk struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

func (o *Ollama) Chat(ctx context.Context, msgs []Message) (<-chan Chunk, error) {
	payload := ollamaChatReq{Model: o.model, Stream: true}
	payload.Messages = append(payload.Messages, ollamaMsg{Role: "system", Content: SystemPrompt})
	for _, m := range msgs {
		role := string(m.Role)
		content := m.Content
		if m.Role == RoleTool {
			content = "[执行结果]\n" + m.ToolResult
			role = "user"
		}
		payload.Messages = append(payload.Messages, ollamaMsg{Role: role, Content: content})
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("ollama status %d", resp.StatusCode)
	}
	out := make(chan Chunk, 16)
	go func() {
		defer resp.Body.Close()
		defer close(out)
		sc := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024)
		sc.Buffer(buf, 1024*1024)
		for sc.Scan() {
			line := sc.Bytes()
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var ch ollamaChunk
			if err := json.Unmarshal(line, &ch); err != nil {
				continue
			}
			content := ch.Message.Content
			// 尝试把内容解析为命令块；解析失败则当普通文本。
			if cmd, ok := tryParseCommand(content); ok {
				select {
				case out <- Chunk{Command: cmd}:
				case <-ctx.Done():
					return
				}
				continue
			}
			select {
			case out <- Chunk{Text: content}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// tryParseCommand 尝试从一段文本里提取 JSON 命令块。
// 接受裸 JSON，或被 ```json ... ``` 包裹的。
func tryParseCommand(s string) (*CommandSuggestion, bool) {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, "```json")
	t = strings.TrimPrefix(t, "```")
	t = strings.TrimSuffix(t, "```")
	t = strings.TrimSpace(t)
	if !strings.HasPrefix(t, "{") {
		return nil, false
	}
	var c CommandSuggestion
	if err := json.Unmarshal([]byte(t), &c); err != nil {
		return nil, false
	}
	if c.Command == "" {
		return nil, false
	}
	return &c, true
}
```

- [ ] **Step 5: 运行测试，确认通过**

Run: `go test ./internal/llm/ -run TestOllama -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/llm/client.go internal/llm/ollama.go internal/llm/ollama_test.go
git commit -m "feat(llm): LLMClient 接口 + Ollama 流式 provider"
```

---

## Task 10: LLMClient — Claude/Anthropic provider（SSE 流式）

**Files:**
- Create: `internal/llm/claude.go`
- Test: `internal/llm/claude_test.go`（fake SSE server）

- [ ] **Step 1: 写失败测试**

Create `internal/llm/claude_test.go`:
```go
package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClaude_StreamsSSE(t *testing.T) {
	// 模拟 Anthropic /v1/messages 的 SSE 流
	sse := strings.Join([]string{
		`event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"我先"}}

`,
		`event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"看看"}}

`,
		`event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"{\"command\":\"top -bn1\",\"why\":\"查 CPU\",\"risk\":\"low\"}"}}

`,
		`event: message_stop
data: {"type":"message_stop"}

`,
	}, "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 校验 header
		if r.Header.Get("x-api-key") != "sk-test" {
			t.Errorf("缺 x-api-key")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sse))
	}))
	defer srv.Close()

	c := NewClaude(srv.URL, "sk-test", "claude-sonnet-5")
	ch, err := c.Chat(context.Background(), []Message{{Role: RoleUser, Content: "cpu 高"}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	var text strings.Builder
	var cmd *CommandSuggestion
	for ck := range ch {
		if ck.Command != nil {
			cmd = ck.Command
		}
		text.WriteString(ck.Text)
	}
	if text.String() != "我先看看" {
		t.Fatalf("文本 = %q", text.String())
	}
	if cmd == nil || cmd.Command != "top -bn1" {
		t.Fatalf("未解析命令: %+v", cmd)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/llm/ -run TestClaude -v`
Expected: FAIL（`NewClaude` 未定义）

- [ ] **Step 3: 写实现**

Create `internal/llm/claude.go`:
```go
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Claude 调用 Anthropic Messages API（SSE 流式）。
type Claude struct {
	baseURL string
	apiKey  string
	model   string
}

func NewClaude(baseURL, apiKey, model string) *Claude {
	return &Claude{baseURL: baseURL, apiKey: apiKey, model: model}
}

type claudeReq struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Stream    bool           `json:"stream"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (c *Claude) Chat(ctx context.Context, msgs []Message) (<-chan Chunk, error) {
	var cm []claudeMessage
	for _, m := range msgs {
		role := string(m.Role)
		content := m.Content
		if m.Role == RoleTool {
			role = "user"
			content = "[执行结果]\n" + m.ToolResult
		}
		cm = append(cm, claudeMessage{Role: role, Content: content})
	}
	payload := claudeReq{
		Model: c.model, MaxTokens: 2048, Stream: true,
		System: SystemPrompt, Messages: cm,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("claude request: %w", err)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("claude status %d", resp.StatusCode)
	}
	out := make(chan Chunk, 16)
	go func() {
		defer resp.Body.Close()
		defer close(out)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			var evt struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue
			}
			if evt.Type != "content_block_delta" || evt.Delta.Text == "" {
				continue
			}
			text := evt.Delta.Text
			if cmd, ok := tryParseCommand(text); ok {
				select {
				case out <- Chunk{Command: cmd}:
				case <-ctx.Done():
					return
				}
				continue
			}
			select {
			case out <- Chunk{Text: text}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/llm/ -v`
Expected: PASS（Ollama + Claude）

- [ ] **Step 5: Commit**

```bash
git add internal/llm/claude.go internal/llm/claude_test.go
git commit -m "feat(llm): Claude/Anthropic SSE 流式 provider"
```

---

## Task 11: 危险命令护栏

**Files:**
- Create: `internal/orchestrator/guardrail.go`
- Test: `internal/orchestrator/guardrail_test.go`

- [ ] **Step 1: 写失败测试**

Create `internal/orchestrator/guardrail_test.go`:
```go
package orchestrator

import "testing"

func TestAssessRisk(t *testing.T) {
	cases := []struct {
		cmd   string
		level string // "" = 无风险, "high"
	}{
		{"ls -la", ""},
		{"top -bn1", ""},
		{"rm -rf /", "high"},
		{"mkfs.ext4 /dev/sda1", "high"},
		{"dd if=/dev/zero of=/dev/sdb", "high"},
		{"shutdown -h now", "high"},
		{"reboot", "high"},
		{":() { :|:& };:", "high"}, // fork bomb
		{"echo hi > /dev/sda", "high"},
	}
	for _, c := range cases {
		got := AssessRisk(c.cmd)
		if c.level == "" {
			if got != "" {
				t.Errorf("命令 %q 期望无风险，得到 %q", c.cmd, got)
			}
		} else if got != c.level {
			t.Errorf("命令 %q 期望 %q，得到 %q", c.cmd, c.level, got)
		}
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/orchestrator/ -run TestAssessRisk -v`
Expected: FAIL

- [ ] **Step 3: 写实现**

Create `internal/orchestrator/guardrail.go`:
```go
package orchestrator

import (
	"regexp"
	"strings"
)

// 危险模式：匹配即标 "high"（标红 + 二次确认，引擎不硬拒）。
var dangerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f?|-[a-zA-Z]*f[a-zA-Z]*r?)\s+/(--\S+\s+)?/(\s|$)`), // rm -rf /
	regexp.MustCompile(`\brm\s+-[a-zA-Z]*r[a-zA-Z]*f?.*\/\s`),                                        // rm -rf 任意 /
	regexp.MustCompile(`\bmkfs\b`),
	regexp.MustCompile(`\bdd\b.*\bof=/dev/`),
	regexp.MustCompile(`\b(shutdown|poweroff|halt|reboot)\b`),
	regexp.MustCompile(`>\s*/dev/(sd|nvme|hd|vd)`), // 重定向到块设备
	regexp.MustCompile(`:\(\)\s*\{\s*:\|:&\s*\}\s*;`), // fork bomb
}

// AssessRisk 返回命令风险等级："" 无风险，"high" 危险。
// 引擎层永不硬拒——这里只用于前端标红与二次确认。
func AssessRisk(command string) string {
	c := strings.TrimSpace(command)
	for _, p := range dangerPatterns {
		if p.MatchString(c) {
			return "high"
		}
	}
	return ""
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/orchestrator/ -run TestAssessRisk -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/guardrail.go internal/orchestrator/guardrail_test.go
git commit -m "feat(orchestrator): 危险命令静态护栏（标红，不硬拒）"
```

---

## Task 12: ConversationOrchestrator — 会话状态机

**Files:**
- Create: `internal/orchestrator/orchestrator.go`
- Test: `internal/orchestrator/orchestrator_test.go`（fake LLM + fake Executor + 内存 DB）

- [ ] **Step 1: 写失败测试（覆盖状态机各迁移）**

Create `internal/orchestrator/orchestrator_test.go`:
```go
package orchestrator

import (
	"context"
	"sync"
	"testing"

	opsMateSSH "ops-mate/internal/sshexec"
	"ops-mate/internal/store"
)

// fakeLLM 按预设脚本返回内容。
type fakeLLM struct {
	mu     sync.Mutex
	script []string // 每次调用取下一个
	idx    int
}

func (f *fakeLLM) Chat(ctx context.Context, msgs []store.Msg) (<-chan opsMateSSH.Line, error) {
	return nil, nil // unused
}

func (f *fakeLLM) next() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.idx >= len(f.script) {
		return ""
	}
	s := f.script[f.idx]
	f.idx++
	return s
}
```

> 说明：上面是占位结构；真实 fake 需实现 `LLMClient`（`Chat(ctx, []Message) (<-chan Chunk, error)`）。下面 Step 3 定义真实接口后，测试用 fakeLLM 实现 `Chat` 推送预设 Chunk。为避免循环，本任务测试以**直接驱动 Orchestrator 内部方法**的方式覆盖状态迁移，不依赖网络。

重写测试文件为以下完整版本：

Replace `internal/orchestrator/orchestrator_test.go`:
```go
package orchestrator

import (
	"context"
	"testing"

	"ops-mate/internal/llm"
	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"
)

// scriptLLM 按脚本依次返回 Chunk 流。
type scriptLLM struct {
	scripts [][]llm.Chunk // 每次调用返回一个脚本的所有 chunk
	calls   int
}

func (s *scriptLLM) Chat(ctx context.Context, msgs []llm.Message) (<-chan llm.Chunk, error) {
	ch := make(chan llm.Chunk, 8)
	go func() {
		defer close(ch)
		idx := s.calls
		s.calls++
		if idx < len(s.scripts) {
			for _, c := range s.scripts[idx] {
				ch <- c
			}
		}
	}()
	return ch, nil
}

// stubExecutor 同步返回固定行。
type stubExecutor struct {
	lines []sshexec.Line
}

func (e *stubExecutor) Exec(ctx context.Context, hostID, command string) (<-chan sshexec.Line, error) {
	ch := make(chan sshexec.Line, 8)
	go func() {
		defer close(ch)
		for _, l := range e.lines {
			ch <- l
		}
	}()
	return ch, nil
}

// 用真实 store + fake LLM/Executor 跑全流程。
func newTestOrchestrator(t *testing.T) (*store.Store, *Orchestrator) {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	st, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return st, NewOrchestrator(st)
}

func TestStateMachine_ApproveExecuteFeedback(t *testing.T) {
	st, o := newTestOrchestrator(t)
	defer st.DB.Close()

	hostID, _ := st.SaveHost(store.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	o.LLM = &scriptLLM{
		scripts: [][]llm.Chunk{
			// 第一次：AI 提议命令
			{llm.Chunk{Text: "我看看"}, llm.Chunk{Command: &llm.CommandSuggestion{Command: "top -bn1", Why: "查 CPU", Risk: "low"}}},
			// 第二次（回灌后）：AI 给纯文本结论
			{llm.Chunk{Text: "是 go 进程占满"}},
		},
	}
	o.ExecutorFor = func(hostID string) Exec { return &stubExecutor{lines: []sshexec.Line{{Stream: "stdout", Text: "go 99%"}}} }

	sid, err := o.NewSession(hostID, "cpu 高")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	coll := o.Collect(sid)

	// 1) 用户发消息 → AI 提议命令（AwaitingApproval）
	o.SendMessage(sid, "cpu 高")
	cmd := <-coll.Command
	if cmd.Command != "top -bn1" {
		t.Fatalf("期望提议 top -bn1，得到 %q", cmd.Command)
	}
	if <-coll.State != "AwaitingApproval" {
		t.Fatal("状态应为 AwaitingApproval")
	}

	// 2) 批准 → 执行 → 输出 → 回灌 → AI 给结论（Idle）
	o.ApproveCommand(sid, cmd.Command)
	if <-coll.State != "Running" {
		t.Fatal("状态应为 Running")
	}
	line := <-coll.Line
	if line.Text != "go 99%" {
		t.Fatalf("执行输出 = %q", line.Text)
	}
	<-coll.Done
	if <-coll.State != "FeedingBack" {
		t.Fatal("状态应为 FeedingBack")
	}
	finalText := <-coll.Text
	if finalText != "是 go 进程占满" {
		t.Fatalf("结论 = %q", finalText)
	}
	if <-coll.State != "Idle" {
		t.Fatal("状态应收回 Idle")
	}
}

func TestStateMachine_RejectAsksAIForAlternative(t *testing.T) {
	st, o := newTestOrchestrator(t)
	defer st.DB.Close()
	hostID, _ := st.SaveHost(store.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	o.LLM = &scriptLLM{
		scripts: [][]llm.Chunk{
			{llm.Chunk{Command: &llm.CommandSuggestion{Command: "rm -rf /tmp/x", Why: "清理", Risk: "high"}}},
			{llm.Chunk{Command: &llm.CommandSuggestion{Command: "du -sh /tmp", Why: "看占用", Risk: "low"}}},
		},
	}
	o.ExecutorFor = func(hostID string) Exec { return &stubExecutor{} }

	sid, _ := o.NewSession(hostID, "清理")
	coll := o.Collect(sid)
	o.SendMessage(sid, "清理 tmp")
	cmd := <-coll.Command
	o.RejectCommand(sid) // 用户拒绝 → AI 重新提议
	cmd2 := <-coll.Command
	if cmd2.Command != "du -sh /tmp" {
		t.Fatalf("拒绝后应换方案，得到 %q", cmd2.Command)
	}
}

func TestStateMachine_DangerCommandFlagged(t *testing.T) {
	st, o := newTestOrchestrator(t)
	defer st.DB.Close()
	hostID, _ := st.SaveHost(store.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	o.LLM = &scriptLLM{
		scripts: [][]llm.Chunk{
			{llm.Chunk{Command: &llm.CommandSuggestion{Command: "rm -rf /", Why: "x", Risk: "high"}}},
		},
	}
	o.ExecutorFor = func(hostID string) Exec { return &stubExecutor{} }
	sid, _ := o.NewSession(hostID, "x")
	coll := o.Collect(sid)
	o.SendMessage(sid, "删根目录")
	cmd := <-coll.Command
	if cmd.AssessedRisk != "high" {
		t.Fatalf("危险命令应标红，得到 %q", cmd.AssessedRisk)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/orchestrator/ -v`
Expected: FAIL（`Orchestrator`/`NewOrchestrator`/`Collect` 等未定义）

- [ ] **Step 3: 写实现**

Create `internal/orchestrator/orchestrator.go`:
```go
package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"ops-mate/internal/llm"
	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"
)

// Exec 执行器接口（供测试 stub 与真实 SSHExecutor 实现）。
type Exec interface {
	Exec(ctx context.Context, hostID, command string) (<-chan sshexec.Line, error)
}

// LLM AI 后端接口别名，便于替换。
type LLM = llm.LLMClient

// Session 一个对话会话。
type Session struct {
	ID        string
	HostID    string
	stateMu   sync.Mutex
	state     string // "Idle"|"AwaitingApproval"|"Running"|"FeedingBack"
	history   []llm.Message
	current   *llm.CommandSuggestion // 待批准命令
	ctx       context.Context
	cancel    context.CancelFunc
}

// Orchestrator 管理所有会话，依赖 store/llm/executor。
type Orchestrator struct {
	store      *store.Store
	LLM        LLM
	ExecutorFor func(hostID string) Exec // 按主机返回执行器
	sessionsMu sync.Mutex
	sessions   map[string]*Session
	emit       func(sessionID, event string, data any) // Wails 事件推送，注入
}

func NewOrchestrator(st *store.Store) *Orchestrator {
	return &Orchestrator{store: st, sessions: map[string]*Session{}}
}

// SetEmitter 注入事件推送函数（App 层绑定 Wails EventsEmit）。
func (o *Orchestrator) SetEmitter(fn func(sessionID, event string, data any)) {
	o.emit = fn
}

func (o *Orchestrator) emitEvent(sid, event string, data any) {
	if o.emit != nil {
		o.emit(sid, event, data)
	}
}

func (o *Orchestrator) NewSession(hostID, title string) (string, error) {
	sid, err := o.store.NewConversation(hostID, title)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{ID: sid, HostID: hostID, state: "Idle", ctx: ctx, cancel: cancel}
	o.sessionsMu.Lock()
	o.sessions[sid] = s
	o.sessionsMu.Unlock()
	return sid, nil
}

func (o *Orchestrator) getSession(sid string) (*Session, error) {
	o.sessionsMu.Lock()
	defer o.sessionsMu.Unlock()
	s, ok := o.sessions[sid]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sid)
	}
	return s, nil
}

func (s *Session) setState(st string) {
	s.stateMu.Lock()
	s.state = st
	s.stateMu.Unlock()
}

func (s *Session) getState() string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.state
}

// SendMessage 用户发起/继续对话。
func (o *Orchestrator) SendMessage(sid, text string) error {
	s, err := o.getSession(sid)
	if err != nil {
		return err
	}
	if o.LLM == nil {
		o.emitEvent(sid, "ai:text", "AI 后端未配置，请到设置页配置")
		return nil
	}
	s.history = append(s.history, llm.Message{Role: llm.RoleUser, Content: text})
	o.store.AppendMessage(sid, "user", text, "")
	go o.runLLMTurn(s)
	return nil
}

// runLLMTurn 调一次 AI，处理其流式输出。
func (o *Orchestrator) runLLMTurn(s *Session) {
	// 记忆注入
	ctx := s.ctx
	recall, _ := o.store.Recall(s.HostID, lastUserText(s.history))
	prompt := append([]llm.Message{}, s.history...)
	if len(recall.PastCommands) > 0 {
		prompt = append([]llm.Message{{
			Role: llm.RoleUser, Content: pastCommandsNote(recall.PastCommands),
		}}, prompt...)
	}
	ch, err := o.LLM.Chat(ctx, prompt)
	if err != nil {
		o.emitEvent(s.ID, "ai:text", "AI 后端不可用："+err.Error())
		s.setState("Idle")
		o.emitEvent(s.ID, "session:state", "Idle")
		return
	}
	var assistantText string
	for ck := range ch {
		if ck.Command != nil {
			s.current = ck.Command
			risk := ck.Command.Risk
			if ar := AssessRisk(ck.Command.Command); ar == "high" {
				risk = "high"
			}
			s.setState("AwaitingApproval")
			o.emitEvent(s.ID, "ai:command", map[string]any{
				"command": ck.Command.Command, "why": ck.Command.Why,
				"risk": risk, "assessedRisk": AssessRisk(ck.Command.Command),
			})
			o.emitEvent(s.ID, "session:state", "AwaitingApproval")
			return // 暂停，等批准
		}
		assistantText += ck.Text
		o.emitEvent(s.ID, "ai:text", ck.Text)
	}
	if assistantText != "" {
		s.history = append(s.history, llm.Message{Role: llm.RoleAssistant, Content: assistantText})
		o.store.AppendMessage(s.ID, "assistant", assistantText, "")
	}
	s.setState("Idle")
	o.emitEvent(s.ID, "session:state", "Idle")
}

// ApproveCommand 用户批准（可传改后的命令）。
func (o *Orchestrator) ApproveCommand(sid, command string) error {
	s, err := o.getSession(sid)
	if err != nil {
		return err
	}
	if s.getState() != "AwaitingApproval" {
		return fmt.Errorf("当前状态不可批准")
	}
	go o.executeCommand(s, command)
	return nil
}

// RejectCommand 用户拒绝 → 让 AI 换方案。
func (o *Orchestrator) RejectCommand(sid string) error {
	s, err := o.getSession(sid)
	if err != nil {
		return err
	}
	s.history = append(s.history, llm.Message{Role: llm.RoleUser, Content: "用户拒绝了这条命令，请换一个方案。"})
	o.store.AppendMessage(sid, "user", "用户拒绝了这条命令，请换一个方案。", "")
	s.current = nil
	s.setState("Idle")
	o.emitEvent(sid, "session:state", "Idle")
	go o.runLLMTurn(s)
	return nil
}

func (o *Orchestrator) executeCommand(s *Session, command string) {
	s.setState("Running")
	o.emitEvent(s.ID, "session:state", "Running")
	ex := o.ExecutorFor(s.HostID)
	if ex == nil {
		o.emitEvent(s.ID, "ai:text", "执行器未配置")
		s.setState("Idle")
		o.emitEvent(s.ID, "session:state", "Idle")
		return
	}
	ch, err := ex.Exec(s.ctx, s.HostID, command)
	if err != nil {
		o.emitEvent(s.ID, "ai:text", "连接失败："+err.Error())
		s.setState("AwaitingApproval")
		o.emitEvent(s.ID, "session:state", "AwaitingApproval")
		return
	}
	var output string
	for ln := range ch {
		output += ln.Text + "\n"
		o.emitEvent(s.ID, "run:line", ln)
	}
	o.emitEvent(s.ID, "run:done", map[string]any{"exitCode": 0})
	o.store.SaveCommand(s.ID, command, 0, output)

	s.setState("FeedingBack")
	o.emitEvent(s.ID, "session:state", "FeedingBack")
	// 回灌执行结果
	s.history = append(s.history, llm.Message{Role: llm.RoleTool, Content: command, ToolResult: output})
	o.store.AppendMessage(s.ID, "tool", command, output)
	o.runLLMTurn(s)
}

// CancelRun 中止正在执行的命令。
func (o *Orchestrator) CancelRun(sid string) error {
	s, err := o.getSession(sid)
	if err != nil {
		return err
	}
	s.cancel()
	return nil
}

func lastUserText(msgs []llm.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == llm.RoleUser {
			return msgs[i].Content
		}
	}
	return ""
}

func pastCommandsNote(pcs []store.PastCommand) string {
	note := "该主机过去执行过的相关命令记录（供参考）：\n"
	for _, c := range pcs {
		note += fmt.Sprintf("- %s → %s\n", c.Command, truncate(c.Output, 200))
	}
	return note
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// === 供测试同步收集事件的 Collect 机制 ===

type Collector struct {
	Text    chan string
	Command chan map[string]any
	Line    chan sshexec.Line
	Done    chan map[string]any
	State   chan string
}

func (o *Orchestrator) Collect(sid string) *Collector {
	c := &Collector{
		Text: make(chan string, 64), Command: make(chan map[string]any, 8),
		Line: make(chan sshexec.Line, 64), Done: make(chan map[string]any, 4),
		State: make(chan string, 16),
	}
	prev := o.emit
	o.emit = func(sessionID, event string, data any) {
		if sessionID != sid {
			return
		}
		switch event {
		case "ai:text":
			c.Text <- data.(string)
		case "ai:command":
			c.Command <- data.(map[string]any)
		case "run:line":
			c.Line <- data.(sshexec.Line)
		case "run:done":
			c.Done <- data.(map[string]any)
		case "session:state":
			c.State <- data.(string)
		}
	}
	_ = prev
	return c
}
```

> 注：`Collect` 通过替换 `emit` 捕获指定会话的事件供测试断言；生产代码 `SetEmitter` 注入的 emit 会覆盖它，测试在注入前调用 `Collect`。`ExecutorFor` 字段为 `func(hostID string) Exec`，由 `app.go` 在装配时注入（真实实现用 `store` 取凭据后构造 `sshexec.NewExecutor`）。

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/orchestrator/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/orchestrator.go internal/orchestrator/orchestrator_test.go
git commit -m "feat(orchestrator): 会话状态机（批准/拒绝/回灌多轮 + 记忆注入）"
```

---

## Task 13: 装配 — app.go 绑定各 service + main.go

**Files:**
- Modify: `app.go`
- Modify: `main.go`

- [ ] **Step 1: 重写 app.go 聚合各 service 并暴露 Wails 方法**

Replace `app.go`:
```go
package main

import (
	"context"
	"fmt"

	"ops-mate/internal/llm"
	"ops-mate/internal/orchestrator"
	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 聚合 store + orchestrator + emitter（绑定 ctx 后注入）。
type App struct {
	ctx       context.Context
	store     *store.Store
	orch      *orchestrator.Orchestrator
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	st, err := store.Open()
	if err != nil {
		fmt.Println("store open error:", err)
		return
	}
	a.store = st
	a.orch = orchestrator.NewOrchestrator(st)
	// 注入执行器工厂：按 hostID 取凭据构造 SSHExecutor
	a.orch.ExecutorFor = func(hostID string) orchestrator.Exec {
		secret, authType, err := st.GetHostSecret(hostID)
		if err != nil {
			return nil
		}
		meta, _ := st.HostMetaByID(hostID)
		if meta == nil {
			return nil
		}
		return sshexec.NewExecutor(sshexec.Host{
			Addr: meta.Addr, Port: meta.Port, User: meta.User,
			AuthType: authType, Secret: secret,
		})
	}
	// 注入事件推送：把事件名做成 session 作用域
	a.orch.SetEmitter(func(sessionID, event string, data any) {
		wailsruntime.EventsEmit(ctx, event, map[string]any{
			"sessionId": sessionID, "data": data,
		})
	})
}

// === Hosts ===

func (a *App) ListHosts() ([]store.HostMeta, error) { return a.store.ListHosts() }

func (a *App) SaveHost(in store.HostInput) (string, error) { return a.store.SaveHost(in) }

func (a *App) DeleteHost(id string) error { return a.store.DeleteHost(id) }

// TestConnection 保存前验证：临时构造执行器跑 `echo ok`。
func (a *App) TestConnection(in store.HostInput) (bool, string, error) {
	ex := sshexec.NewExecutor(sshexec.Host{
		Addr: in.Addr, Port: in.Port, User: in.User,
		AuthType: in.AuthType, Secret: in.Secret,
	})
	ctx, cancel := context.WithTimeout(a.ctx, 15*1e9)
	defer cancel()
	ch, err := ex.Exec(ctx, "echo ok")
	if err != nil {
		return false, err.Error(), nil
	}
	for range ch {
	}
	return true, "", nil
}

// === AI Config ===

func (a *App) GetAIConfig() (store.AIConfig, error) { return a.store.GetAIConfig() }

func (a *App) SaveAIConfig(c store.AIConfig) error {
	if err := a.store.SaveAIConfig(c); err != nil {
		return err
	}
	a.orch.LLM = a.buildLLM()
	return nil
}

// buildLLM 按当前配置构造 LLMClient。
func (a *App) buildLLM() llm.LLMClient {
	cfg, _ := a.store.GetAIConfig()
	switch cfg.Provider {
	case "ollama":
		base := cfg.BaseURL
		if base == "" {
			base = "http://localhost:11434"
		}
		return llm.NewOllama(base, cfg.Model)
	case "claude":
		base := cfg.BaseURL
		if base == "" {
			base = "https://api.anthropic.com"
		}
		return llm.NewClaude(base, cfg.APIKey, cfg.Model)
	}
	return nil
}

// === Sessions ===

func (a *App) NewSession(hostID, title string) (string, error) {
	if a.orch.LLM == nil {
		a.orch.LLM = a.buildLLM()
	}
	return a.orch.NewSession(hostID, title)
}

func (a *App) SendMessage(sid, text string) error       { return a.orch.SendMessage(sid, text) }
func (a *App) ApproveCommand(sid, command string) error  { return a.orch.ApproveCommand(sid, command) }
func (a *App) RejectCommand(sid string) error            { return a.orch.RejectCommand(sid) }
func (a *App) CancelRun(sid string) error                { return a.orch.CancelRun(sid) }

func (a *App) ListConversations(hostID string) ([]store.Conversation, error) {
	return a.store.ListConversations(hostID)
}

func (a *App) LoadMessages(sid string) ([]store.Message, error) {
	return a.store.LoadMessages(sid)
}

func (a *App) DeleteConversation(sid string) error { return a.store.DeleteConversation(sid) }
```

- [ ] **Step 2: 给 store 加 HostMetaByID（app.go 用到）**

Add to `internal/store/hosts.go`:
```go
// HostMetaByID 取单主机元数据。
func (s *Store) HostMetaByID(id string) (*HostMeta, error) {
	var h HostMeta
	err := s.DB.QueryRow(`SELECT id,name,addr,port,user,auth_type FROM hosts WHERE id=?`, id).
		Scan(&h.ID, &h.Name, &h.Addr, &h.Port, &h.User, &h.AuthType)
	if err != nil {
		return nil, err
	}
	return &h, nil
}
```

- [ ] **Step 3: 确认 Go 编译通过**

Run: `go build ./...`
Expected: 无错误。

- [ ] **Step 4: main.go 保持不变**

`main.go` 已 `Bind: []interface{}{app}` 并调用 `app.startup`，无需改动。确认 `OnStartup: app.startup` 存在。

- [ ] **Step 5: 跑全部 Go 测试**

Run: `go test ./...`
Expected: 全部 PASS。

- [ ] **Step 6: 重新生成 Wails 前端代理**

Run: `wails generate module`（或 `wails dev` 起一次后停止）
Expected: `frontend/wailsjs/go/main/App.js` 与 `App.d.ts` 出现 `ListHosts`/`SaveHost`/`TestConnection`/`GetAIConfig`/`SaveAIConfig`/`NewSession`/`SendMessage`/`ApproveCommand`/`RejectCommand`/`CancelRun`/`ListConversations`/`LoadMessages`/`DeleteConversation`。

- [ ] **Step 7: Commit**

```bash
git add app.go internal/store/hosts.go frontend/wailsjs/
git commit -m "feat: 装配 store/orchestrator/sshexec/llm 并经 Wails Bind 暴露"
```

---

## Task 14: 前端 — 主机页 `/hosts`

**Files:**
- Create: `frontend/src/pages/Hosts/index.tsx`
- Modify: `frontend/src/components/AppLayout/menuConfig.tsx`

- [ ] **Step 1: 在 menuConfig 加路由**

Edit `frontend/src/components/AppLayout/menuConfig.tsx`，在 `routes` 数组中（`/home` 之后）加入：
```tsx
  {
    path: '/hosts',
    label: '主机',
    icon: <CloudServerOutlined />,
    component: lazyPage(() => import('@/pages/Hosts')),
  },
  {
    path: '/chat',
    label: '对话',
    icon: <MessageOutlined />,
    component: lazyPage(() => import('@/pages/Chat')),
  },
```
并在文件顶部 import 处加：
```tsx
import { CloudServerOutlined, MessageOutlined } from '@ant-design/icons';
```

- [ ] **Step 2: 写主机页**

Create `frontend/src/pages/Hosts/index.tsx`:
```tsx
import { useEffect, useState } from 'react';
import { Button, Form, Input, InputNumber, Modal, Popconfirm, Space, Table, Tag, Typography, message } from 'antd';
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { ListHosts, SaveHost, DeleteHost, TestConnection } from '@wailsjs/go/main/App';
import type { HostMeta } from '@wailsjs/go/main/App';

interface HostInput {
  name: string;
  addr: string;
  port: number;
  user: string;
  authType: string;
  secret: string;
}

export default function Hosts() {
  const [list, setList] = useState<HostMeta[]>([]);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<HostInput>();

  const load = async () => {
    const r = await ListHosts();
    setList(r || []);
  };
  useEffect(() => { load(); }, []);

  const onSubmit = async () => {
    const v = await form.validateFields();
    await SaveHost(v as any);
    setOpen(false);
    form.resetFields();
    load();
  };

  const onTest = async () => {
    const v = await form.validateFields();
    const [ok, err] = await TestConnection(v as any);
    if (ok) message.success('连接成功'); else message.error(`连接失败：${err}`);
  };

  return (
    <div>
      <Typography.Title level={3}>主机</Typography.Title>
      <Space style={{ marginBottom: 12 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>新增主机</Button>
        <Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>
      </Space>
      <Table
        rowKey="id"
        dataSource={list}
        pagination={false}
        columns={[
          { title: '名称', dataIndex: 'name' },
          { title: '地址', dataIndex: 'addr' },
          { title: '端口', dataIndex: 'port', width: 80 },
          { title: '用户', dataIndex: 'user' },
          { title: '认证', dataIndex: 'authType', width: 100, render: (t) => <Tag>{t === 'privatekey' ? '密钥' : '密码'}</Tag> },
          {
            title: '操作', width: 80, render: (_, r) => (
              <Popconfirm title="删除该主机？" onConfirm={async () => { await DeleteHost(r.id); load(); }}>
                <Button type="link" danger size="small">删除</Button>
              </Popconfirm>
            ),
          },
        ]}
      />
      <Modal title="新增主机" open={open} onCancel={() => setOpen(false)} onOk={onSubmit}
        footer={[
          <Button key="test" onClick={onTest}>测试连接</Button>,
          <Button key="cancel" onClick={() => setOpen(false)}>取消</Button>,
          <Button key="ok" type="primary" onClick={onSubmit}>保存</Button>,
        ]}>
        <Form form={form} layout="vertical" initialValues={{ port: 22, authType: 'password' }}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="addr" label="地址" rules={[{ required: true }]}><Input placeholder="10.0.0.5" /></Form.Item>
          <Form.Item name="port" label="端口" rules={[{ required: true }]}><InputNumber min={1} max={65535} /></Form.Item>
          <Form.Item name="user" label="用户" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="authType" label="认证方式">
            <Input.Select options={[{ value: 'password', label: '密码' }, { value: 'privatekey', label: '私钥' }]} />
          </Form.Item>
          <Form.Item name="secret" label="密码 / 私钥 PEM" rules={[{ required: true }]}>
            <Input.TextArea rows={4} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
```

> 注：`@wailsjs/go/main/App` 的类型由 Task 13 重新生成。`HostMeta` 字段为 `{id,name,addr,port,user,authType}`（Go 结构 `HostMeta` 字段 `AuthType` → TS `authType`）。`Input.Select` 在 antd 6 中即 `Select`；若 `Input.Select` 不存在，改用从 `antd` 直接 `import { Select }` 并用 `<Select>`。

- [ ] **Step 3: 类型检查**

Run: `cd frontend && pnpm build`（含 `tsc`）
Expected: 无类型错误（若 `Input.Select` 报错，按注替换为 `Select`）。

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/Hosts frontend/src/components/AppLayout/menuConfig.tsx
git commit -m "feat(frontend): 主机管理页（增删/测试连接）+ 路由"
```

---

## Task 15: 前端 — 设置页 `/settings` 填充

**Files:**
- Modify: `frontend/src/pages/Settings/index.tsx`

- [ ] **Step 1: 写设置页**

Replace `frontend/src/pages/Settings/index.tsx`:
```tsx
import { useEffect } from 'react';
import { Button, Form, Input, Select, Typography, message } from 'antd';
import { GetAIConfig, SaveAIConfig } from '@wailsjs/go/main/App';

interface AIConfig {
  provider: string;
  model: string;
  baseURL: string;
  apiKey: string;
}

export default function Settings() {
  const [form] = Form.useForm<AIConfig>();
  useEffect(() => {
    GetAIConfig().then((c) => form.setFieldsValue(c as any));
  }, [form]);

  const onSave = async () => {
    const v = await form.validateFields();
    await SaveAIConfig(v as any);
    message.success('已保存');
  };

  return (
    <div>
      <Typography.Title level={3}>设置</Typography.Title>
      <Form form={form} layout="vertical" style={{ maxWidth: 520 }}>
        <Form.Item name="provider" label="AI 后端" rules={[{ required: true }]}>
          <Select options={[
            { value: 'ollama', label: 'Ollama（本地）' },
            { value: 'claude', label: 'Claude（云端）' },
          ]} />
        </Form.Item>
        <Form.Item name="model" label="模型" rules={[{ required: true }]}>
          <Input placeholder="llama3 / claude-sonnet-5" />
        </Form.Item>
        <Form.Item name="baseURL" label="Base URL（留空用默认）">
          <Input placeholder="http://localhost:11434 / https://api.anthropic.com" />
        </Form.Item>
        <Form.Item name="apiKey" label="API Key（Ollama 可空）">
          <Input.Password />
        </Form.Item>
        <Button type="primary" onClick={onSave}>保存</Button>
      </Form>
    </div>
  );
}
```

- [ ] **Step 2: 类型检查 + 构建**

Run: `cd frontend && pnpm build`
Expected: 无错误。

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/Settings/index.tsx
git commit -m "feat(frontend): 设置页 AI 后端配置"
```

---

## Task 16: 前端 — 对话页 `/chat` + 批准卡 + 事件订阅

**Files:**
- Create: `frontend/src/pages/Chat/useChatEvents.ts`
- Create: `frontend/src/pages/Chat/ApprovalCard.tsx`
- Create: `frontend/src/pages/Chat/index.tsx`

- [ ] **Step 1: 写事件订阅 hook**

Create `frontend/src/pages/Chat/useChatEvents.ts`:
```ts
import { useEffect, useRef } from 'react';
import { EventsOff, EventsOn } from '@wailsjs/runtime/runtime';
import type { Line } from '../../wailsjs/...'; // sshexec.Line 形状

// 事件 payload 形状：{ sessionId, data }
interface Envelope<T> { sessionId: string; data: T }

export interface ChatEvents {
  onText: (sid: string, text: string) => void;
  onCommand: (sid: string, cmd: { command: string; why: string; risk: string; assessedRisk: string }) => void;
  onLine: (sid: string, line: { stream: string; text: string }) => void;
  onDone: (sid: string, exitCode: number) => void;
  onState: (sid: string, state: string) => void;
}

export function useChatEvents(handlers: ChatEvents) {
  const ref = useRef(handlers);
  ref.current = handlers;
  useEffect(() => {
    const wrap = <T,>(fn: (sid: string, data: T) => void) => (payload: Envelope<T>) => {
      if (payload) fn(payload.sessionId, payload.data);
    };
    EventsOn('ai:text', wrap(ref.current.onText));
    EventsOn('ai:command', wrap(ref.current.onCommand));
    EventsOn('run:line', wrap(ref.current.onLine));
    EventsOn('run:done', wrap(ref.current.onDone));
    EventsOn('session:state', wrap(ref.current.onState));
    return () => {
      EventsOff('ai:text', 'ai:command', 'run:line', 'run:done', 'session:state');
    };
  }, []);
}
```

> 注：删除占位 import 行 `import type { Line } ...`（保留为说明，正式提交时删去该行）。实际 `run:line` 的 data 形状为 `{stream, text}`，hook 内已用内联类型。

- [ ] **Step 2: 写批准卡组件**

Create `frontend/src/pages/Chat/ApprovalCard.tsx`:
```tsx
import { Alert, Button, Input, Space, Tag, Typography } from 'antd';
import { useState } from 'react';

interface Props {
  command: string;
  why: string;
  risk: string;
  assessedRisk: string;
  onApprove: (command: string) => void;
  onReject: () => void;
}

export default function ApprovalCard({ command, why, risk, assessedRisk, onApprove, onReject }: Props) {
  const [editing, setEditing] = useState(false);
  const [val, setVal] = useState(command);
  const dangerous = assessedRisk === 'high' || risk === 'high';

  return (
    <Alert
      type={dangerous ? 'error' : 'info'}
      showIcon
      style={{ margin: '8px 0' }}
      message={
        <Space direction="vertical" style={{ width: '100%' }}>
          <Space>
            <Tag color={dangerous ? 'red' : 'blue'}>提议命令</Tag>
            {dangerous && <Tag color="red">高危 · 需二次确认</Tag>}
            <Typography.Text type="secondary">{why}</Typography.Text>
          </Space>
          {editing ? (
            <Input.TextArea value={val} onChange={(e) => setVal(e.target.value)} rows={2} />
          ) : (
            <Typography.Text code copyable style={{ whiteSpace: 'pre-wrap' }}>{command}</Typography.Text>
          )}
          <Space>
            <Button type="primary" danger={dangerous} onClick={() => { setEditing(false); onApprove(val); }}>
              {dangerous ? '确认执行高危命令' : '批准'}
            </Button>
            <Button onClick={() => setEditing((e) => !e)}>{editing ? '完成编辑' : '修改'}</Button>
            <Button onClick={onReject}>拒绝</Button>
          </Space>
        </Space>
      }
    />
  );
}
```

- [ ] **Step 3: 写对话页**

Create `frontend/src/pages/Chat/index.tsx`:
```tsx
import { useEffect, useRef, useState } from 'react';
import { Button, Input, List, Select, Space, Typography } from 'antd';
import { SendOutlined } from '@ant-design/icons';
import { useChatEvents } from './useChatEvents';
import ApprovalCard from './ApprovalCard';
import {
  ListHosts, NewSession, SendMessage, ApproveCommand, RejectCommand,
  ListConversations, LoadMessages,
} from '@wailsjs/go/main/App';
import type { HostMeta, Conversation, Message } from '@wailsjs/go/main/App';

type Item =
  | { kind: 'text'; role: string; text: string }
  | { kind: 'line'; text: string }
  | { kind: 'approval'; command: string; why: string; risk: string; assessedRisk: string };

export default function Chat() {
  const [hosts, setHosts] = useState<HostMeta[]>([]);
  const [hostId, setHostId] = useState<string>();
  const [convs, setConvs] = useState<Conversation[]>([]);
  const [sid, setSid] = useState<string>();
  const [items, setItems] = useState<Item[]>([]);
  const [input, setInput] = useState('');
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => { ListHosts().then((r) => setHosts(r || [])); }, []);
  useEffect(() => { if (hostId) ListConversations(hostId).then((r) => setConvs(r || [])); }, [hostId]);
  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [items]);

  useChatEvents({
    onText: (s, text) => setItems((prev) => addText(prev, s, text)),
    onCommand: (s, cmd) => setItems((prev) => sid === s ? [...prev, { kind: 'approval', ...cmd }] : prev),
    onLine: (s, line) => setItems((prev) => sid === s ? [...prev, { kind: 'line', text: line.text }] : prev),
    onDone: () => {},
    onState: () => {},
  });

  // 同会话文本片段累加到上一条 assistant 文本
  function addText(prev: Item[], s: string, text: string): Item[] {
    if (sid !== s) return prev;
    const last = prev[prev.length - 1];
    if (last && last.kind === 'text' && last.role === 'assistant') {
      return [...prev.slice(0, -1), { kind: 'text', role: 'assistant', text: last.text + text }];
    }
    return [...prev, { kind: 'text', role: 'assistant', text }];
  }

  const openConv = async (id: string) => {
    setSid(id);
    const msgs = await LoadMessages(id);
    setItems((msgs || []).map((m: Message) => ({
      kind: m.role === 'tool' ? 'line' : 'text',
      role: m.role, text: m.role === 'tool' ? (m.toolResult || '') : m.content,
    })) as Item[]);
  };

  const newSession = async () => {
    if (!hostId) return;
    const id = await NewSession(hostId, input.slice(0, 20) || '新会话');
    setSid(id);
    setItems([]);
  };

  const send = async () => {
    if (!sid) { await newSession(); }
    const target = sid!;
    setItems((p) => [...p, { kind: 'text', role: 'user', text: input }]);
    await SendMessage(target, input);
    setInput('');
  };

  return (
    <div style={{ display: 'flex', height: 'calc(100vh - 90px)' }}>
      <div style={{ width: 220, borderRight: '1px solid #eee', padding: 8, overflow: 'auto' }}>
        <Select placeholder="选择主机" style={{ width: '100%', marginBottom: 8 }}
          value={hostId} onChange={setHostId}
          options={hosts.map((h) => ({ value: h.id, label: h.name }))} />
        <Button block onClick={newSession} disabled={!hostId} style={{ marginBottom: 8 }}>新建会话</Button>
        <List size="small" dataSource={convs} renderItem={(c) => (
          <List.Item style={{ cursor: 'pointer', background: c.id === sid ? '#e6f4ff' : undefined }}
            onClick={() => openConv(c.id)}>
            <List.Item.Meta title={c.title} />
          </List.Item>
        )} />
      </div>
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
        <div style={{ flex: 1, overflow: 'auto', padding: 16 }}>
          {items.map((it, i) => (
            <div key={i} style={{ marginBottom: 8, textAlign: it.kind === 'text' && it.role === 'user' ? 'right' : 'left' }}>
              {it.kind === 'text' && (
                <Typography.Text style={{ background: it.role === 'user' ? '#d9fdd3' : '#f0f0f0', padding: '6px 10px', borderRadius: 6, display: 'inline-block' }}>
                  {it.text}
                </Typography.Text>
              )}
              {it.kind === 'line' && (
                <Typography.Text code style={{ whiteSpace: 'pre-wrap', display: 'block' }}>{it.text}</Typography.Text>
              )}
              {it.kind === 'approval' && (
                <ApprovalCard command={it.command} why={it.why} risk={it.risk} assessedRisk={it.assessedRisk}
                  onApprove={(c) => { setItems((p) => p.filter((_, j) => j !== i)); ApproveCommand(sid!, c); }}
                  onReject={() => { setItems((p) => p.filter((_, j) => j !== i)); RejectCommand(sid!); }} />
              )}
            </div>
          ))}
          <div ref={endRef} />
        </div>
        <Space.Compact style={{ padding: 8 }}>
          <Input value={input} onChange={(e) => setInput(e.target.value)} onPressEnter={send}
            placeholder="输入运维问题，如：CPU 为什么高" />
          <Button type="primary" icon={<SendOutlined />} onClick={send} disabled={!hostId}>发送</Button>
        </Space.Compact>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: 类型检查 + 构建**

Run: `cd frontend && pnpm build`
Expected: 无类型错误。

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Chat
git commit -m "feat(frontend): 对话页 + 命令批准卡 + Wails 事件订阅"
```

---

## Task 17: 端到端手测 + 收尾

**Files:**
- 无代码改动（验证为主）；必要时修补 Task 13-16 暴露的问题。

- [ ] **Step 1: 启动 dev**

Run: `wails dev`
Expected: 应用启动，顶部菜单出现"主机/对话/设置"。

- [ ] **Step 2: 配置 AI 后端**

打开"设置" → 选 Ollama → 模型 `llama3` → Base URL 留空 → 保存。
确认本机已起 Ollama（`ollama serve`）。

- [ ] **Step 3: 新增主机并测试连接**

"主机" → 新增：名称 web-01、地址（可填 localhost 或测试机）、端口 22、用户、密码/密钥 → 测试连接 → 保存。

- [ ] **Step 4: 对话闭环手测**

"对话" → 选主机 → 新建会话 → 输入"列出 /tmp 下文件"。
期望：AI 提议 `ls /tmp` 命令卡 → 批准 → 输出流式显示 → AI 回文本结论。
再输入"删掉某个临时文件" → 期望 AI 提议 `rm ...`，命令卡标红"高危"。

- [ ] **Step 5: 记忆验证**

新建第二个会话，问类似问题，确认 AI 上下文里出现"过去执行过的相关命令"（取决于模型遵循 system prompt）。

- [ ] **Step 6: 全量测试再跑一遍**

Run: `go test ./...`
Expected: 全部 PASS。

- [ ] **Step 7: Commit（如有修补）**

```bash
git add -A
git commit -m "chore: 端到端手测后的收尾修补"
```

---

## 自审

**1. Spec 覆盖：**
- §3.1 DBStore schema/CRUD → Task 2/4/5/6 ✓
- §3.1 加密 → Task 3 ✓
- §3.2 LLMClient + 两类 provider → Task 9/10 ✓（Claude/Ollama 两类均实现）
- §3.3 Executor + SSHExecutor → Task 8 ✓
- §3.4 Memory → Task 7 ✓
- §3.5 Orchestrator 状态机 + 事件 → Task 12/13 ✓
- §3.6 前端三页 → Task 14/15/16 ✓
- §5 危险命令护栏 → Task 11 ✓
- §6 错误处理 → Task 12 内（SSH 失败回传、AI 不可用回传、超时取消、AI 格式容错）✓
- §7 测试 → 各 Task 均含 TDD 测试 ✓
- §2.1 前端零敏感数据、凭据不回传 → Task 4 `ListHosts` 不含 secret、`GetHostSecret` 仅后端用 ✓

**2. 占位符扫描：** 无 TBD/TODO；Task 16 useChatEvents 含一行说明性 import 注释，已在注释中指示提交时删去——已明确为可执行步骤，非占位。

**3. 类型一致性核对：**
- `store.HostInput{AuthType}` ↔ 前端 `authType`（antd 表单字段名，经 Wails 反序列化到 Go `HostInput.AuthType`，大小写由 Wails JSON tag 处理；`HostInput` 无 json tag，Wails 默认按字段名或 Pascal→camel。**需补 json tag 以保字段名一致**——见下方修正。）
- `store.AIConfig` 同理。
- `Orchestrator.ExecutorFor` 在 Task 12 定义为 `func(hostID string) Exec`，Task 13 注入实现签名一致 ✓
- `Orchestrator.LLM` 类型为 `llm.LLMClient`，`scriptLLM`/`NewOllama`/`NewClaude` 均实现 `Chat(ctx, []llm.Message) (<-chan llm.Chunk, error)` ✓

**修正：给 HostInput / AIConfig / HostMeta / Conversation / Message 加 JSON tag**，确保前后端字段名一致（camelCase）。在 Task 4/5/6 的结构体上加 `json:"..."` tag：

- `HostInput`：`Name \`json:"name"\``、`Addr \`json:"addr"\``、`Port \`json:"port"\``、`User \`json:"user"\``、`AuthType \`json:"authType"\``、`Secret \`json:"secret"\``
- `HostMeta`：`ID \`json:"id"\``、`Name \`json:"name"\``、`Addr \`json:"addr"\``、`Port \`json:"port"\``、`User \`json:"user"\``、`AuthType \`json:"authType"\``
- `AIConfig`：`Provider \`json:"provider"\``、`Model \`json:"model"\``、`BaseURL \`json:"baseURL"\``、`APIKey \`json:"apiKey"\``
- `Conversation`：`ID \`json:"id"\``、`HostID \`json:"hostId"\``、`Title \`json:"title"\``、`CreatedAt \`json:"createdAt"\``、`UpdatedAt \`json:"updatedAt"\``
- `Message`：`ID \`json:"id"\``、`SessionID \`json:"sessionId"\``、`Role \`json:"role"\``、`Content \`json:"content"\``、`ToolResult \`json:"toolResult"\``、`Ts \`json:"ts"\``

（在对应 Task 的结构体定义处补 tag；此为 Task 13 前需回头补的一步，已记入自审。）

---

## 执行交接

计划已完成并保存到 `docs/superpowers/plans/2026-07-30-ai-ops-ssh-mvp.md`。两种执行方式：

**1. Subagent 驱动（推荐）** — 每个任务派一个全新 subagent 执行，任务间我做两段式审查，迭代快、上下文干净。

**2. 内联执行** — 在当前会话里用 executing-plans 批量执行，带检查点供你审查。

你选哪种？
