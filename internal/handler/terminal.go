package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

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

// CommandInfo 描述一个可执行命令及其简介，用于终端命令补全。
type CommandInfo struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

// ListHostCommands 抓取指定主机上可用的命令列表（compgen -c），并尝试用 whatis 获取简介。
// 结果按名称去重排序；whatis 失败或不存在时 Desc 为空。
func (h *TerminalHandler) ListHostCommands(hostID string) ([]CommandInfo, error) {
	host, err := h.getHost(hostID)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(Ctx(), 10*time.Second)
	defer cancel()

	exec := sshexec.NewExecutor(*host)
	out, err := exec.Exec(ctx, "compgen -c | sort -u")
	if err != nil {
		return nil, fmt.Errorf("抓取命令列表失败: %w", err)
	}

	nameSet := make(map[string]struct{})
	for line := range out {
		if line.Stream != "stdout" {
			continue
		}
		name := strings.TrimSpace(line.Text)
		if name == "" {
			continue
		}
		nameSet[name] = struct{}{}
	}

	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}
	sort.Strings(names)

	// 尝试获取命令描述，失败不影响主结果。
	descMap := h.fetchCommandDescriptions(ctx, host, names)

	result := make([]CommandInfo, 0, len(names))
	for _, name := range names {
		result = append(result, CommandInfo{Name: name, Desc: descMap[name]})
	}
	return result, nil
}

// fetchCommandDescriptions 分批执行 whatis 获取命令描述。
func (h *TerminalHandler) fetchCommandDescriptions(ctx context.Context, host *sshexec.Host, names []string) map[string]string {
	descMap := make(map[string]string)
	if len(names) == 0 {
		return descMap
	}

	// 控制总量，避免 whatis 过慢；只处理前 500 个命令。
	limit := 500
	if len(names) > limit {
		names = names[:limit]
	}

	batchSize := 100
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < len(names); i += batchSize {
		end := min(i+batchSize, len(names))
		batch := names[i:end]
		wg.Add(1)
		go func(batch []string) {
			defer wg.Done()
			// whatis 命令行长度安全上限由 shell 决定，100 个命令通常安全。
			cmd := "whatis " + strings.Join(batch, " ") + " 2>/dev/null"
			execCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			exec := sshexec.NewExecutor(*host)
			out, err := exec.Exec(execCtx, cmd)
			if err != nil {
				return
			}
			for line := range out {
				if line.Stream != "stdout" {
					continue
				}
				// whatis 格式：name (section) - description
				text := strings.TrimSpace(line.Text)
				namePart, descPart, ok := strings.Cut(text, ") - ")
				if !ok {
					continue
				}
				nameEnd := strings.LastIndex(namePart, " (")
				if nameEnd == -1 {
					continue
				}
				name := strings.TrimSpace(namePart[:nameEnd])
				desc := strings.TrimSpace(descPart)
				if name != "" && desc != "" {
					mu.Lock()
					descMap[name] = desc
					mu.Unlock()
				}
			}
		}(batch)
	}
	wg.Wait()
	return descMap
}

// newSessionID 生成终端会话 ID。
func newSessionID() string {
	return crypto.NewID()
}
