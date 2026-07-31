// Package configstore 管理 AI 提供商配置的读写与加密。
package configstore

import (
	"fmt"

	"gorm.io/gorm"

	"ops-mate/internal/store"
)

// configModel GORM 模型，对应 ai_config 单行表。
type configModel struct {
	ID               int    `gorm:"column:id;primaryKey"`
	Provider         string `gorm:"column:provider"`
	Model            string `gorm:"column:model"`
	BaseURL          string `gorm:"column:base_url"`
	APIKeyEncrypted  []byte `gorm:"column:api_key_encrypted"`
}

func (configModel) TableName() string { return "ai_config" }

// AIConfig AI 提供商配置（DTO）。
type AIConfig struct {
	Provider string `json:"provider"`
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
	enc, err := s.app.Encrypt([]byte(c.APIKey))
	if err != nil {
		return fmt.Errorf("encrypt key: %w", err)
	}
	return s.app.GORM().Save(&configModel{
		ID: 1, Provider: c.Provider, Model: c.Model,
		BaseURL: c.BaseURL, APIKeyEncrypted: enc,
	}).Error
}

func (s *ConfigStore) GetAIConfig() (AIConfig, error) {
	var m configModel
	err := s.app.GORM().First(&m, 1).Error
	if err == gorm.ErrRecordNotFound {
		return AIConfig{}, nil
	}
	if err != nil {
		return AIConfig{}, err
	}
	if len(m.APIKeyEncrypted) > 0 {
		pt, err := s.app.Decrypt(m.APIKeyEncrypted)
		if err != nil {
			return AIConfig{}, fmt.Errorf("decrypt key: %w", err)
		}
		return AIConfig{
			Provider: m.Provider, Model: m.Model,
			BaseURL: m.BaseURL, APIKey: string(pt),
		}, nil
	}
	return AIConfig{
		Provider: m.Provider, Model: m.Model, BaseURL: m.BaseURL,
	}, nil
}
