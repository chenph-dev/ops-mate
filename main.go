package main

import (
	"context"
	"embed"
	"fmt"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"ops-mate/internal/einoagent"
	"ops-mate/internal/handler"
	"ops-mate/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	st, err := store.Open()
	if err != nil {
		fmt.Println("store open error:", err)
		return
	}

	chatModel, err := einoagent.NewChatModel(context.Background(), mustAIConfig(st))
	if err != nil {
		fmt.Println("build chat model error:", err)
	}

	sessionManager := einoagent.NewSessionManager(st, chatModel)

	hosts := handler.NewHostsHandler(st)
	aiConfig := handler.NewAIConfigHandler(st)
	sessions := handler.NewSessionsHandler(st, sessionManager)

	err = wails.Run(&options.App{
		Title:  "ops-mate",
		Width:  1024,
		Height: 768,
		// 允许窗口调整大小（最大化按钮需要）
		DisableResize: false,
		// 窗口无边框
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			handler.SetCtx(ctx)
		},
		// 每个 handler 是独立模块，前端通过 wailsjs/go/main/<TypeName> 访问。
		Bind: []interface{}{
			hosts,
			aiConfig,
			sessions,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// mustAIConfig 读取 AI 配置（启动阶段，错误可忽略，用空配置兜底）。
func mustAIConfig(st *store.Store) store.AIConfig {
	cfg, _ := st.GetAIConfig()
	return cfg
}
