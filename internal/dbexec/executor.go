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
	"unicode/utf8"

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

// Column 数据库表列元数据。
type Column struct {
	Name       string `json:"name"`
	DataType   string `json:"dataType"`
	IsNullable bool   `json:"isNullable"`
	Key        string `json:"key,omitempty"` // MySQL COLUMN_KEY（PRI/MUL/""），PG 留空
}

// Table 表元数据。
type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
}

// Schema 数据库结构（表 + 列）。
type Schema struct {
	Tables []Table `json:"tables"`
}

// Schema 查询目标库的表/列结构（information_schema）。
func (e *Executor) Schema(ctx context.Context) (*Schema, error) {
	res, err := e.Query(ctx, e.schemaQuery())
	if err != nil {
		return nil, err
	}
	return parseSchema(res)
}

// schemaQuery 按驱动返回 information_schema 查询 SQL（表 + 列 + 类型 + 可空 + key）。
func (e *Executor) schemaQuery() string {
	drv, err := driverName(e.host.Driver)
	if err != nil {
		drv = "mysql"
	}
	if drv == "postgres" {
		return `SELECT t.table_name, c.column_name, c.data_type, c.is_nullable, '' AS key
FROM information_schema.tables t
JOIN information_schema.columns c
  ON c.table_schema = t.table_schema AND c.table_name = t.table_name
WHERE t.table_schema = current_schema() AND t.table_type = 'BASE TABLE'
ORDER BY t.table_name, c.ordinal_position`
	}
	return `SELECT t.TABLE_NAME, c.COLUMN_NAME, c.DATA_TYPE, c.IS_NULLABLE, c.COLUMN_KEY
FROM information_schema.TABLES t
JOIN information_schema.COLUMNS c
  ON c.TABLE_SCHEMA = t.TABLE_SCHEMA AND c.TABLE_NAME = t.TABLE_NAME
WHERE t.TABLE_SCHEMA = DATABASE() AND t.TABLE_TYPE = 'BASE TABLE'
ORDER BY t.TABLE_NAME, c.ORDINAL_POSITION`
}

// parseSchema 把 Query 结果解析为 Schema（按表名分组，保持列顺序）。
func parseSchema(res *Result) (*Schema, error) {
	if res == nil {
		return nil, fmt.Errorf("schema 结果为空")
	}
	var schema Schema
	idx := make(map[string]int)
	for _, row := range res.Rows {
		if len(row) < 5 {
			continue
		}
		table := fmt.Sprintf("%v", row[0])
		ti, ok := idx[table]
		if !ok {
			schema.Tables = append(schema.Tables, Table{Name: table})
			idx[table] = len(schema.Tables) - 1
			ti = idx[table]
		}
		schema.Tables[ti].Columns = append(schema.Tables[ti].Columns, Column{
			Name:       fmt.Sprintf("%v", row[1]),
			DataType:   fmt.Sprintf("%v", row[2]),
			IsNullable: fmt.Sprintf("%v", row[3]) == "YES",
			Key:        fmt.Sprintf("%v", row[4]),
		})
	}
	return &schema, nil
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
		// 文本列（合法 UTF-8）转 string；二进制（BLOB/非 UTF-8）转 base64 避免 JSON 失败。
		// MySQL 驱动对 VARCHAR/TEXT 默认返回 []byte，直接 base64 会把表名/文本列变乱码。
		if utf8.Valid(t) {
			return string(t)
		}
		return base64.StdEncoding.EncodeToString(t)
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	default:
		return v
	}
}
