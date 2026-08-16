// Package hosts 提供资产管理与远程命令执行的 Wails 绑定 handler。
package hosts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ops-mate/internal/connector"
	"ops-mate/internal/handler/base"
	hoststore "ops-mate/internal/store/hosts"
)

// HostsHandler 处理资产管理相关的前端调用。
type HostsHandler struct {
	hosts    *hoststore.HostsStore
	onChange func() // 资产变更后回调（通知 SessionManager 策略失效，资产覆盖即时生效）
	resolver *base.ExecutorResolver
}

// NewHostsHandler 构造 HostsHandler。onChange 可为 nil。
func NewHostsHandler(hosts *hoststore.HostsStore, onChange func()) *HostsHandler {
	return &HostsHandler{hosts: hosts, onChange: onChange, resolver: base.NewExecutorResolver(hosts)}
}

func (h *HostsHandler) SaveHost(in hoststore.HostInput) (string, error) {
	return h.hosts.SaveHost(in)
}

// UpdateHost 更新资产信息（节点编辑）。
func (h *HostsHandler) UpdateHost(id string, in hoststore.HostInput) error {
	if err := h.hosts.UpdateHost(id, in); err != nil {
		return err
	}
	if h.onChange != nil {
		h.onChange()
	}
	return nil
}

// GetHostSecret 返回资产解密后的凭据（编辑表单回填密码/私钥）。
func (h *HostsHandler) GetHostSecret(hostID string) (string, error) {
	secret, _, err := h.hosts.GetHostSecret(hostID)
	return secret, err
}

func (h *HostsHandler) DeleteHost(id string) error {
	return h.hosts.DeleteNode(id)
}

func (h *HostsHandler) CreateFolder(name, parentID string) (string, error) {
	return h.hosts.CreateFolder(name, parentID)
}

func (h *HostsHandler) ListTree() ([]hoststore.TreeNode, error) {
	return h.hosts.ListTree()
}

func (h *HostsHandler) MoveNode(nodeID, newParentID string) error {
	return h.hosts.MoveNode(nodeID, newParentID)
}

func (h *HostsHandler) DeleteNode(nodeID string) error {
	return h.hosts.DeleteNode(nodeID)
}

// TestConnection 保存前验证：临时构造执行器跑 `echo ok`。
// 注意：返回 (bool, error) 两个值——Wails 绑定层（boundMethod.go）仅支持
// 1~2 个返回值，返回 3 个值（如 bool+string+error）时前端恒得到空值，
// 导致连接成功也显示"失败"。失败原因走 error 供前端 catch 展示。
func (h *HostsHandler) TestConnection(in hoststore.HostInput) (bool, error) {
	protocol := in.Protocol
	// 数据库驱动（IsDB）：走 connector 注册表构造 + Pingable；命令型走 ExecutorForHost
	if d := connector.Get(protocol); d != nil && d.IsDB() {
		params := in.Params
		if params == nil {
			params = map[string]any{}
		}
		cap, err := connector.New(protocol, connector.Config{
			Addr: in.Addr, Port: in.Port, User: in.User,
			Password: in.Secret, Params: params,
		})
		if err != nil {
			return false, err
		}
		pingable, ok := cap.(connector.Pingable)
		if !ok {
			return false, fmt.Errorf("连接类型 %q 不支持连接测试", protocol)
		}
		ctx, cancel := context.WithTimeout(base.Ctx(), 15*time.Second)
		defer cancel()
		if err := pingable.Ping(ctx); err != nil {
			return false, err
		}
		return true, nil
	}
	ex := base.ExecutorForHost(in.Protocol, in.Addr, in.Port, in.User, in.AuthType, in.Secret)
	if ex == nil {
		return false, fmt.Errorf("无法解析资产连接，请确认协议与凭据")
	}
	ctx, cancel := context.WithTimeout(base.Ctx(), 15*time.Second)
	defer cancel()
	ch, err := ex.Exec(ctx, "echo ok")
	if err != nil {
		return false, err
	}
	for ln := range ch {
		if ln.Stream == "error" {
			return false, fmt.Errorf("%s", ln.Text)
		}
		if ln.Stream == "exit" {
			return false, fmt.Errorf("命令执行失败：%s", ln.Text)
		}
	}
	return true, nil
}

// ExecuteCommand 在指定资产上执行单条命令，返回输出。
// 按资产协议分流（ssh/winrm），协议分支收敛在 ExecutorResolver。
func (h *HostsHandler) ExecuteCommand(hostID, command string) (string, error) {
	ex := h.resolver.ExecFor(hostID)
	if ex == nil {
		return "", fmt.Errorf("获取资产执行器失败，请确认资产凭据已录入")
	}
	ctx, cancel := context.WithTimeout(base.Ctx(), 30*time.Second)
	defer cancel()
	ch, err := ex.Exec(ctx, command)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for line := range ch {
		switch line.Stream {
		case "stdout":
			sb.WriteString(line.Text)
			sb.WriteString("\n")
		case "stderr":
			sb.WriteString(line.Text)
			sb.WriteString("\n")
		case "error":
			return "", fmt.Errorf("%s", line.Text)
		case "exit":
			return "", fmt.Errorf("命令执行失败：%s", line.Text)
		}
	}
	return sb.String(), nil
}
