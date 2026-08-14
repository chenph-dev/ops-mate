// Package dbexec 提供 MySQL / PostgreSQL 数据库执行器。
// 连接模型：每次执行新建连接（无会话状态），返回结构化结果（列 + 行）。
package dbexec

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// Host 描述一个数据库目标。
type Host struct {
	Driver   string // "mysql" | "postgres"
	Addr     string
	Port     int
	User     string
	Password string
	Database string
}

// Result SQL 执行结果。Rows 中的值为 JSON 友好类型
// （string/int64/float64/bool/nil/base64 二进制）。
type Result struct {
	Columns      []string `json:"columns"`
	Rows         [][]any  `json:"rows"`
	RowsAffected int64    `json:"rowsAffected,omitempty"`
}

// Executor 数据库执行器（每次执行新建连接）。
type Executor struct {
	host Host
}

// NewExecutor 构造 Executor。
func NewExecutor(h Host) *Executor {
	return &Executor{host: h}
}

// driverName 归一驱动名，同时校验支持的驱动。
func driverName(driver string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "mysql":
		return "mysql", nil
	case "postgres", "postgresql", "pq":
		return "postgres", nil
	}
	return "", fmt.Errorf("不支持的数据库驱动: %q（仅支持 mysql/postgres）", driver)
}

// dsn 构造连接串。
func (e *Executor) dsn(driver string) string {
	switch driver {
	case "postgres":
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
			e.host.User, url.QueryEscape(e.host.Password), e.host.Addr, e.host.Port, e.host.Database)
	default: // mysql
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
			e.host.User, e.host.Password, e.host.Addr, e.host.Port, e.host.Database)
	}
}

// open 新建一个数据库连接句柄（懒建连，执行后由调用方 Close）。
func (e *Executor) open(ctx context.Context) (*sql.DB, error) {
	drv, err := driverName(e.host.Driver)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(drv, e.dsn(drv))
	if err != nil {
		return nil, fmt.Errorf("构造数据库连接失败: %w", err)
	}
	// 单次执行单连接，避免每次新建连接时的连接池放大。
	db.SetMaxOpenConns(1)
	// 探测超时由 ctx 控制；这里给连接建立兜底超时。
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	return db, nil
}

// Ping 测试连接（保存前连接测试）。
func (e *Executor) Ping(ctx context.Context) error {
	db, err := e.open(ctx)
	if err != nil {
		return err
	}
	db.Close()
	return nil
}

// Query 执行查询类 SQL（SELECT/SHOW/DESC 等），返回列与行。
func (e *Executor) Query(ctx context.Context, sqlText string) (*Result, error) {
	db, err := e.open(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, sqlText)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("读取列名失败: %w", err)
	}
	result := &Result{Columns: cols}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("扫描行失败: %w", err)
		}
		row := make([]any, len(vals))
		for i, v := range vals {
			row[i] = normalizeValue(v)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// Exec 执行非查询 SQL（INSERT/UPDATE/DELETE/DDL 等），返回受影响行数。
func (e *Executor) Exec(ctx context.Context, sqlText string) (*Result, error) {
	db, err := e.open(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	res, err := db.ExecContext(ctx, sqlText)
	if err != nil {
		return nil, fmt.Errorf("执行失败: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		affected = 0
	}
	return &Result{RowsAffected: affected}, nil
}

// IsQuery 判断 SQL 首关键字是否为查询类（走 Query 返回行集）。
// 仅按首关键字粗分；WITH 开头的写语句（如 CTE + UPDATE）会保守走 Query，
// 但 Query 对无行结果返回空 Rows，不影响正确性。
func IsQuery(sqlText string) bool {
	switch firstKeyword(sqlText) {
	case "select", "show", "desc", "describe", "explain", "with", "pragma", "values":
		return true
	}
	return false
}

// firstKeyword 提取 SQL 的首个关键字（小写）。
func firstKeyword(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t\r\n"); i >= 0 {
		s = s[:i]
	}
	return strings.ToLower(s)
}

// normalizeValue 把驱动返回值转为 JSON 友好 / 前端可展示的类型。
func normalizeValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		// 二进制（BLOB）转 base64，避免 JSON 序列化失败。
		return base64.StdEncoding.EncodeToString(t)
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	default:
		return v
	}
}
