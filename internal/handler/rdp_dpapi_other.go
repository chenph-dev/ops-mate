//go:build !windows

package handler

import "errors"

// protectPassword 在非 Windows 平台不可用（DPAPI 仅 Windows 支持）。
func protectPassword(password string) (string, error) {
	return "", errors.New("RDP 密码加密仅支持 Windows")
}
