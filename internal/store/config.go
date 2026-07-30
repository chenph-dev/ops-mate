package store

import (
	"database/sql"
	"fmt"
)

type AIConfig struct {
	Provider string `json:"provider"` // "ollama" | "claude"
	Model    string `json:"model"`
	BaseURL  string `json:"baseURL"`
	APIKey   string `json:"apiKey"` // 内存中明文；落库加密
}

func (s *Store) SaveAIConfig(c AIConfig) error {
	var enc []byte
	if c.APIKey != "" {
		b, err := encrypt(s.key, []byte(c.APIKey))
		if err != nil {
			return fmt.Errorf("encrypt key: %w", err)
		}
		enc = b
	}
	_, err := s.DB.Exec(
		`INSERT INTO ai_config(id,provider,model,base_url,api_key_encrypted) VALUES(1,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET provider=excluded.provider, model=excluded.model,
		   base_url=excluded.base_url, api_key_encrypted=excluded.api_key_encrypted`,
		c.Provider, c.Model, c.BaseURL, enc,
	)
	return err
}

func (s *Store) GetAIConfig() (AIConfig, error) {
	var c AIConfig
	var enc []byte
	err := s.DB.QueryRow(`SELECT provider,model,base_url,api_key_encrypted FROM ai_config WHERE id=1`).
		Scan(&c.Provider, &c.Model, &c.BaseURL, &enc)
	if err == sql.ErrNoRows {
		return AIConfig{}, nil
	}
	if err != nil {
		return AIConfig{}, err
	}
	if len(enc) > 0 {
		pt, err := decrypt(s.key, enc)
		if err != nil {
			return AIConfig{}, fmt.Errorf("decrypt key: %w", err)
		}
		c.APIKey = string(pt)
	}
	return c, nil
}
