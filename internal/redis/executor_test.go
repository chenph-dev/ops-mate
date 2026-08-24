package redis

import (
	"testing"

	"ops-mate/internal/connector"
)

func TestRedisRegistered(t *testing.T) {
	d := connector.Get("redis")
	if d == nil {
		t.Fatal("redis 应已注册")
	}
	if !d.IsDB() {
		t.Error("redis 应为 db 型驱动")
	}
	// Params 仅含 redis 特有参数（db），host/port/password 走通用字段
	if len(d.Params) != 1 || d.Params[0].Key != "db" {
		t.Errorf("redis Params 应只含 db，得到 %+v", d.Params)
	}

	cap, err := NewExecutor(connector.Config{Addr: "127.0.0.1", Port: 6380})
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	ex, ok := cap.(*executor)
	if !ok {
		t.Fatalf("NewExecutor 应返回 *executor, got %T", cap)
	}
	defer ex.Close()
	if _, ok := cap.(connector.Pingable); !ok {
		t.Fatal("executor 应实现 Pingable")
	}
	o := ex.client.Options()
	if o.Addr != "127.0.0.1:6380" {
		t.Errorf("Addr 应为 127.0.0.1:6380（读 cfg.Port），得到 %q", o.Addr)
	}
	if o.DB != 0 {
		t.Errorf("DB 默认应为 0，得到 %d", o.DB)
	}
}

func TestNewExecutor_ParamsFallback(t *testing.T) {
	// 兼容旧设计：host/port/password/db 放进 Params 时回退读取
	cap, err := NewExecutor(connector.Config{
		Params: map[string]any{"host": "1.2.3.4", "port": "6390", "password": "pwd", "db": "2"},
	})
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	ex := cap.(*executor)
	defer ex.Close()
	o := ex.client.Options()
	if o.Addr != "1.2.3.4:6390" {
		t.Errorf("Params.host/port 应生效，得到 %q", o.Addr)
	}
	if o.DB != 2 {
		t.Errorf("Params.db 应解析为 2，得到 %d", o.DB)
	}
	if o.Password != "pwd" {
		t.Errorf("Params.password 应生效，得到 %q", o.Password)
	}
}

func TestParseCommand(t *testing.T) {
	cases := []struct {
		in   string
		want []any
	}{
		{"GET foo", []any{"GET", "foo"}},
		{`SET msg "hello world"`, []any{"SET", "msg", "hello world"}},
		{`SET msg 'hello world'`, []any{"SET", "msg", "hello world"}},
		{`SET msg "a\"b"`, []any{"SET", "msg", `a"b`}},
		{`SET msg "it's"`, []any{"SET", "msg", "it's"}},
		{"", nil},
		{"   ", nil},
	}
	for _, c := range cases {
		got, err := parseCommand(c.in)
		if err != nil {
			t.Errorf("parseCommand(%q) 出错: %v", c.in, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("parseCommand(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("parseCommand(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
	if _, err := parseCommand(`SET msg "abc`); err == nil {
		t.Error("未闭合引号应报错")
	}
}

func TestFormatResult(t *testing.T) {
	cases := []struct {
		name string
		in   any
		cols []string
	}{
		{"string", "v", []string{"value"}},
		{"int64", int64(42), []string{"value"}},
		{"list", []string{"a", "b"}, []string{"index", "value"}},
		{"map", map[string]string{"b": "2", "a": "1"}, []string{"key", "value"}},
		{"nil", nil, []string{"value"}},
	}
	for _, c := range cases {
		res := formatResult(c.in)
		if len(res.Columns) != len(c.cols) {
			t.Errorf("%s: columns = %v, want %v", c.name, res.Columns, c.cols)
			continue
		}
		for i := range c.cols {
			if res.Columns[i] != c.cols[i] {
				t.Errorf("%s: columns = %v, want %v", c.name, res.Columns, c.cols)
				break
			}
		}
		if len(res.Rows) == 0 {
			t.Errorf("%s: 应至少一行", c.name)
		}
	}

	// map 分支按 key 排序，行序稳定
	res := formatResult(map[string]string{"b": "2", "a": "1"})
	if len(res.Rows) != 2 || res.Rows[0][0] != "a" || res.Rows[1][0] != "b" {
		t.Errorf("map 应按 key 排序，得到 %v", res.Rows)
	}
}
