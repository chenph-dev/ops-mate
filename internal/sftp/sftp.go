// Package sftp 提供基于 SSH 的 SFTP 文件传输（目录浏览、上传、下载、删除、重命名、新建目录）。
package sftp

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/pkg/sftp"

	"ops-mate/internal/sshexec"
)

// Entry 远端目录项。
type Entry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"modTime"`
}

// Manager 管理每主机的 SFTP 连接（懒建立、复用、随应用退出释放）。
type Manager struct {
	mu      sync.Mutex
	clients map[string]*sftp.Client
	hostFor func(hostID string) (*sshexec.Host, error)
}

// NewManager 构造 Manager。hostFor 按 hostID 解析主机凭据。
func NewManager(hostFor func(hostID string) (*sshexec.Host, error)) *Manager {
	return &Manager{clients: map[string]*sftp.Client{}, hostFor: hostFor}
}

// clientFor 获取（懒建立）主机的 SFTP 连接。
func (m *Manager) clientFor(ctx context.Context, hostID string) (*sftp.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.clients[hostID]; ok && c != nil {
		return c, nil
	}
	host, err := m.hostFor(hostID)
	if err != nil {
		return nil, fmt.Errorf("获取主机凭据: %w", err)
	}
	conn, err := sshexec.Dial(ctx, *host)
	if err != nil {
		return nil, err
	}
	client, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("建立 SFTP: %w", err)
	}
	m.clients[hostID] = client
	return client, nil
}

// List 列目录，返回排序后的条目。
func (m *Manager) List(ctx context.Context, hostID, dir string) ([]Entry, error) {
	c, err := m.clientFor(ctx, hostID)
	if err != nil {
		return nil, err
	}
	infos, err := c.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("列目录 %s: %w", dir, err)
	}
	entries := make([]Entry, 0, len(infos))
	for _, fi := range infos {
		entries = append(entries, Entry{
			Name: fi.Name(), IsDir: fi.IsDir(), Size: fi.Size(),
			Mode: fi.Mode().String(), ModTime: fi.ModTime().Unix(),
		})
	}
	return entries, nil
}

// Mkdir 新建目录。
func (m *Manager) Mkdir(ctx context.Context, hostID, dir string) error {
	c, err := m.clientFor(ctx, hostID)
	if err != nil {
		return err
	}
	if err := c.Mkdir(dir); err != nil {
		return fmt.Errorf("新建目录 %s: %w", dir, err)
	}
	return nil
}

// Remove 删除文件或空目录。
func (m *Manager) Remove(ctx context.Context, hostID, p string) error {
	c, err := m.clientFor(ctx, hostID)
	if err != nil {
		return err
	}
	if err := c.Remove(p); err != nil {
		return fmt.Errorf("删除 %s: %w", p, err)
	}
	return nil
}

// Rename 重命名/移动。
func (m *Manager) Rename(ctx context.Context, hostID, oldPath, newPath string) error {
	c, err := m.clientFor(ctx, hostID)
	if err != nil {
		return err
	}
	if err := c.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("重命名 %s: %w", oldPath, err)
	}
	return nil
}

// Upload 本地文件 → 远端（io.Copy 流式，支持大文件）。
func (m *Manager) Upload(ctx context.Context, hostID, localPath, remotePath string) error {
	c, err := m.clientFor(ctx, hostID)
	if err != nil {
		return err
	}
	local, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开本地文件: %w", err)
	}
	defer local.Close()
	remote, err := c.Create(remotePath)
	if err != nil {
		return fmt.Errorf("创建远端文件: %w", err)
	}
	defer remote.Close()
	if _, err := io.Copy(remote, local); err != nil {
		return fmt.Errorf("上传 %s: %w", remotePath, err)
	}
	return nil
}

// Download 远端 → 本地文件（io.Copy 流式，支持大文件）。
func (m *Manager) Download(ctx context.Context, hostID, remotePath, localPath string) error {
	c, err := m.clientFor(ctx, hostID)
	if err != nil {
		return err
	}
	remote, err := c.Open(remotePath)
	if err != nil {
		return fmt.Errorf("打开远端文件: %w", err)
	}
	defer remote.Close()
	local, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建本地文件: %w", err)
	}
	defer local.Close()
	if _, err := io.Copy(local, remote); err != nil {
		return fmt.Errorf("下载 %s: %w", remotePath, err)
	}
	return nil
}

// Close 关闭全部连接（应用退出时调用）。
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, c := range m.clients {
		_ = c.Close()
		delete(m.clients, id)
	}
}
