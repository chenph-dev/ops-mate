package skillsstore

import (
	"testing"

	"gorm.io/gorm"

	"ops-mate/internal/store"
)

func newTestStore(t *testing.T) *SkillStore {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	app, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, _ := app.GORM().DB(); sqlDB != nil {
			sqlDB.Close()
		}
	})
	return NewSkillStore(app)
}

func TestSkillStore_CreateGetRoundTrip(t *testing.T) {
	s := newTestStore(t)
	id, err := s.Create("nginx-check", "nginx-check", "检查 Nginx 状态与常见故障")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Fatal("Create 应返回非空 id")
	}
	got, err := s.Get("nginx-check")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "nginx-check" || got.Description != "检查 Nginx 状态与常见故障" {
		t.Fatalf("Get 结果: %+v", got)
	}
	if !got.Enabled {
		t.Fatal("Create 后默认应启用")
	}
}

func TestSkillStore_CreateDuplicateName(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Create("disk-analysis", "disk-analysis", ""); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.Create("disk-analysis", "disk-analysis", ""); err == nil {
		t.Fatal("重复 name 应报错")
	}
}

func TestSkillStore_ListSortedAndEnabledFilter(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Create("b-skill", "b", ""); err != nil {
		t.Fatalf("Create b: %v", err)
	}
	if _, err := s.Create("a-skill", "a", ""); err != nil {
		t.Fatalf("Create a: %v", err)
	}
	if err := s.SetEnabled("b-skill", false); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 || list[0].Name != "a-skill" || list[1].Name != "b-skill" {
		t.Fatalf("List 应按 name 升序: %+v", list)
	}

	enabled, err := s.ListEnabled()
	if err != nil {
		t.Fatalf("ListEnabled: %v", err)
	}
	if len(enabled) != 1 || enabled[0].Name != "a-skill" {
		t.Fatalf("ListEnabled 应只含启用项: %+v", enabled)
	}
}

func TestSkillStore_SetEnabledRoundTrip(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Create("skill-a", "a", ""); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.SetEnabled("skill-a", false); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	got, _ := s.Get("skill-a")
	if got.Enabled {
		t.Fatal("SetEnabled(false) 后应停用")
	}
	if err := s.SetEnabled("skill-a", true); err != nil {
		t.Fatalf("SetEnabled(true): %v", err)
	}
	got, _ = s.Get("skill-a")
	if !got.Enabled {
		t.Fatal("SetEnabled(true) 后应启用")
	}
}

func TestSkillStore_Delete(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Create("skill-a", "a", ""); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Delete("skill-a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get("skill-a"); err != gorm.ErrRecordNotFound {
		t.Fatalf("删除后 Get 应报 ErrRecordNotFound，got %v", err)
	}
}
