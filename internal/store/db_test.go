package store

import "testing"

func TestOpen_CreatesAndMigrates(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	s, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer closeDB(s)

	var name string
	err = s.GORM().Raw(`SELECT name FROM sqlite_master WHERE type='table' AND name='commands_fts'`).Scan(&name).Error
	if err != nil {
		t.Fatalf("查询 FTS 表失败: %v", err)
	}
	if name != "commands_fts" {
		t.Fatalf("期望 commands_fts，得到 %s", name)
	}
}

func closeDB(s *DB) {
	sqlDB, _ := s.GORM().DB()
	if sqlDB != nil {
		sqlDB.Close()
	}
}
