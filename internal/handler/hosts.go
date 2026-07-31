package handler

import (
	"context"
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
func (h *HostsHandler) TestConnection(in hoststore.HostInput) (bool, string, error) {
	ex := sshexec.NewExecutor(sshexec.Host{
		Addr: in.Addr, Port: in.Port, User: in.User,
		AuthType: in.AuthType, Secret: in.Secret,
	})
	ctx, cancel := context.WithTimeout(Ctx(), 15*time.Second)
	defer cancel()
	ch, err := ex.Exec(ctx, "echo ok")
	if err != nil {
		return false, err.Error(), nil
	}
	for range ch {
	}
	return true, "", nil
}
