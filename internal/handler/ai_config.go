package handler

import configstore "ops-mate/internal/store/config"

// AIConfigHandler 处理 AI 配置相关的前端调用。
type AIConfigHandler struct {
	config *configstore.ConfigStore
}

// NewAIConfigHandler 构造 AIConfigHandler。
func NewAIConfigHandler(config *configstore.ConfigStore) *AIConfigHandler {
	return &AIConfigHandler{config: config}
}

func (h *AIConfigHandler) GetAIConfig() (configstore.AIConfig, error) {
	return h.config.GetAIConfig()
}

func (h *AIConfigHandler) SaveAIConfig(c configstore.AIConfig) error {
	return h.config.SaveAIConfig(c)
}
