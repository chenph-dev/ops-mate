// Package hoststore 管理 SSH 主机的增删查改与凭据加密存储。
package hoststore

import (
	"fmt"
	"time"

	"ops-mate/internal/store"
	"ops-mate/internal/store/crypto"
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

// HostsStore 提供主机管理操作。
type HostsStore struct {
	app *store.DB
}

// NewHostsStore 构造 HostsStore。
func NewHostsStore(app *store.DB) *HostsStore {
	return &HostsStore{app: app}
}

func (s *HostsStore) SaveHost(in HostInput) (string, error) {
	id := crypto.NewID()
	enc, err := s.app.Encrypt([]byte(in.Secret))
	if err != nil {
		return "", fmt.Errorf("encrypt auth: %w", err)
	}
	_, err = s.app.DB().Exec(
		`INSERT INTO hosts(id,name,addr,port,user,auth_encrypted,auth_type,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		id, in.Name, in.Addr, in.Port, in.User, enc, in.AuthType, time.Now().Unix(),
	)
	if err != nil {
		return "", fmt.Errorf("insert host: %w", err)
	}
	return id, nil
}

func (s *HostsStore) ListHosts() ([]HostMeta, error) {
	rows, err := s.app.DB().Query(`SELECT id,name,addr,port,user,auth_type FROM hosts ORDER BY name`)
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
func (s *HostsStore) GetHostSecret(id string) (secret, authType string, err error) {
	var blob []byte
	err = s.app.DB().QueryRow(`SELECT auth_encrypted, auth_type FROM hosts WHERE id=?`, id).Scan(&blob, &authType)
	if err != nil {
		return "", "", err
	}
	pt, err := s.app.Decrypt(blob)
	if err != nil {
		return "", "", fmt.Errorf("decrypt auth: %w", err)
	}
	return string(pt), authType, nil
}

// HostMetaByID 取单主机元数据。
func (s *HostsStore) HostMetaByID(id string) (*HostMeta, error) {
	var h HostMeta
	err := s.app.DB().QueryRow(`SELECT id,name,addr,port,user,auth_type FROM hosts WHERE id=?`, id).
		Scan(&h.ID, &h.Name, &h.Addr, &h.Port, &h.User, &h.AuthType)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (s *HostsStore) DeleteHost(id string) error {
	_, err := s.app.DB().Exec(`DELETE FROM hosts WHERE id=?`, id)
	return err
}
