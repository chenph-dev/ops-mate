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
	cfgstore "ops-mate/internal/store/config"
	convstore "ops-mate/internal/store/conversations"
	hoststore "ops-mate/internal/store/hosts"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app, err := store.Open()
	if err != nil {
		fmt.Println("store open error:", err)
		return
	}

	cfgStore := cfgstore.NewConfigStore(app)
	chatModel, err := einoagent.NewChatModel(context.Background(), mustAIConfig(cfgStore))
	if err != nil {
		fmt.Println("build chat model error:", err)
	}

	sessionManager := einoagent.NewSessionManager(app, chatModel)

	hostsStore := hoststore.NewHostsStore(app)
	convStore := convstore.NewConvStore(app)

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
			handler.NewHostsHandler(hostsStore),
			handler.NewAIConfigHandler(cfgStore),
			handler.NewSessionsHandler(hostsStore, convStore, sessionManager),
			handler.NewTerminalHandler(hostsStore),
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// mustAIConfig 读取 AI 配置（启动阶段，错误可忽略，用空配置兜底）。
func mustAIConfig(s *cfgstore.ConfigStore) cfgstore.AIConfig {
	cfg, _ := s.GetAIConfig()
	return cfg
}
