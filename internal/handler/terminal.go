package handler

import (
	"encoding/base64"
	"fmt"
	"sync"

	"ops-mate/internal/sshexec"
	"ops-mate/internal/store/crypto"
	hoststore "ops-mate/internal/store/hosts"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// TerminalHandler 管理交互式 SSH 终端会话，并通过 Wails 事件推送输出。
type TerminalHandler struct {
	hosts    *hoststore.HostsStore
	sessions map[string]*sshexec.Session
	mu       sync.Mutex
}

// NewTerminalHandler 构造 TerminalHandler。
func NewTerminalHandler(hosts *hoststore.HostsStore) *TerminalHandler {
	return &TerminalHandler{hosts: hosts, sessions: map[string]*sshexec.Session{}}
}

// getHost 按 hostID 取凭据与元信息，构造 SSH 目标。
func (h *TerminalHandler) getHost(hostID string) (*sshexec.Host, error) {
	secret, authType, err := h.hosts.GetHostSecret(hostID)
	if err != nil {
		return nil, fmt.Errorf("获取主机凭据失败: %w", err)
	}
	meta, err := h.hosts.HostMetaByID(hostID)
	if err != nil {
		return nil, fmt.Errorf("获取主机信息失败: %w", err)
	}
	return &sshexec.Host{
		Addr: meta.Addr, Port: meta.Port, User: meta.User,
		AuthType: authType, Secret: secret,
	}, nil
}

// OpenTerminal 双击主机时建立交互式 SSH 会话，返回 sessionID。
func (h *TerminalHandler) OpenTerminal(hostID string, cols, rows int) (string, error) {
	host, err := h.getHost(hostID)
	if err != nil {
		return "", err
	}
	sess, err := sshexec.OpenSession(Ctx(), *host, cols, rows)
	if err != nil {
		return "", fmt.Errorf("连接失败: %w", err)
	}
	id := newSessionID()
	h.mu.Lock()
	h.sessions[id] = sess
	h.mu.Unlock()

	// 启动输出读取协程，推送到前端。
	out := sess.Output()
	go func() {
		for chunk := range out {
			wailsruntime.EventsEmit(Ctx(), "terminal:output", map[string]any{
				"sessionId": id,
				"data":      base64.StdEncoding.EncodeToString(chunk),
			})
		}
		// 会话结束，通知前端断开。
		h.CloseTerminal(id)
		wailsruntime.EventsEmit(Ctx(), "terminal:closed", map[string]any{"sessionId": id})
	}()
	return id, nil
}

// TerminalInput 将前端输入（base64）写入远程 shell stdin。
func (h *TerminalHandler) TerminalInput(sessionID, data string) error {
	sess, err := h.getSession(sessionID)
	if err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("base64 decode: %w", err)
	}
	return sess.Input(raw)
}

// TerminalResize 更新远程 PTY 尺寸。
func (h *TerminalHandler) TerminalResize(sessionID string, cols, rows int) error {
	sess, err := h.getSession(sessionID)
	if err != nil {
		return err
	}
	return sess.Resize(cols, rows)
}

// CloseTerminal 关闭指定会话。
func (h *TerminalHandler) CloseTerminal(sessionID string) {
	h.mu.Lock()
	sess, ok := h.sessions[sessionID]
	if ok {
		delete(h.sessions, sessionID)
	}
	h.mu.Unlock()
	if ok {
		sess.Close()
	}
}

func (h *TerminalHandler) getSession(sessionID string) (*sshexec.Session, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sess, ok := h.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话不存在或已关闭")
	}
	return sess, nil
}

// newSessionID 生成终端会话 ID。
func newSessionID() string {
	return crypto.NewID()
}