package configstore

import (
	"reflect"
	"testing"

	"ops-mate/internal/store"
)

func TestApprovalPolicy_DefaultWhenMissing(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer closeDB(app)

	s := NewPolicyStore(app)
	p, err := s.GetApprovalPolicy()
	if err != nil {
		t.Fatalf("GetApprovalPolicy: %v", err)
	}
	if !p.EnableAuto {
		t.Error("无记录时默认应开启自动放行")
	}
	if len(p.ReadOnlyList) != 0 {
		t.Errorf("无记录时白名单应为空（回退内置默认），得到 %v", p.ReadOnlyList)
	}
}

func TestApprovalPolicy_SaveLoadRoundtrip(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer closeDB(app)

	s := NewPolicyStore(app)
	want := ApprovalPolicy{EnableAuto: false, ReadOnlyList: []string{"ls", "df", "free"}}
	if err := s.SaveApprovalPolicy(want); err != nil {
		t.Fatalf("SaveApprovalPolicy: %v", err)
	}
	got, err := s.GetApprovalPolicy()
	if err != nil {
		t.Fatalf("GetApprovalPolicy: %v", err)
	}
	if got.EnableAuto != want.EnableAuto {
		t.Errorf("EnableAuto = %v，want %v", got.EnableAuto, want.EnableAuto)
	}
	if !reflect.DeepEqual(got.ReadOnlyList, want.ReadOnlyList) {
		t.Errorf("ReadOnlyList = %v，want %v", got.ReadOnlyList, want.ReadOnlyList)
	}
}
