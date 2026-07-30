package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "ops-mate"
	keyringKey     = "master-key"
)

// masterKey 返回 32 字节主密钥。优先 OS keyring；失败时回退到
// 数据目录下 master.key 文件（0600）。
func masterKey(appDir string) ([]byte, error) {
	if os.Getenv("OPS_MATE_TEST_NO_KEYRING") != "1" {
		secret, err := keyring.Get(keyringService, keyringKey)
		if err == nil && len(secret) == 32 {
			return []byte(secret), nil
		}
		k, err := randomKey(32)
		if err != nil {
			return nil, err
		}
		if err := keyring.Set(keyringService, keyringKey, string(k)); err == nil {
			return k, nil
		}
	}
	return fileMasterKey(appDir)
}

func fileMasterKey(appDir string) ([]byte, error) {
	p := filepath.Join(appDir, "master.key")
	b, err := os.ReadFile(p)
	if err == nil && len(b) >= 32 {
		return b[:32], nil
	}
	k, err := randomKey(32)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, k, 0o600); err != nil {
		return nil, fmt.Errorf("write master.key: %w", err)
	}
	return k, nil
}

func randomKey(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}
	return b, nil
}

// encrypt 用 AES-256-GCM 加密。输出格式: nonce(12) || ciphertext。
func encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ct...), nil
}

// decrypt 解密 encrypt 的输出。
func decrypt(key, blob []byte) ([]byte, error) {
	if len(blob) < 13 {
		return nil, errors.New("密文过短")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, ct := blob[:gcm.NonceSize()], blob[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}
