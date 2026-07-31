package configstore

import (
	"testing"

	"ops-mate/internal/store"
)

func TestAIConfig_SaveLoad_APIKeyEncrypted(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, _ := store.Open()
	defer closeDB(app)

	s := NewConfigStore(app)
	cfg := AIConfig{Provider: "claude", Model: "claude-sonnet-5", BaseURL: "https://api.anthropic.com", APIKey: "sk-secret"}
	if err := s.SaveAIConfig(cfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	var blob []byte
	app.GORM().Raw(`SELECT api_key_encrypted FROM ai_config WHERE id=1`).Scan(&blob)
	if string(blob) == "sk-secret" {
		t.Fatal("API Key 明文存储")
	}

	got, err := s.GetAIConfig()
	if err != nil {
		t.Fatalf("GetAIConfig: %v", err)
	}
	if got.APIKey != "sk-secret" || got.Model != "claude-sonnet-5" {
		t.Fatalf("回读不匹配: %+v", got)
	}

	if err := s.SaveAIConfig(AIConfig{Provider: "ollama", Model: "llama3", BaseURL: "http://localhost:11434", APIKey: ""}); err != nil {
		t.Fatalf("空 key Save: %v", err)
	}
	got2, _ := s.GetAIConfig()
	if got2.APIKey != "" {
		t.Fatal("空 key 应为空")
	}
}

func closeDB(app *store.DB) {
	sqlDB, _ := app.GORM().DB()
	if sqlDB != nil {
		sqlDB.Close()
	}
}
