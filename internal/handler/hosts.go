package handler

import (
	"context"
	"time"

	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"
)

// HostsHandler 处理主机管理相关的前端调用。
type HostsHandler struct {
	store *store.Store
}

// NewHostsHandler 构造 HostsHandler。
func NewHostsHandler(store *store.Store) *HostsHandler {
	return &HostsHandler{store: store}
}

func (h *HostsHandler) ListHosts() ([]store.HostMeta, error) {
	return h.store.ListHosts()
}

func (h *HostsHandler) SaveHost(in store.HostInput) (string, error) {
	return h.store.SaveHost(in)
}

func (h *HostsHandler) DeleteHost(id string) error {
	return h.store.DeleteHost(id)
}

// TestConnection 保存前验证：临时构造执行器跑 `echo ok`。
func (h *HostsHandler) TestConnection(in store.HostInput) (bool, string, error) {
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
