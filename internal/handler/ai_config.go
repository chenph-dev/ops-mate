package handler

import "ops-mate/internal/store"

// AIConfigHandler 处理 AI 配置相关的前端调用。
type AIConfigHandler struct {
	store *store.Store
}

// NewAIConfigHandler 构造 AIConfigHandler。
func NewAIConfigHandler(store *store.Store) *AIConfigHandler {
	return &AIConfigHandler{store: store}
}

func (h *AIConfigHandler) GetAIConfig() (store.AIConfig, error) {
	return h.store.GetAIConfig()
}

func (h *AIConfigHandler) SaveAIConfig(c store.AIConfig) error {
	return h.store.SaveAIConfig(c)
}
