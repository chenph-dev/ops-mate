package einoagent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	claudemodel "github.com/cloudwego/eino-ext/components/model/claude"
	ollamamodel "github.com/cloudwego/eino-ext/components/model/ollama"
	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"

	"ops-mate/internal/store"
)

// NewChatModel 按 store.AIConfig 构造 eino ToolCallingChatModel。
// 支持的 provider：
//   - "ollama" : 本地 Ollama，仅需 BaseURL + Model
//   - "claude" : Anthropic Claude，需 APIKey
//   - "openai" / "deepseek" / "dashscope" / "zhipu" 等：通过 OpenAI 兼容接口
//
// 返回的 model.ToolCallingChatModel 实现了 Stream/Generate，可直接用于 eino Graph，
// 也可通过 LLMAdapter 适配为 einoagent.LLMClient 接口。
func NewChatModel(ctx context.Context, cfg store.AIConfig) (model.ToolCallingChatModel, error) {
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
