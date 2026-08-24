// Package dbexec 提供 MySQL / PostgreSQL 数据库执行器。
// 连接模型：每次执行新建连接（无会话状态），返回结构化结果（列 + 行）。
package dbexec

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"

	"ops-mate/internal/connector"
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
	case "sqlite":
		return "sqlite", nil
	case "clickhouse":
		return "clickhouse", nil
	case "sqlserver", "mssql":
		return "sqlserver", nil
	}
	return "", fmt.Errorf("不支持的数据库驱动: %q（仅支持 mysql/postgres/sqlite/clickhouse/sqlserver）", driver)
}

// dsn 构造连接串。
func (e *Executor) dsn(driver string) string {
	switch driver {
	case "postgres":
		// sslmode=prefer：服务端支持 TLS 时加密传输（防明文泄露凭据/查询），不支持时回退明文。
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=prefer",
			e.host.User, url.QueryEscape(e.host.Password), e.host.Addr, e.host.Port, e.host.Database)
	case "sqlite":
		return e.host.Database // 本地文件路径
	case "clickhouse":
		// native TCP 协议（默认 9000）；密码经 QueryEscape 后由驱动反转义
		return fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s",
			e.host.User, url.QueryEscape(e.host.Password), e.host.Addr, e.host.Port, e.host.Database)
	case "sqlserver":
		// go-mssqldb DSN（默认 1433）；database 走 query 参数，密码经 QueryEscape 后由驱动反转义
		return fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
			e.host.User, url.QueryEscape(e.host.Password), e.host.Addr, e.host.Port, e.host.Database)
	default: // mysql
		// go-sql-driver 用「最后一个 @」分割 userinfo/host、不反转义 password，
		// 密码原样拼接即可（含 @ : / 由驱动解析策略正确处理）；dbname 是路径段同样不转义。
		// 注意：不要对 password/database 用 url.QueryEscape——驱动不会反转义，
		// 转义串会作为字面值（如 %40）发给服务器导致 Access denied / Unknown database。
		// tls=preferred：服务端支持 TLS 时加密传输，不支持时回退明文。
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&tls=preferred",
			e.host.User, e.host.Password, e.host.Addr, e.host.Port,
			e.host.Database)
	}
}

// open 新建一个数据库连接句柄（懒建连，执行后由调用方 Close）。
func (e *Executor) open(ctx context.Context) (*sql.DB, error) {
	drv, err := driverName(e.host.Driver)
	if err != nil {
		return nil, err
	}
	dsn := e.dsn(drv)
	if drv == "sqlite" && e.host.Database != "" {
		// 相对路径基于应用 CWD 解析为绝对路径，避免文件落点不确定
		if abs, err := filepath.Abs(e.host.Database); err == nil {
			dsn = abs
		}
	}
	db, err := sql.Open(drv, dsn)
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
// sqlite 特殊：文件必须已存在（不自动创建），避免填错路径"测试通过"并在错误位置
// 生成空库；实际查询（open）仍自动创建，支持"新建数据库"场景。
func (e *Executor) Ping(ctx context.Context) error {
	if drv, err := driverName(e.host.Driver); err == nil && drv == "sqlite" {
		if e.host.Database == "" {
			return fmt.Errorf("sqlite 数据库文件路径为空")
		}
		path := e.host.Database
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("sqlite 数据库文件不存在：%s（若确认路径无误，保存后打开将自动创建）", path)
		}
	}
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
	Type    string   `json:"type,omitempty"` // "table" | "view"，旧数据缺失默认 table
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
	// 三驱动均在第 6 列返回对象类型 obj_type（table/view），供前端对象树分类。
	if drv == "sqlite" {
		return `SELECT m.name AS table_name, p.name AS column_name, p.type AS data_type,
       CASE WHEN p."notnull" THEN 'NO' ELSE 'YES' END AS is_nullable, '' AS key, m.type AS obj_type
FROM sqlite_master m
JOIN pragma_table_info(m.name) p
WHERE m.type IN ('table','view') AND m.name NOT LIKE 'sqlite_%'
ORDER BY m.name, p.cid`
	}
	if drv == "clickhouse" {
		// system.tables/system.columns：obj_type 为表引擎（MergeTree/View 等），非 view 引擎归表组
		return `SELECT t.name AS table_name, c.name AS column_name, c.type AS data_type,
       'YES' AS is_nullable, '' AS key, t.engine AS obj_type
FROM system.tables t
JOIN system.columns c ON c.database = t.database AND c.table = t.table
WHERE t.database = currentDatabase() AND t.is_temporary = 0
ORDER BY t.name, c.position`
	}
	if drv == "sqlserver" {
		return `SELECT t.TABLE_NAME, c.COLUMN_NAME, c.DATA_TYPE, c.IS_NULLABLE, '' AS key, t.TABLE_TYPE AS obj_type
FROM information_schema.TABLES t
JOIN information_schema.COLUMNS c ON c.TABLE_SCHEMA = t.TABLE_SCHEMA AND c.TABLE_NAME = t.TABLE_NAME
WHERE t.TABLE_CATALOG = DB_NAME() AND t.TABLE_TYPE IN ('BASE TABLE','VIEW')
ORDER BY t.TABLE_NAME, c.ORDINAL_POSITION`
	}
	if drv == "postgres" {
		return `SELECT t.table_name, c.column_name, c.data_type, c.is_nullable, '' AS key, t.table_type AS obj_type
FROM information_schema.tables t
JOIN information_schema.columns c
  ON c.table_schema = t.table_schema AND c.table_name = t.table_name
WHERE t.table_schema = current_schema() AND t.table_type IN ('BASE TABLE','VIEW')
ORDER BY t.table_name, c.ordinal_position`
	}
	return `SELECT t.TABLE_NAME, c.COLUMN_NAME, c.DATA_TYPE, c.IS_NULLABLE, c.COLUMN_KEY, t.TABLE_TYPE AS obj_type
FROM information_schema.TABLES t
JOIN information_schema.COLUMNS c
  ON c.TABLE_SCHEMA = t.TABLE_SCHEMA AND c.TABLE_NAME = t.TABLE_NAME
WHERE t.TABLE_SCHEMA = DATABASE() AND t.TABLE_TYPE IN ('BASE TABLE','VIEW')
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
		// 第 6 列为对象类型（table/view）；缺失/空回退 table（兼容旧查询）。
		objType := "table"
		if len(row) > 5 {
			if t := strings.TrimSpace(fmt.Sprintf("%v", row[5])); t != "" && t != "<nil>" {
				objType = normalizeObjectType(t)
			}
		}
		ti, ok := idx[table]
		if !ok {
			schema.Tables = append(schema.Tables, Table{Name: table, Type: objType})
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

// normalizeObjectType 把驱动返回的对象类型归一化为 "table"/"view"。
func normalizeObjectType(t string) string {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case "BASE TABLE", "TABLE":
		return "table"
	case "VIEW":
		return "view"
	default:
		return strings.ToLower(strings.TrimSpace(t))
	}
}

// IsQuery 判断 SQL 首关键字是否为查询类（委托 connector.IsQuery，供 db 工作台使用）。
func IsQuery(sqlText string) bool {
	return connector.IsQuery(sqlText)
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
