package handler

import (
	"path"
	"path/filepath"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	sftppkg "ops-mate/internal/sftp"
)

// SftpHandler 处理主机 SFTP 文件传输。
type SftpHandler struct {
	mgr *sftppkg.Manager
}

// NewSftpHandler 构造 SftpHandler。
func NewSftpHandler(mgr *sftppkg.Manager) *SftpHandler {
	return &SftpHandler{mgr: mgr}
}

// ListSftpDir 列远端目录。
func (h *SftpHandler) ListSftpDir(hostID, dir string) ([]sftppkg.Entry, error) {
	return h.mgr.List(Ctx(), hostID, dir)
}

// SftpMkdir 新建远端目录。
func (h *SftpHandler) SftpMkdir(hostID, dir string) error {
	return h.mgr.Mkdir(Ctx(), hostID, dir)
}

// SftpDelete 删除远端文件/空目录。
func (h *SftpHandler) SftpDelete(hostID, p string) error {
	return h.mgr.Remove(Ctx(), hostID, p)
}

// SftpRename 重命名/移动远端条目。
func (h *SftpHandler) SftpRename(hostID, oldPath, newPath string) error {
	return h.mgr.Rename(Ctx(), hostID, oldPath, newPath)
}

// UploadSftp 上传：原生对话框选本地文件，上传到远端目录。
// 返回上传的文件名（用户取消返回空串、nil）。
func (h *SftpHandler) UploadSftp(hostID, remoteDir string) (string, error) {
	local, err := wailsruntime.OpenFileDialog(Ctx(), wailsruntime.OpenDialogOptions{
		Title: "选择要上传的文件",
	})
	if err != nil || local == "" {
		return "", nil // 用户取消
	}
	remotePath := path.Join(remoteDir, filepath.Base(local))
	if err := h.mgr.Upload(Ctx(), hostID, local, remotePath); err != nil {
		return "", err
	}
	return filepath.Base(local), nil
}

// DownloadSftp 下载：原生对话框选保存位置。
// 返回本地保存路径（用户取消返回空串、nil）。
func (h *SftpHandler) DownloadSftp(hostID, remotePath string) (string, error) {
	local, err := wailsruntime.SaveFileDialog(Ctx(), wailsruntime.SaveDialogOptions{
		Title: "选择保存位置",
	})
	if err != nil || local == "" {
		return "", nil // 用户取消
	}
	if err := h.mgr.Download(Ctx(), hostID, remotePath, local); err != nil {
		return "", err
	}
	return local, nil
}
