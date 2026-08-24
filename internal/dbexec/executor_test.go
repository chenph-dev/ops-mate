package dbexec

import (
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestDriverName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"mysql", "mysql"},
		{"MySQL", "mysql"},
		{"postgres", "postgres"},
		{"postgresql", "postgres"},
		{"pq", "postgres"},
		{"sqlite", "sqlite"},
	}
	for _, c := range cases {
		got, err := driverName(c.in)
		if err != nil || got != c.want {
			t.Errorf("driverName(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	if _, err := driverName("oracle"); err == nil {
		t.Error("oracle 应不支持")
	}
	if _, err := driverName(""); err == nil {
		t.Error("空驱动应报错")
	}
}

func TestDSN(t *testing.T) {
	e := NewExecutor(Host{Driver: "mysql", Addr: "10.0.0.5", Port: 3306, User: "root", Password: "p@ss", Database: "app"})
	if got, want := e.dsn("mysql"), "root:p@ss@tcp(10.0.0.5:3306)/app?parseTime=true&tls=preferred"; got != want {
		t.Errorf("mysql dsn = %q, want %q", got, want)
	}
	// 特殊字符库名（中文/空格等）不应被 QueryEscape：go-sql-driver 只反转义
	// password、不反转义 dbname（路径段），转义会连到错误的库名报 Unknown database。
	special := NewExecutor(Host{Driver: "mysql", Addr: "10.0.0.5", Port: 3306, User: "root", Password: "x", Database: "my db"})
	if got, want := special.dsn("mysql"), "root:x@tcp(10.0.0.5:3306)/my db?parseTime=true&tls=preferred"; got != want {
		t.Errorf("mysql dsn 特殊字符库名 = %q, want %q（dbname 不应 URL 转义）", got, want)
	}

	pg := NewExecutor(Host{Driver: "postgres", Addr: "10.0.0.6", Port: 5432, User: "pg", Password: "p@ss/", Database: "db"})
	got := pg.dsn("postgres")
	if !strings.Contains(got, "postgres://pg:p%40ss%2F@10.0.0.6:5432/db") {
		t.Errorf("postgres dsn = %q（密码应 URL 转义）", got)
	}
	// 默认加密优先（prefer）：服务端支持 TLS 时加密，防明文泄露凭据/查询。
	if !strings.Contains(got, "sslmode=prefer") {
		t.Errorf("postgres dsn 缺 sslmode=prefer: %q", got)
	}
	if strings.Contains(got, "sslmode=disable") {
		t.Errorf("postgres dsn 不应显式关闭 TLS: %q", got)
	}
}

func TestNormalizeValue(t *testing.T) {
	if normalizeValue(nil) != nil {
		t.Error("nil 应保持 nil")
	}
	if got := normalizeValue([]byte("abc")); got != "abc" {
		t.Errorf("UTF-8 []byte（文本列）应转 string，got %v", got)
	}
	if got := normalizeValue([]byte{0xff, 0x00, 0x01}); got != "/wAB" { // base64
		t.Errorf("非 UTF-8 []byte（二进制列）应转 base64，got %v", got)
	}
	ts := time.Date(2026, 8, 14, 10, 30, 0, 0, time.UTC)
	if got := normalizeValue(ts); got != "2026-08-14 10:30:00" {
		t.Errorf("time.Time 应格式化为字符串，got %v", got)
	}
	if got := normalizeValue(int64(42)); got != int64(42) {
		t.Errorf("其他类型原样返回，got %v", got)
	}
}

func TestSchemaQuery(t *testing.T) {
	mysql := NewExecutor(Host{Driver: "mysql"})
	q := mysql.schemaQuery()
	if !strings.Contains(q, "DATABASE()") {
		t.Errorf("mysql 应查 DATABASE()：%q", q)
	}
	if !strings.Contains(q, "COLUMN_KEY") {
		t.Errorf("mysql 应含 COLUMN_KEY：%q", q)
	}
	if !strings.Contains(q, "'BASE TABLE','VIEW'") || !strings.Contains(q, "TABLE_TYPE AS obj_type") {
		t.Errorf("mysql 应含视图与 obj_type 列：%q", q)
	}

	pg := NewExecutor(Host{Driver: "postgres"})
	q = pg.schemaQuery()
	if !strings.Contains(q, "current_schema()") {
		t.Errorf("pg 应查 current_schema()：%q", q)
	}
	if !strings.Contains(q, "'' AS key") {
		t.Errorf("pg key 应为空串：%q", q)
	}
	if !strings.Contains(q, "'BASE TABLE','VIEW'") || !strings.Contains(q, "table_type AS obj_type") {
		t.Errorf("pg 应含视图与 obj_type 列：%q", q)
	}

	sl := NewExecutor(Host{Driver: "sqlite"})
	q = sl.schemaQuery()
	if !strings.Contains(q, "'table','view'") || !strings.Contains(q, "m.type AS obj_type") {
		t.Errorf("sqlite 应含视图与 obj_type 列：%q", q)
	}
}

func TestParseSchema(t *testing.T) {
	res := &Result{
		Columns: []string{"table_name", "column_name", "data_type", "is_nullable", "key", "obj_type"},
		Rows: [][]any{
			{"users", "id", "int", "NO", "PRI", "BASE TABLE"},
			{"users", "name", "varchar", "YES", "", "BASE TABLE"},
			{"orders", "id", "bigint", "NO", "PRI", "BASE TABLE"},
			{"v_user_sum", "total", "bigint", "YES", "", "VIEW"},
		},
	}
	s, err := parseSchema(res)
	if err != nil {
		t.Fatalf("parseSchema: %v", err)
	}
	if len(s.Tables) != 3 {
		t.Fatalf("应 3 个对象，得 %d", len(s.Tables))
	}
	users := s.Tables[0]
	if users.Name != "users" || users.Type != "table" || len(users.Columns) != 2 {
		t.Errorf("users 解析错误: %+v", users)
	}
	if users.Columns[0].Name != "id" || users.Columns[0].Key != "PRI" || users.Columns[0].IsNullable {
		t.Errorf("id 列错误: %+v", users.Columns[0])
	}
	if users.Columns[1].Name != "name" || !users.Columns[1].IsNullable {
		t.Errorf("name 列错误: %+v", users.Columns[1])
	}
	view := s.Tables[2]
	if view.Name != "v_user_sum" || view.Type != "view" || len(view.Columns) != 1 {
		t.Errorf("视图解析错误: %+v", view)
	}
	// 缺第 6 列（旧查询）回退 table
	legacy := &Result{
		Rows: [][]any{{"legacy", "id", "int", "NO", "PRI"}},
	}
	ls, err := parseSchema(legacy)
	if err != nil || ls.Tables[0].Type != "table" {
		t.Errorf("缺 type 列应回退 table，got %+v err=%v", ls, err)
	}
	if _, err := parseSchema(nil); err == nil {
		t.Error("parseSchema(nil) 应报错")
	}
}

// TestDSN_PasswordRoundtrip 验证 DSN 构造后 go-sql-driver ParseDSN 能还原原始密码：
// 转义往返必须正确，特殊字符密码（@ / % + 空格中文等）在测试连接时不被弄错。
func TestDSN_PasswordRoundtrip(t *testing.T) {
	orig := "p@ss/%+w 中"
	e := NewExecutor(Host{Driver: "mysql", Addr: "10.0.0.5", Port: 3306, User: "root", Password: orig, Database: "app"})
	cfg, err := mysql.ParseDSN(e.dsn("mysql"))
	if err != nil {
		t.Fatalf("ParseDSN: %v", err)
	}
	if cfg.Passwd != orig {
		t.Errorf("密码往返失败: 还原密码 = %q, want %q", cfg.Passwd, orig)
	}
}

func TestSQLiteDSN(t *testing.T) {
	e := NewExecutor(Host{Driver: "sqlite", Database: `C:\data\app.db`})
	if got, want := e.dsn("sqlite"), `C:\data\app.db`; got != want {
		t.Errorf("sqlite dsn = %q, want %q", got, want)
	}
}

func TestSQLiteSchemaQuery(t *testing.T) {
	e := NewExecutor(Host{Driver: "sqlite"})
	q := e.schemaQuery()
	if !strings.Contains(q, "sqlite_master") {
		t.Errorf("sqlite 应查 sqlite_master：%q", q)
	}
	if !strings.Contains(q, "pragma_table_info") {
		t.Errorf("sqlite 应用 pragma_table_info：%q", q)
	}
}
