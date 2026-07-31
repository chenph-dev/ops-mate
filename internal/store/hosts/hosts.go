// Package hoststore 管理 SSH 主机的增删查改与凭据加密存储。
package hoststore

import (
	"fmt"
	"time"

	"ops-mate/internal/store"
	"ops-mate/internal/store/crypto"
)

// Host GORM 模型，对应 hosts 表。
type Host struct {
	ID            string `gorm:"column:id;primaryKey"`
	Name          string `gorm:"column:name"`
	Addr          string `gorm:"column:addr"`
	Port          int    `gorm:"column:port"`
	User          string `gorm:"column:user"`
	AuthEncrypted []byte `gorm:"column:auth_encrypted"`
	AuthType      string `gorm:"column:auth_type"`
	CreatedAt     int64  `gorm:"column:created_at"`
}

func (Host) TableName() string { return "hosts" }

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
	err = s.app.GORM().Create(&Host{
		ID: id, Name: in.Name, Addr: in.Addr, Port: in.Port,
		User: in.User, AuthEncrypted: enc, AuthType: in.AuthType,
		CreatedAt: time.Now().Unix(),
	}).Error
	if err != nil {
		return "", fmt.Errorf("insert host: %w", err)
	}
	return id, nil
}

func (s *HostsStore) ListHosts() ([]HostMeta, error) {
	var hosts []Host
	if err := s.app.GORM().Order("name").Find(&hosts).Error; err != nil {
		return nil, err
	}
	out := make([]HostMeta, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, HostMeta{
			ID: h.ID, Name: h.Name, Addr: h.Addr,
			Port: h.Port, User: h.User, AuthType: h.AuthType,
		})
	}
	return out, nil
}

// GetHostSecret 返回解密后的凭据与类型。
func (s *HostsStore) GetHostSecret(id string) (secret, authType string, err error) {
	var h Host
	err = s.app.GORM().First(&h, "id = ?", id).Error
	if err != nil {
		return "", "", err
	}
	pt, err := s.app.Decrypt(h.AuthEncrypted)
	if err != nil {
		return "", "", fmt.Errorf("decrypt auth: %w", err)
	}
	return string(pt), h.AuthType, nil
}

// HostMetaByID 取单主机元数据。
func (s *HostsStore) HostMetaByID(id string) (*HostMeta, error) {
	var h Host
	if err := s.app.GORM().First(&h, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &HostMeta{
		ID: h.ID, Name: h.Name, Addr: h.Addr,
		Port: h.Port, User: h.User, AuthType: h.AuthType,
	}, nil
}

func (s *HostsStore) DeleteHost(id string) error {
	return s.app.GORM().Delete(&Host{}, "id = ?", id).Error
}
