//go:build windows

package handler

import (
	"encoding/hex"
	"testing"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestProtectPasswordRoundtrip(t *testing.T) {
	const secret = "P@ssw0rd!"
	hexed, err := protectPassword(secret)
	if err != nil {
		t.Fatalf("protectPassword: %v", err)
	}
	if hexed == "" {
		t.Fatal("加密结果为空")
	}
	raw, err := hex.DecodeString(hexed)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	inBlob := windows.DataBlob{Size: uint32(len(raw)), Data: &raw[0]}
	var outBlob windows.DataBlob
	if err := windows.CryptUnprotectData(&inBlob, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &outBlob); err != nil {
		t.Fatalf("CryptUnprotectData: %v", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(outBlob.Data)))
	out := unsafe.Slice(outBlob.Data, int(outBlob.Size))
	u16 := make([]uint16, len(out)/2)
	for i := range u16 {
		u16[i] = uint16(out[i*2]) | uint16(out[i*2+1])<<8
	}
	if got := string(utf16.Decode(u16)); got != secret {
		t.Fatalf("解密结果不匹配: got %q want %q", got, secret)
	}
}
