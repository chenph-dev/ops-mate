package main

import (
	"context"
	"embed"
	"fmt"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ops-mate/internal/einoagent/session"
	"ops-mate/internal/handler"
	sftppkg "ops-mate/internal/sftp"
	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"
	cfgstore "ops-mate/internal/store/config"
	convstore "ops-mate/internal/store/conversations"
	hoststore "ops-mate/internal/store/hosts"
	logsstore "ops-mate/internal/store/logs"
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
	policyStore := cfgstore.NewPolicyStore(app)
	hostsStore := hoststore.NewHostsStore(app)
	convStore := convstore.NewConvStore(app)
	logsStore := logsstore.NewLogsStore(app)
	// 终端会话管理：按 hostID 记录最近输出，供 AI 上下文注入。
	terminalHandler := handler.NewTerminalHandler(hostsStore)

	// AI 模型不在启动时构建（配置可能为空/变更）——
	// SessionManager 在每轮对话开始时按最新配置懒构建（热更新）。
	sessionManager := session.NewSessionManager(app, cfgStore,
		executorFor(hostsStore),
		// 解析主机名，注入系统提示词模板
		func(hostID string) (string, error) {
			meta, err := hostsStore.HostMetaByID(hostID)
			if err != nil {
				return "", err
			}
			return meta.Name, nil
		},
		emitEvent)
	// 审批分级：全局策略 + 主机覆盖（on/off/inherit）。读取失败保守关闭自动放行。
	sessionManager.SetApprovalPolicyResolver(func(hostID string) (bool, []string) {
		p, err := policyStore.GetApprovalPolicy()
		if err != nil {
			return false, nil
		}
		auto := p.EnableAuto
		if override, oerr := hostsStore.GetAutoApprove(hostID); oerr == nil {
			switch override {
			case "on":
				auto = true
			case "off":
				auto = false
			}
		}
		return auto, p.ReadOnlyList
	})
	// 终端上下文注入：每次发消息时把当前主机终端最近输出清洗后给模型。
	sessionManager.SetTerminalContextResolver(terminalHandler.TerminalContext)

	// SFTP 管理器：按 hostID 懒建立/复用连接，应用退出时关闭。
	sftpManager := sftppkg.NewManager(
		func(hostID string) (*sshexec.Host, error) {
			secret, authType, err := hostsStore.GetHostSecret(hostID)
			if err != nil {
				return nil, err
			}
			meta, err := hostsStore.HostMetaByID(hostID)
			if err != nil {
				return nil, err
			}
			return &sshexec.Host{
				Addr: meta.Addr, Port: meta.Port, User: meta.User,
				AuthType: authType, Secret: secret,
			}, nil
		},
		// 传输进度事件：推送前端实时更新任务进度
		func(t *sftppkg.Task) {
			wailsruntime.EventsEmit(handler.Ctx(), "sftp:progress", map[string]any{
				"taskID": t.ID, "done": t.Done, "total": t.Total,
				"status": string(t.Status),
			})
		},
		// 任务开始事件：前端自动打开传输任务弹窗
		func(t *sftppkg.Task) {
			wailsruntime.EventsEmit(handler.Ctx(), "sftp:task-start", map[string]any{
				"taskID": t.ID, "direction": t.Direction,
				"localPath": t.LocalPath, "remotePath": t.RemotePath,
				"total": t.Total,
			})
		},
	)

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
		OnShutdown: func(ctx context.Context) {
			sftpManager.Close()
		},
		// 每个 handler 是独立模块，前端通过 wailsjs/go/main/<TypeName> 访问。
		Bind: []interface{}{
			handler.NewHostsHandler(hostsStore, sessionManager.InvalidateConfig),
			handler.NewAIConfigHandler(cfgStore, sessionManager.InvalidateConfig),
			handler.NewApprovalPolicyHandler(policyStore, sessionManager.InvalidateConfig),
			handler.NewSessionsHandler(convStore, sessionManager),
			terminalHandler,
			handler.NewSftpHandler(sftpManager),
			handler.NewLogsHandler(logsStore),
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
