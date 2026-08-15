package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ops-mate/internal/einoagent/session"
	"ops-mate/internal/handler/aiconfig"
	"ops-mate/internal/handler/approvalpolicy"
	"ops-mate/internal/handler/base"
	"ops-mate/internal/handler/db"
	"ops-mate/internal/handler/hosts"
	"ops-mate/internal/handler/logs"
	"ops-mate/internal/handler/rdp"
	"ops-mate/internal/handler/sessions"
	"ops-mate/internal/handler/sftp"
	"ops-mate/internal/handler/skills"
	"ops-mate/internal/handler/terminal"
	sftppkg "ops-mate/internal/sftp"
	"ops-mate/internal/skill"
	"ops-mate/internal/store"
	cfgstore "ops-mate/internal/store/config"
	convstore "ops-mate/internal/store/conversations"
	hoststore "ops-mate/internal/store/hosts"
	logsstore "ops-mate/internal/store/logs"
	skillsstore "ops-mate/internal/store/skills"
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
	// 运维技能：DB 元数据 + 磁盘文件（<DataDir>/skills/）。技能增删/启停后使 Graph 失效。
	dataDir, err := store.DataDir()
	if err != nil {
		fmt.Println("data dir error:", err)
		return
	}
	skillRoot := filepath.Join(dataDir, "skills")
	if err := os.MkdirAll(skillRoot, 0o700); err != nil {
		fmt.Println("mkdir skills dir error:", err)
		return
	}
	skillStore := skillsstore.NewSkillStore(app)
	skillManager := skill.NewManager(skillStore, skillRoot)
	// 资产连接解析器：收敛凭据读取 + 协议分流（ssh/winrm），
	// AI 会话、命令执行、连接测试、SFTP、终端共用一个解析入口。
	resolver := base.NewExecutorResolver(hostsStore)

	// 终端会话管理：按 hostID 记录最近输出，供 AI 上下文注入。
	terminalHandler := terminal.NewTerminalHandler(hostsStore)

	// AI 模型不在启动时构建（配置可能为空/变更）——
	// SessionManager 在每轮对话开始时按最新配置懒构建（热更新）。
	sessionManager := session.NewSessionManager(app, cfgStore,
		resolver.ExecFor,
		// 解析资产名，注入系统提示词模板
		func(hostID string) (string, error) {
			meta, err := hostsStore.HostMetaByID(hostID)
			if err != nil {
				return "", err
			}
			return meta.Name, nil
		},
		emitEvent)
	// 审批分级：全局策略 + 资产覆盖（on/off/inherit）。读取失败保守关闭自动放行。
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
	// 终端上下文注入：每次发消息时把当前资产终端最近输出清洗后给模型。
	sessionManager.SetTerminalContextResolver(terminalHandler.TerminalContext)
	// 运维技能：技能目录注入系统提示词 + 技能工具解析（load_skill / run_skill_script）。
	sessionManager.SetSkillResolver(skillManager.Catalog, skillManager.Lookup)
	// 协议解析：AI 智能体按资产协议切换 Linux/PowerShell/SQL 语义。
	sessionManager.SetProtocolResolver(func(hostID string) string {
		meta, err := hostsStore.HostMetaByID(hostID)
		if err != nil || meta == nil {
			return "ssh"
		}
		return meta.Protocol
	})
	// 连接能力解析：数据库资产经 connector 注册表构造 execute_sql 工具（DB 语义由驱动 SkillPack 提供）。
	sessionManager.SetCapabilityResolver(resolver.ConnFor)

	// SFTP 管理器：按 hostID 懒建立/复用连接，应用退出时关闭。
	// hostFor 由 resolver.HostFor 提供（仅 SSH，WinRM 返回错误）。
	sftpManager := sftppkg.NewManager(
		resolver.HostFor,
		// 传输进度事件：推送前端实时更新任务进度
		func(t *sftppkg.Task) {
			wailsruntime.EventsEmit(base.Ctx(), "sftp:progress", map[string]any{
				"taskID": t.ID, "done": t.Done, "total": t.Total,
				"status": string(t.Status),
			})
		},
		// 任务开始事件：前端自动打开传输任务弹窗
		func(t *sftppkg.Task) {
			wailsruntime.EventsEmit(base.Ctx(), "sftp:task-start", map[string]any{
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
			base.SetCtx(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			sftpManager.Close()
		},
		// 每个 handler 是独立子包，前端通过 wailsjs/go/<子包>/<TypeName> 访问。
		Bind: []any{
			hosts.NewHostsHandler(hostsStore, sessionManager.InvalidateConfig),
			aiconfig.NewAIConfigHandler(cfgStore, sessionManager.InvalidateConfig),
			approvalpolicy.NewApprovalPolicyHandler(policyStore, sessionManager.InvalidateConfig),
			sessions.NewSessionsHandler(convStore, sessionManager),
			terminalHandler,
			skills.NewSkillsHandler(skillManager, sessionManager.InvalidateConfig),
			sftp.NewSftpHandler(sftpManager),
			logs.NewLogsHandler(logsStore),
			rdp.NewRdpHandler(hostsStore),
			db.NewDbHandler(resolver),
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// emitEvent 统一的 Wails 事件推送（载荷 {sessionId, data}）。
func emitEvent(sessionID, event string, data any) {
	wailsruntime.EventsEmit(base.Ctx(), event, map[string]any{
		"sessionId": sessionID, "data": data,
	})
}
