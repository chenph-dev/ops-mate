package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ops-mate/internal/store"
	skillsstore "ops-mate/internal/store/skills"
)

func openTestDB() (*store.DB, error) {
	return store.Open()
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	app, err := openTestDB()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, _ := app.GORM().DB(); sqlDB != nil {
			sqlDB.Close()
		}
	})
	store := skillsstore.NewSkillStore(app)
	root := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return NewManager(store, root)
}

func validSkillZip() []byte {
	return buildZip(map[string]string{
		"disk-analysis/SKILL.md":   "---\nname: disk-analysis\ndescription: 分析磁盘占用\n---\n运行 df -h 与 du 分析\n",
		"disk-analysis/scripts/x": "#!/usr/bin/env bash\necho ok\n",
	})
}

func TestManager_InstallAndList(t *testing.T) {
	m := newTestManager(t)
	s, err := m.Install(validSkillZip())
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if s.Name != "disk-analysis" || !s.Enabled {
		t.Fatalf("Install 结果: %+v", s)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md 应落盘: %v", err)
	}
	list, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Name != "disk-analysis" {
		t.Fatalf("List: %+v", list)
	}
}

func TestManager_InstallDuplicate(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Install(validSkillZip()); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := m.Install(validSkillZip()); err == nil {
		t.Fatal("同名二次安装应报错")
	}
}

func TestManager_InstallMissingSkillMD(t *testing.T) {
	m := newTestManager(t)
	_, err := m.Install(buildZip(map[string]string{"foo.txt": "x"}))
	if err == nil {
		t.Fatal("缺 SKILL.md 应报错")
	}
}

func TestManager_CatalogRespectsEnabled(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Install(validSkillZip()); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if c := m.Catalog(); !strings.Contains(c, "disk-analysis") {
		t.Fatalf("Catalog 应含技能: %q", c)
	}
	if err := m.SetEnabled("disk-analysis", false); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if c := m.Catalog(); c != "" {
		t.Fatalf("停用后 Catalog 应为空: %q", c)
	}
}

func TestManager_DeleteRemovesFiles(t *testing.T) {
	m := newTestManager(t)
	s, err := m.Install(validSkillZip())
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := m.Delete(s.Name); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(s.Dir); !os.IsNotExist(err) {
		t.Fatalf("删除后目录应移除，got err=%v", err)
	}
	if _, err := m.Lookup(s.Name); err == nil {
		t.Fatal("删除后 Lookup 应报错")
	}
}

func TestManager_ReadMarkdownAndScriptPath(t *testing.T) {
	m := newTestManager(t)
	s, _ := m.Install(validSkillZip())
	md, err := m.ReadMarkdown(s)
	if err != nil {
		t.Fatalf("ReadMarkdown: %v", err)
	}
	if !strings.Contains(md, "disk-analysis") {
		t.Fatalf("SKILL.md 内容: %q", md)
	}
	p, err := m.ScriptPath(s, "x")
	if err != nil {
		t.Fatalf("ScriptPath: %v", err)
	}
	if filepath.Base(p) != "x" {
		t.Fatalf("ScriptPath: %q", p)
	}
	if _, err := m.ScriptPath(s, "missing.sh"); err == nil {
		t.Fatal("不存在的脚本应报错")
	}
	if _, err := m.ScriptPath(s, "../evil"); err == nil {
		t.Fatal("含路径分隔符的脚本名应报错")
	}
}
