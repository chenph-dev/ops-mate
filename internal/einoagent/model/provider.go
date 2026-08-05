package model

import (
	"context"
	"fmt"

	einomodel "github.com/cloudwego/eino/components/model"
	claudemodel "github.com/cloudwego/eino-ext/components/model/claude"
	ollamamodel "github.com/cloudwego/eino-ext/components/model/ollama"
	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"

	"ops-mate/internal/store/config"
)

// NewChatModel 按 store.AIConfig 构造 eino ToolCallingChatModel。
// Provider 现在是"协议"语义，由前端 AI 配置表单选择：
//   - "openai" : OpenAI 兼容协议。OpenAI / DeepSeek / 通义 / 智谱 / Ollama(v1) 等
//     服务商都走此分支，差异通过 BaseURL（完整地址，含 /v1）与 Model 表达。
//   - "claude" : Anthropic 官方协议（Claude 系列），BaseURL 默认 https://api.anthropic.com。
//
// 以下历史值仅作旧配置兼容（前端已不再展示）：
//   - "ollama" : 本地 Ollama 原生协议（等价选 openai + baseURL=http://localhost:11434/v1）
//   - "deepseek" / "dashscope" / "zhipu" 等：等价选 openai + 各自 BaseURL
//
// 返回的 model.ToolCallingChatModel 实现了 Stream/Generate，可直接用于 eino Graph。
func NewChatModel(ctx context.Context, cfg configstore.AIConfig) (einomodel.ToolCallingChatModel, error) {
	switch cfg.Provider {
	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return ollamamodel.NewChatModel(ctx, &ollamamodel.ChatModelConfig{
			BaseURL: baseURL,
			Model:   cfg.Model,
		})

	case "claude":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		url := baseURL
		return claudemodel.NewChatModel(ctx, &claudemodel.Config{
			APIKey:     cfg.APIKey,
			BaseURL:    &url,
			Model:      cfg.Model,
			MaxTokens:  2048,
			HTTPClient: nil,
		})

	case "openai", "deepseek", "dashscope", "zhipu", "moonshot", "volcengine":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		return openaimodel.NewChatModel(ctx, &openaimodel.ChatModelConfig{
			APIKey:  cfg.APIKey,
			BaseURL: baseURL,
			Model:   cfg.Model,
		})

	default:
		return nil, fmt.Errorf("unsupported AI provider: %q", cfg.Provider)
	}
}
