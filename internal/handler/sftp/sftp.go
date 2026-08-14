// Package sftp 提供 SFTP 文件传输的 Wails 绑定 handler。
package sftp

import (
	"path/filepath"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ops-mate/internal/handler/base"
	sftppkg "ops-mate/internal/sftp"
)

// SftpHandler 处理资产 SFTP 文件传输（目录浏览 + 传输任务）。
type SftpHandler struct {
	mgr *sftppkg.Manager
}

// NewSftpHandler 构造 SftpHandler。
func NewSftpHandler(mgr *sftppkg.Manager) *SftpHandler {
	return &SftpHandler{mgr: mgr}
}

// ListSftpDir 列远端目录。
func (h *SftpHandler) ListSftpDir(hostID, dir string) ([]sftppkg.Entry, error) {
	return h.mgr.List(base.Ctx(), hostID, dir)
}

// SftpMkdir 新建远端目录。
func (h *SftpHandler) SftpMkdir(hostID, dir string) error {
	return h.mgr.Mkdir(base.Ctx(), hostID, dir)
}

// SftpDelete 删除远端文件/空目录。
func (h *SftpHandler) SftpDelete(hostID, p string) error {
	return h.mgr.Remove(base.Ctx(), hostID, p)
}

// SftpRename 重命名/移动远端条目。
func (h *SftpHandler) SftpRename(hostID, oldPath, newPath string) error {
	return h.mgr.Rename(base.Ctx(), hostID, oldPath, newPath)
}

// StartUpload 上传：原生对话框选本地文件，启动传输任务。返回 taskID（取消返回空串、nil）。
func (h *SftpHandler) StartUpload(hostID, remoteDir string) (string, error) {
	local, err := wailsruntime.OpenFileDialog(base.Ctx(), wailsruntime.OpenDialogOptions{
		Title: "选择要上传的文件",
	})
	if err != nil || local == "" {
		return "", nil // 用户取消
	}
	return h.mgr.StartUpload(base.Ctx(), hostID, local, remoteDir)
}

// StartDownload 下载：原生对话框选保存位置（默认填远端文件名），启动传输任务。
// 返回 taskID（取消返回空串、nil）。
func (h *SftpHandler) StartDownload(hostID, remotePath string) (string, error) {
	local, err := wailsruntime.SaveFileDialog(base.Ctx(), wailsruntime.SaveDialogOptions{
		Title:           "选择保存位置",
		DefaultFilename: filepath.Base(remotePath),
	})
	if err != nil || local == "" {
		return "", nil // 用户取消
	}
	return h.mgr.StartDownload(base.Ctx(), hostID, remotePath, local)
}

// PauseTask 暂停传输任务。
func (h *SftpHandler) PauseTask(id string) error {
	return h.mgr.PauseTask(id)
}

// ResumeTask 继续传输任务。
func (h *SftpHandler) ResumeTask(id string) error {
	return h.mgr.ResumeTask(id)
}

// CancelTask 取消传输任务（保留记录，状态置 cancelled）。
func (h *SftpHandler) CancelTask(id string) error {
	return h.mgr.CancelTask(id)
}

// RemoveTask 移除传输任务记录（清理已结束/已取消任务）。
func (h *SftpHandler) RemoveTask(id string) error {
	return h.mgr.RemoveTask(id)
}

// ListTasks 返回全部传输任务快照。
func (h *SftpHandler) ListTasks() ([]sftppkg.TaskInfo, error) {
	return h.mgr.ListTasks(), nil
}
