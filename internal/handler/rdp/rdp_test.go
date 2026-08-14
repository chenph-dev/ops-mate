package rdp

import (
	"strings"
	"testing"
)

func TestRdpContent(t *testing.T) {
	c := rdpContent("10.0.0.5", 3389, "Administrator", "")
	if !strings.Contains(c, "full address:s:10.0.0.5:3389") {
		t.Errorf("rdp 内容缺地址: %q", c)
	}
	if !strings.Contains(c, "username:s:Administrator") {
		t.Errorf("rdp 内容缺用户名: %q", c)
	}
	if strings.Contains(c, "password 51:b:") {
		t.Errorf("无密码时不应写入 password 行: %q", c)
	}
	if !strings.Contains(c, "prompt for credentials:i:1") {
		t.Errorf("无密码时应强制提示凭据: %q", c)
	}
}

func TestRdpContentWithPassword(t *testing.T) {
	c := rdpContent("10.0.0.5", 3389, "Administrator", "deadbeef")
	if !strings.Contains(c, "password 51:b:deadbeef") {
		t.Errorf("应写入加密密码: %q", c)
	}
	if !strings.Contains(c, "prompt for credentials:i:0") {
		t.Errorf("有密码时应自动登录: %q", c)
	}
}

func TestRdpPortOrDefault(t *testing.T) {
	if rdpPortOrDefault(0) != 3389 {
		t.Error("空端口应归一为 3389")
	}
	if rdpPortOrDefault(3390) != 3390 {
		t.Error("非空端口应原样返回")
	}
}

func TestRdpContentSanitizesCRLF(t *testing.T) {
	c := rdpContent("10.0.0.5\r\nalternate shell:s:cmd.exe", 3389, "Admin\r\nx", "")
	if strings.Contains(c, "\r") {
		t.Errorf("rdp 内容不应含 CR: %q", c)
	}
	if strings.Contains(c, "\nalternate shell") {
		t.Errorf("注入的指令应被清除: %q", c)
	}
}
