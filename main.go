package main

import (
	"context"
	"embed"
	"fmt"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ops-mate/internal/einoagent"
	"ops-mate/internal/handler"
	"ops-mate/internal/sshexec"
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
	hostsStore := hoststore.NewHostsStore(app)
	convStore := convstore.NewConvStore(app)

	// AI 模型不在启动时构建（配置可能为空/变更）——
	// SessionManager 在每轮对话开始时按最新配置懒构建（热更新）。
	sessionManager := einoagent.NewSessionManager(app, cfgStore,
		executorFor(hostsStore), emitEvent)

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
			handler.NewAIConfigHandler(cfgStore, sessionManager.InvalidateConfig),
			handler.NewSessionsHandler(convStore, sessionManager),
			handler.NewTerminalHandler(hostsStore),
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// executorFor 返回按 hostID 解析凭据并构造 SSH 执行器的工厂。
// 凭据解析失败返回 nil（SessionManager 会给出"凭据不可用"错误事件）。
func executorFor(hosts *hoststore.HostsStore) func(hostID string) sshexec.Exec {
	return func(hostID string) sshexec.Exec {
		secret, authType, err := hosts.GetHostSecret(hostID)
		if err != nil {
			return nil
		}
		meta, err := hosts.HostMetaByID(hostID)
		if err != nil || meta == nil {
			return nil
		}
		return sshexec.NewExecutor(sshexec.Host{
			Addr: meta.Addr, Port: meta.Port, User: meta.User,
			AuthType: authType, Secret: secret,
		})
	}
}

// emitEvent 统一的 Wails 事件推送（载荷 {sessionId, data}）。
func emitEvent(sessionID, event string, data any) {
	wailsruntime.EventsEmit(handler.Ctx(), event, map[string]any{
		"sessionId": sessionID, "data": data,
	})
}
