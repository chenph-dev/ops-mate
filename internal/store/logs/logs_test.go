package logsstore

import (
	"testing"

	"ops-mate/internal/store"
)

func TestLogs_SaveAndList(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, _ := store.Open()
	defer closeDB(app)

	s := NewLogsStore(app)
	if err := s.SaveLog(CallLog{
		SessionID: "s1", Component: "model", Name: "llm", Provider: "OpenAI",
		TokensIn: 10, TokensOut: 20, TokensTotal: 30, DurationMS: 100, OK: true,
	}); err != nil {
		t.Fatalf("SaveLog: %v", err)
	}
	if err := s.SaveLog(CallLog{
		SessionID: "s1", Component: "tool", Name: "execute_command", OK: false, Error: "boom",
	}); err != nil {
		t.Fatalf("SaveLog: %v", err)
	}

	list, err := s.ListLogs(10)
	if err != nil {
		t.Fatalf("ListLogs: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("应有 2 条，得到 %d", len(list))
	}
	// 时间倒序：tool 是后存的，应在最前
	if list[0].Component != "tool" || list[0].Error != "boom" {
		t.Fatalf("最新应在前，得到 %+v", list[0])
	}
}

func TestLogs_TokenSummary(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, _ := store.Open()
	defer closeDB(app)

	s := NewLogsStore(app)
	s.SaveLog(CallLog{Component: "model", TokensIn: 10, TokensOut: 20, TokensTotal: 30, OK: true})
	s.SaveLog(CallLog{Component: "model", TokensIn: 5, TokensOut: 5, TokensTotal: 10, OK: true})
	s.SaveLog(CallLog{Component: "tool", OK: true})

	sum, err := s.TokenSummary()
	if err != nil {
		t.Fatalf("TokenSummary: %v", err)
	}
	if sum.TotalCalls != 3 || sum.ModelCalls != 2 || sum.ToolCalls != 1 {
		t.Fatalf("calls 不符: %+v", sum)
	}
	if sum.TokensIn != 15 || sum.TokensOut != 25 || sum.TokensTotal != 40 {
		t.Fatalf("tokens 不符: %+v", sum)
	}
}

func closeDB(app *store.DB) {
	sqlDB, _ := app.GORM().DB()
	if sqlDB != nil {
		sqlDB.Close()
	}
}
