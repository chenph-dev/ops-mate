// Package configstore 管理 AI 提供商配置的读写与加密。
package configstore

import (
	"database/sql"
	"fmt"

	"ops-mate/internal/store"
)

// AIConfig AI 提供商配置。
type AIConfig struct {
	Provider string `json:"provider"` // "ollama" | "claude" | "openai" 等
	Model    string `json:"model"`
	BaseURL  string `json:"baseURL"`
	APIKey   string `json:"apiKey"` // 内存中明文；落库加密
}

// ConfigStore 提供 AI 配置读写。
type ConfigStore struct {
	app *store.DB
}

// NewConfigStore 构造 ConfigStore。
func NewConfigStore(app *store.DB) *ConfigStore {
	return &ConfigStore{app: app}
}

func (s *ConfigStore) SaveAIConfig(c AIConfig) error {
	var enc []byte
	if c.APIKey != "" {
		b, err := s.app.Encrypt([]byte(c.APIKey))
		if err != nil {
			return fmt.Errorf("encrypt key: %w", err)
		}
		enc = b
	}
	_, err := s.app.DB().Exec(
		`INSERT INTO ai_config(id,provider,model,base_url,api_key_encrypted) VALUES(1,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET provider=excluded.provider, model=excluded.model,
		   base_url=excluded.base_url, api_key_encrypted=excluded.api_key_encrypted`,
		c.Provider, c.Model, c.BaseURL, enc,
	)
	return err
}

func (s *ConfigStore) GetAIConfig() (AIConfig, error) {
	var c AIConfig
	var enc []byte
	err := s.app.DB().QueryRow(`SELECT provider,model,base_url,api_key_encrypted FROM ai_config WHERE id=1`).
		Scan(&c.Provider, &c.Model, &c.BaseURL, &enc)
	if err == sql.ErrNoRows {
		return AIConfig{}, nil
	}
	if err != nil {
		return AIConfig{}, err
	}
	if len(enc) > 0 {
		pt, err := s.app.Decrypt(enc)
		if err != nil {
			return AIConfig{}, fmt.Errorf("decrypt key: %w", err)
		}
		c.APIKey = string(pt)
	}
	return c, nil
}
