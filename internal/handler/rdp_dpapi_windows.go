//go:build windows

package handler

import (
	"encoding/hex"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// protectPassword 用 DPAPI（绑定当前用户）加密密码，返回十六进制字符串。
// 输出可直接写入 .rdp 的 "password 51:b:" 行，mstsc 会用 CryptUnprotectData 解密。
func protectPassword(password string) (string, error) {
	u16 := utf16.Encode([]rune(password))
	in := make([]byte, len(u16)*2)
	for i, c := range u16 {
		in[i*2] = byte(c)
		in[i*2+1] = byte(c >> 8)
	}
	inBlob := windows.DataBlob{Size: uint32(len(in)), Data: &in[0]}
	var outBlob windows.DataBlob
	if err := windows.CryptProtectData(&inBlob, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &outBlob); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(outBlob.Data)))
	out := unsafe.Slice(outBlob.Data, int(outBlob.Size))
	return hex.EncodeToString(out), nil
}
