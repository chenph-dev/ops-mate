package main

import (
	"context"
	"fmt"
	"time"

	"ops-mate/internal/llm"
	"ops-mate/internal/orchestrator"
	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 聚合 store + orchestrator + emitter（绑定 ctx 后注入）。
type App struct {
	ctx   context.Context
	store *store.Store
	orch  *orchestrator.Orchestrator
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	st, err := store.Open()
	if err != nil {
		fmt.Println("store open error:", err)
		return
	}
	a.store = st
	a.orch = orchestrator.NewOrchestrator(st)
	// 注入执行器工厂：按 hostID 取凭据构造 SSHExecutor
	a.orch.ExecutorFor = func(hostID string) orchestrator.Exec {
		secret, authType, err := st.GetHostSecret(hostID)
		if err != nil {
			return nil
		}
		meta, err := st.HostMetaByID(hostID)
		if err != nil || meta == nil {
			return nil
		}
		return sshexec.NewExecutor(sshexec.Host{
			Addr: meta.Addr, Port: meta.Port, User: meta.User,
			AuthType: authType, Secret: secret,
		})
	}
	// 注入事件推送：把事件名做成 session 作用域
	a.orch.SetEmitter(func(sessionID, event string, data any) {
		wailsruntime.EventsEmit(ctx, event, map[string]any{
			"sessionId": sessionID, "data": data,
		})
	})
}

// === Hosts ===

func (a *App) ListHosts() ([]store.HostMeta, error) { return a.store.ListHosts() }

func (a *App) SaveHost(in store.HostInput) (string, error) { return a.store.SaveHost(in) }

func (a *App) DeleteHost(id string) error { return a.store.DeleteHost(id) }

// TestConnection 保存前验证：临时构造执行器跑 `echo ok`。
func (a *App) TestConnection(in store.HostInput) (bool, string, error) {
	ex := sshexec.NewExecutor(sshexec.Host{
		Addr: in.Addr, Port: in.Port, User: in.User,
		AuthType: in.AuthType, Secret: in.Secret,
	})
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	ch, err := ex.Exec(ctx, "echo ok")
	if err != nil {
		return false, err.Error(), nil
	}
	for range ch {
	}
	return true, "", nil
}

// === AI Config ===

func (a *App) GetAIConfig() (store.AIConfig, error) { return a.store.GetAIConfig() }

func (a *App) SaveAIConfig(c store.AIConfig) error {
	if err := a.store.SaveAIConfig(c); err != nil {
		return err
	}
	a.orch.LLM = a.buildLLM()
	return nil
}

// buildLLM 按当前配置构造 LLMClient。
func (a *App) buildLLM() llm.LLMClient {
	cfg, _ := a.store.GetAIConfig()
	switch cfg.Provider {
	case "ollama":
		base := cfg.BaseURL
		if base == "" {
			base = "http://localhost:11434"
		}
		return llm.NewOllama(base, cfg.Model)
	case "claude":
		base := cfg.BaseURL
		if base == "" {
			base = "https://api.anthropic.com"
		}
		return llm.NewClaude(base, cfg.APIKey, cfg.Model)
	}
	return nil
}

// === Sessions ===

func (a *App) NewSession(hostID, title string) (string, error) {
	if a.orch.LLM == nil {
		a.orch.LLM = a.buildLLM()
	}
	return a.orch.NewSession(hostID, title)
}

func (a *App) SendMessage(sid, text string) error        { return a.orch.SendMessage(sid, text) }
func (a *App) ApproveCommand(sid, command string) error  { return a.orch.ApproveCommand(sid, command) }
func (a *App) RejectCommand(sid string) error            { return a.orch.RejectCommand(sid) }
func (a *App) CancelRun(sid string) error                { return a.orch.CancelRun(sid) }

func (a *App) ListConversations(hostID string) ([]store.Conversation, error) {
	return a.store.ListConversations(hostID)
}

func (a *App) LoadMessages(sid string) ([]store.Message, error) {
	return a.store.LoadMessages(sid)
}

func (a *App) DeleteConversation(sid string) error { return a.store.DeleteConversation(sid) }
