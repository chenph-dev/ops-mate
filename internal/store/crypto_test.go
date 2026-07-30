package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMasterKey_PersistsAcrossOpens(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)

	k1, err := masterKey(filepath.Join(dir, "ops-mate"))
	if err != nil {
		t.Fatalf("masterKey 1: %v", err)
	}
	if len(k1) != 32 {
		t.Fatalf("密钥长度 %d，期望 32", len(k1))
	}
	k2, err := masterKey(filepath.Join(dir, "ops-mate"))
	if err != nil {
		t.Fatalf("masterKey 2: %v", err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("同一目录应返回同一主密钥")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := []byte("01234567890123456789012345678901") // 32 字节
	plain := []byte("super-secret-password")
	ct, err := encrypt(key, plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(ct, plain) {
		t.Fatal("密文不应包含明文")
	}
	pt, err := decrypt(key, ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(pt, plain) {
		t.Fatalf("解密结果不匹配")
	}
	ct2, _ := encrypt(key, plain)
	if bytes.Equal(ct, ct2) {
		t.Fatal("相同明文应产生不同密文（随机 nonce）")
	}
}

func TestMasterKey_FallsBackToFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	os.Setenv("OPS_MATE_TEST_NO_KEYRING", "1")
	defer os.Unsetenv("OPS_MATE_TEST_NO_KEYRING")

	k, err := masterKey(filepath.Join(dir, "ops-mate"))
	if err != nil {
		t.Fatalf("文件回退失败: %v", err)
	}
	if len(k) != 32 {
		t.Fatalf("密钥长度 %d", len(k))
	}
	_ = filepath.Separator // 占位避免未用导入
}
