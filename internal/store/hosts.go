package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// HostInput 主机录入数据。Secret 为密码或私钥 PEM 明文。
type HostInput struct {
	Name     string `json:"name"`
	Addr     string `json:"addr"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	AuthType string `json:"authType"`
	Secret   string `json:"secret"`
}

// HostMeta 主机列表项（不含凭据）。
type HostMeta struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Addr     string `json:"addr"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	AuthType string `json:"authType"`
}

func (s *Store) SaveHost(in HostInput) (string, error) {
	id := newID()
	enc, err := encrypt(s.key, []byte(in.Secret))
	if err != nil {
		return "", fmt.Errorf("encrypt auth: %w", err)
	}
	_, err = s.DB.Exec(
		`INSERT INTO hosts(id,name,addr,port,user,auth_encrypted,auth_type,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		id, in.Name, in.Addr, in.Port, in.User, enc, in.AuthType, time.Now().Unix(),
	)
	if err != nil {
		return "", fmt.Errorf("insert host: %w", err)
	}
	return id, nil
}

func (s *Store) ListHosts() ([]HostMeta, error) {
	rows, err := s.DB.Query(`SELECT id,name,addr,port,user,auth_type FROM hosts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HostMeta
	for rows.Next() {
		var h HostMeta
		if err := rows.Scan(&h.ID, &h.Name, &h.Addr, &h.Port, &h.User, &h.AuthType); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetHostSecret 返回解密后的凭据与类型。
func (s *Store) GetHostSecret(id string) (secret, authType string, err error) {
	var blob []byte
	err = s.DB.QueryRow(`SELECT auth_encrypted, auth_type FROM hosts WHERE id=?`, id).Scan(&blob, &authType)
	if err != nil {
		return "", "", err
	}
	pt, err := decrypt(s.key, blob)
	if err != nil {
		return "", "", fmt.Errorf("decrypt auth: %w", err)
	}
	return string(pt), authType, nil
}

// HostMetaByID 取单主机元数据。
func (s *Store) HostMetaByID(id string) (*HostMeta, error) {
	var h HostMeta
	err := s.DB.QueryRow(`SELECT id,name,addr,port,user,auth_type FROM hosts WHERE id=?`, id).
		Scan(&h.ID, &h.Name, &h.Addr, &h.Port, &h.User, &h.AuthType)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (s *Store) DeleteHost(id string) error {
	_, err := s.DB.Exec(`DELETE FROM hosts WHERE id=?`, id)
	return err
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
