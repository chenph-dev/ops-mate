package dbexec

import (
	"strings"
	"testing"
	"time"
)

func TestDriverName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"mysql", "mysql"},
		{"MySQL", "mysql"},
		{"postgres", "postgres"},
		{"postgresql", "postgres"},
		{"pq", "postgres"},
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
	if got, want := e.dsn("mysql"), "root:p@ss@tcp(10.0.0.5:3306)/app?parseTime=true"; got != want {
		t.Errorf("mysql dsn = %q, want %q", got, want)
	}

	pg := NewExecutor(Host{Driver: "postgres", Addr: "10.0.0.6", Port: 5432, User: "pg", Password: "p@ss/", Database: "db"})
	got := pg.dsn("postgres")
	if !strings.Contains(got, "postgres://pg:p%40ss%2F@10.0.0.6:5432/db") {
		t.Errorf("postgres dsn = %q（密码应 URL 转义）", got)
	}
	if !strings.Contains(got, "sslmode=disable") {
		t.Errorf("postgres dsn 缺 sslmode=disable: %q", got)
	}
}

func TestNormalizeValue(t *testing.T) {
	if normalizeValue(nil) != nil {
		t.Error("nil 应保持 nil")
	}
	if got := normalizeValue([]byte("abc")); got != "YWJj" { // base64("abc")
		t.Errorf("[]byte 应转 base64，got %v", got)
	}
	ts := time.Date(2026, 8, 14, 10, 30, 0, 0, time.UTC)
	if got := normalizeValue(ts); got != "2026-08-14 10:30:00" {
		t.Errorf("time.Time 应格式化为字符串，got %v", got)
	}
	if got := normalizeValue(int64(42)); got != int64(42) {
		t.Errorf("其他类型原样返回，got %v", got)
	}
}
