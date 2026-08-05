package handler

import (
	"context"
	"fmt"
	"time"

	"ops-mate/internal/sshexec"
	hoststore "ops-mate/internal/store/hosts"
)

// HostsHandler 处理主机管理相关的前端调用。
type HostsHandler struct {
	hosts *hoststore.HostsStore
}

// NewHostsHandler 构造 HostsHandler。
func NewHostsHandler(hosts *hoststore.HostsStore) *HostsHandler {
	return &HostsHandler{hosts: hosts}
}

func (h *HostsHandler) ListHosts() ([]hoststore.HostMeta, error) {
	return h.hosts.ListHosts()
}

func (h *HostsHandler) SaveHost(in hoststore.HostInput) (string, error) {
	return h.hosts.SaveHost(in)
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
	ex := sshexec.NewExecutor(sshexec.Host{
		Addr: in.Addr, Port: in.Port, User: in.User,
		AuthType: in.AuthType, Secret: in.Secret,
	})
	ctx, cancel := context.WithTimeout(Ctx(), 15*time.Second)
	defer cancel()
	ch, err := ex.Exec(ctx, "echo ok")
	if err != nil {
		return false, err
	}
	for range ch {
	}
	return true, nil
}

// ExecuteCommand 在指定主机上执行单条命令，返回输出。
func (h *HostsHandler) ExecuteCommand(hostID, command string) (string, error) {
	secret, authType, err := h.hosts.GetHostSecret(hostID)
	if err != nil {
		return "", fmt.Errorf("获取主机凭据失败: %w", err)
	}
	meta, err := h.hosts.HostMetaByID(hostID)
	if err != nil {
		return "", fmt.Errorf("获取主机信息失败: %w", err)
	}
	ex := sshexec.NewExecutor(sshexec.Host{
		Addr: meta.Addr, Port: meta.Port, User: meta.User,
		AuthType: authType, Secret: secret,
	})
	ctx, cancel := context.WithTimeout(Ctx(), 30*time.Second)
	defer cancel()
	ch, err := ex.Exec(ctx, command)
	if err != nil {
		return "", err
	}
	var output string
	for line := range ch {
		if line.Stream == "stdout" {
			output += line.Text + "\n"
		} else if line.Stream == "stderr" {
			output += line.Text + "\n"
		}
	}
	return output, nil
}
