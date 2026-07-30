package store

import "testing"

func TestOpen_CreatesAndMigrates(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	s, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.DB.Close()

	var name string
	err = s.DB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='commands_fts'`).Scan(&name)
	if err != nil {
		t.Fatalf("查询 FTS 表失败: %v", err)
	}
	if name != "commands_fts" {
		t.Fatalf("期望 commands_fts，得到 %s", name)
	}
}
