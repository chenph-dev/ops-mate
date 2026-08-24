package es

import (
	"testing"

	"ops-mate/internal/connector"
)

func TestESRegistered(t *testing.T) {
	d := connector.Get("elasticsearch")
	if d == nil {
		t.Fatal("elasticsearch 应已注册")
	}
	if !d.IsDB() {
		t.Error("elasticsearch 应为 db 型驱动")
	}
	cap, err := NewExecutor(connector.Config{Addr: "127.0.0.1", Port: 9200})
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	if _, ok := cap.(connector.QueryRunner); !ok {
		t.Fatalf("executor 应实现 QueryRunner, got %T", cap)
	}
	if _, ok := cap.(connector.Pingable); !ok {
		t.Fatal("executor 应实现 Pingable")
	}
}

func TestParseCommand(t *testing.T) {
	cases := []struct {
		in         string
		method     string
		path       string
		hasBody    bool
	}{
		{"_cat/indices", "GET", "_cat/indices", false},
		{"  _cluster/health  ", "GET", "_cluster/health", false},
		{"POST myindex/_doc", "POST", "myindex/_doc", false},
		{"myindex/_search\n{\"query\":{\"match_all\":{}}}", "GET", "myindex/_search", true},
		{"DELETE myindex", "DELETE", "myindex", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		got, err := parseCommand(c.in)
		if c.in == "" {
			if err == nil {
				t.Errorf("空命令应报错")
			}
			continue
		}
		if err != nil {
			t.Errorf("parseCommand(%q) 出错: %v", c.in, err)
			continue
		}
		if got.method != c.method || got.path != c.path {
			t.Errorf("parseCommand(%q) = %s %s, want %s %s", c.in, got.method, got.path, c.method, c.path)
		}
		if (got.body != "") != c.hasBody {
			t.Errorf("parseCommand(%q) body 存在性错误: %q", c.in, got.body)
		}
	}
}

func TestFormatResponse_Array(t *testing.T) {
	raw := []byte(`[{"index":"logs-2026","docs.count":"123","store.size":"1mb"},{"index":"users","docs.count":"50","store.size":"200kb"}]`)
	res, err := formatResponse(raw)
	if err != nil {
		t.Fatalf("formatResponse: %v", err)
	}
	if len(res.Columns) != 3 || len(res.Rows) != 2 {
		t.Fatalf("数组应 3 列 2 行, got cols=%v rows=%d", res.Columns, len(res.Rows))
	}
	if res.Rows[0][0] != "logs-2026" {
		t.Errorf("首行 index 应为 logs-2026, got %v", res.Rows[0][0])
	}
}

func TestFormatResponse_Object(t *testing.T) {
	raw := []byte(`{"cluster_name":"mycluster","status":"green","nodes":2}`)
	res, err := formatResponse(raw)
	if err != nil {
		t.Fatalf("formatResponse: %v", err)
	}
	if len(res.Rows) != 1 || len(res.Columns) != 3 {
		t.Fatalf("对象应 1 行 3 列, got cols=%v rows=%d", res.Columns, len(res.Rows))
	}
	// 列按 key 排序：cluster_name / nodes / status
	if res.Columns[0] != "cluster_name" || res.Columns[1] != "nodes" || res.Columns[2] != "status" {
		t.Fatalf("对象列应按 key 排序: %v", res.Columns)
	}
	if res.Rows[0][2] != "green" {
		t.Errorf("status 值应为 green, got %v", res.Rows[0])
	}
}

func TestFormatResponse_Search(t *testing.T) {
	raw := []byte(`{"hits":{"hits":[
		{"_index":"logs","_id":"1","_score":1.0,"_source":{"level":"error","msg":"boom","nested":{"a":1}}},
		{"_index":"logs","_id":"2","_score":0.5,"_source":{"level":"info","msg":"ok"}}
	]}}`)
	res, err := formatResponse(raw)
	if err != nil {
		t.Fatalf("formatResponse: %v", err)
	}
	// 列：_index/_id/_score + level/msg/nested（并集）
	if len(res.Rows) != 2 || len(res.Columns) != 6 {
		t.Fatalf("hits 应 2 行 6 列, got cols=%v rows=%d", res.Columns, len(res.Rows))
	}
	// 嵌套值应序列化为 JSON 字符串
	if res.Rows[0][5] != `{"a":1}` {
		t.Errorf("嵌套值应 JSON 字符串, got %v", res.Rows[0][5])
	}
}

func TestFormatResponse_NonJSON(t *testing.T) {
	res, err := formatResponse([]byte("green 2026"))
	if err != nil {
		t.Fatalf("formatResponse: %v", err)
	}
	if len(res.Rows) != 1 || res.Rows[0][0] != "green 2026" {
		t.Errorf("非 JSON 应单行文本, got %v", res.Rows)
	}
}
