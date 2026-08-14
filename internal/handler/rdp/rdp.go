// Package rdp 提供 Windows 主机 RDP 拉起的 Wails 绑定 handler。
package rdp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	hoststore "ops-mate/internal/store/hosts"
)

// RdpHandler 处理 Windows 主机 RDP 拉起。
type RdpHandler struct {
	hosts *hoststore.HostsStore
}

// NewRdpHandler 构造 RdpHandler。
func NewRdpHandler(hosts *hoststore.HostsStore) *RdpHandler {
	return &RdpHandler{hosts: hosts}
}

// OpenRdp 生成临时 .rdp 文件并拉起 mstsc.exe（仅 Windows 平台）。
func (h *RdpHandler) OpenRdp(hostID string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("仅 Windows 支持 RDP 拉起")
	}
	meta, err := h.hosts.HostMetaByID(hostID)
	if err != nil {
		return fmt.Errorf("获取主机信息失败: %w", err)
	}
	secret, authType, err := h.hosts.GetHostSecret(hostID)
	if err != nil {
		return fmt.Errorf("获取凭据失败: %w", err)
	}
	var passwordHex string
	if authType == "password" && secret != "" {
		passwordHex, err = protectPassword(secret)
		if err != nil {
			return fmt.Errorf("加密 RDP 密码失败: %w", err)
		}
	}
	content := rdpContent(meta.Addr, rdpPortOrDefault(meta.RdpPort), meta.User, passwordHex)
	path := filepath.Join(os.TempDir(), "ops-mate-rdp-"+hostID+".rdp")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("写入 .rdp 文件失败: %w", err)
	}
	if err := exec.Command("mstsc.exe", path).Start(); err != nil {
		return fmt.Errorf("启动 mstsc.exe 失败: %w", err)
	}
	return nil
}

// rdpContent 生成 .rdp 文件内容。passwordHex 为 DPAPI 加密后的十六进制
// 密码（可为空，为空时强制提示凭据）。
func rdpContent(addr string, port int, user, passwordHex string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "full address:s:%s:%d\n", sanitizeRdp(addr), port)
	fmt.Fprintf(&b, "username:s:%s\n", sanitizeRdp(user))
	if passwordHex != "" {
		fmt.Fprintf(&b, "password 51:b:%s\n", passwordHex)
		b.WriteString("prompt for credentials:i:0\n")
	} else {
		b.WriteString("prompt for credentials:i:1\n")
	}
	b.WriteString("autoreconnection enabled:i:1\n")
	return b.String()
}

// sanitizeRdp 去掉 CR/LF，防止注入 .rdp 文件指令。
func sanitizeRdp(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

// rdpPortOrDefault 空端口归一为 3389。
func rdpPortOrDefault(port int) int {
	if port == 0 {
		return 3389
	}
	return port
}
