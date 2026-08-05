package handler

import configstore "ops-mate/internal/store/config"

// AIConfigHandler 处理 AI 配置相关的前端调用。
type AIConfigHandler struct {
	config   *configstore.ConfigStore
	onChange func() // 保存成功后回调（通知 SessionManager 使模型缓存失效）
}

// NewAIConfigHandler 构造 AIConfigHandler。onChange 可为 nil。
func NewAIConfigHandler(config *configstore.ConfigStore, onChange func()) *AIConfigHandler {
	return &AIConfigHandler{config: config, onChange: onChange}
}

func (h *AIConfigHandler) GetAIConfig() (configstore.AIConfig, error) {
	return h.config.GetAIConfig()
}

// SaveAIConfig 保存配置并触发热更新通知。
func (h *AIConfigHandler) SaveAIConfig(c configstore.AIConfig) error {
	if err := h.config.SaveAIConfig(c); err != nil {
		return err
	}
	if h.onChange != nil {
		h.onChange()
	}
	return nil
}
