package connector

import (
	"context"
	"testing"
)

// fakeQueryRunner 实现 QueryRunner，用于注册测试。
type fakeQueryRunner struct{}

func (fakeQueryRunner) Query(_ context.Context, query string) (*QueryResult, error) {
	return &QueryResult{}, nil
}
func (fakeQueryRunner) Exec(_ context.Context, query string) (*ExecResult, error) {
	return &ExecResult{}, nil
}

func TestRegisterAndGet(t *testing.T) {
	Register(&Driver{
		Protocol: "fake", Name: "Fake",
		Params: []ParamSchema{{Key: "db", Label: "库", Type: ParamString, Required: true}},
		New: func(cfg Config) (Capability, error) {
			return fakeQueryRunner{}, nil
		},
	})
	if d := Get("fake"); d == nil || d.Protocol != "fake" {
		t.Fatalf("Get(fake) = %+v, want 已注册", d)
	}
	if Get("nope") != nil {
		t.Fatal("Get(nope) 应为 nil")
	}
}

func TestListSortedByName(t *testing.T) {
	Register(&Driver{Protocol: "z_zzz", Name: "Zulu"})
	Register(&Driver{Protocol: "a_aaa", Name: "Alpha"})
	list := List()
	if len(list) < 2 {
		t.Fatalf("List 长度应 >= 2, got %d", len(list))
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].Name > list[i].Name {
			t.Fatalf("List 未按 Name 排序: %q > %q", list[i-1].Name, list[i].Name)
		}
	}
}

func TestNew(t *testing.T) {
	Register(&Driver{
		Protocol: "fake2", Name: "Fake2",
		New: func(cfg Config) (Capability, error) {
			return fakeQueryRunner{}, nil
		},
	})
	cap, err := New("fake2", Config{})
	if err != nil {
		t.Fatalf("New(fake2): %v", err)
	}
	if _, ok := cap.(QueryRunner); !ok {
		t.Fatalf("fake2 应实现 QueryRunner, got %T", cap)
	}
	if _, err := New("unknown", Config{}); err == nil {
		t.Fatal("New(unknown) 应报错")
	}
}

func TestRegisterPanicsOnEmptyProtocol(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("空 Protocol 注册应 panic")
		}
	}()
	Register(&Driver{Protocol: ""})
}

func TestGet_CaseInsensitive(t *testing.T) {
	Register(&Driver{Protocol: "MixedCaseProto", Name: "CS"})
	if Get("mixedcaseproto") == nil {
		t.Fatal("Get 应大小写不敏感（小写查询）")
	}
	if Get("MIXEDCASEPROTO") == nil {
		t.Fatal("Get 应大小写不敏感（大写查询）")
	}
}
