// Package hoststore 管理 SSH 主机与目录的树形结构。
package hoststore

import (
	"fmt"
	"time"

	"ops-mate/internal/store"
	"ops-mate/internal/store/crypto"
)

// Host GORM 模型，对应 hosts 表。
// node_type='folder' 时 addr/port/user 为空；node_type='host' 时为实际主机。
type Host struct {
	ID            string  `gorm:"column:id;primaryKey"`
	Name          string  `gorm:"column:name"`
	NodeType      string  `gorm:"column:node_type"`
	ParentID      *string `gorm:"column:parent_id"`
	Addr          string  `gorm:"column:addr"`
	Port          int     `gorm:"column:port"`
	User          string  `gorm:"column:user"`
	AuthEncrypted []byte  `gorm:"column:auth_encrypted"`
	AuthType      string  `gorm:"column:auth_type"`
	CreatedAt     int64   `gorm:"column:created_at"`
}

func (Host) TableName() string { return "hosts" }

// HostInput 主机录入数据。Secret 为密码或私钥 PEM 明文。
type HostInput struct {
	Name     string `json:"name"`
	ParentID string `json:"parentId"` // 父目录 ID，空串表示根级
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
	NodeType string `json:"nodeType"`
	ParentID string `json:"parentId"`
	Addr     string `json:"addr"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	AuthType string `json:"authType"`
}

// TreeNode 树形节点返回结构。
type TreeNode struct {
	ID       string     `json:"id"`
	Key      string     `json:"key"`
	Name     string     `json:"name"`
	NodeType string     `json:"nodeType"`
	ParentID string     `json:"parentId"`
	Children []TreeNode `json:"children,omitempty"`
	// Host 专属字段
	Addr     string `json:"addr,omitempty"`
	Port     int    `json:"port,omitempty"`
	User     string `json:"user,omitempty"`
	AuthType string `json:"authType,omitempty"`
}

// HostsStore 提供主机/目录管理操作。
type HostsStore struct {
	app *store.DB
}

// NewHostsStore 构造 HostsStore。
func NewHostsStore(app *store.DB) *HostsStore {
	return &HostsStore{app: app}
}

// SaveHost 保存主机（node_type='host'）。
func (s *HostsStore) SaveHost(in HostInput) (string, error) {
	id := crypto.NewID()
	enc, err := s.app.Encrypt([]byte(in.Secret))
	if err != nil {
		return "", fmt.Errorf("encrypt auth: %w", err)
	}
	parentID := in.ParentID
	var pid *string
	if parentID != "" {
		pid = &parentID
	}
	err = s.app.GORM().Create(&Host{
		ID: id, Name: in.Name, NodeType: "host", ParentID: pid,
		Addr: in.Addr, Port: in.Port, User: in.User,
		AuthEncrypted: enc, AuthType: in.AuthType,
		CreatedAt: time.Now().Unix(),
	}).Error
	if err != nil {
		return "", fmt.Errorf("insert host: %w", err)
	}
	return id, nil
}

// CreateFolder 创建目录（node_type='folder'）。
func (s *HostsStore) CreateFolder(name, parentID string) (string, error) {
	id := crypto.NewID()
	var pid *string
	if parentID != "" {
		p := parentID
		pid = &p
	}
	err := s.app.GORM().Create(&Host{
		ID: id, Name: name, NodeType: "folder", ParentID: pid,
		CreatedAt: time.Now().Unix(),
	}).Error
	if err != nil {
		return "", fmt.Errorf("insert folder: %w", err)
	}
	return id, nil
}

// ListHosts 返回所有主机的扁平列表（兼容旧接口）。
func (s *HostsStore) ListHosts() ([]HostMeta, error) {
	var hosts []Host
	if err := s.app.GORM().Where("node_type = 'host'").Order("name").Find(&hosts).Error; err != nil {
		return nil, err
	}
	out := make([]HostMeta, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, HostMeta{
			ID: h.ID, Name: h.Name, NodeType: h.NodeType,
			ParentID: strPtrVal(h.ParentID),
			Addr: h.Addr, Port: h.Port, User: h.User, AuthType: h.AuthType,
		})
	}
	return out, nil
}

// ListTree 返回完整的树形结构。
func (s *HostsStore) ListTree() ([]TreeNode, error) {
	var all []Host
	if err := s.app.GORM().Order("node_type, name").Find(&all).Error; err != nil {
		return nil, err
	}

	// 构建 id -> *TreeNode 映射
	nodeMap := make(map[string]*TreeNode)
	for i := range all {
		h := &all[i]
		nodeMap[h.ID] = &TreeNode{
			ID: h.ID, Key: h.ID, Name: h.Name, NodeType: h.NodeType,
			ParentID: strPtrVal(h.ParentID),
			Addr: h.Addr, Port: h.Port, User: h.User, AuthType: h.AuthType,
		}
	}

	// 构建树：先建立父子关系（通过指针操作 map 中的节点）
	rootsPtr := make([]*TreeNode, 0)
	for i := range all {
		h := &all[i]
		node := nodeMap[h.ID]
		if h.ParentID == nil || *h.ParentID == "" {
			rootsPtr = append(rootsPtr, node)
		} else {
			if parent, ok := nodeMap[*h.ParentID]; ok {
				parent.Children = append(parent.Children, *node)
			}
		}
	}

	// 解引用指针切片为值切片
	roots := make([]TreeNode, len(rootsPtr))
	for i, p := range rootsPtr {
		roots[i] = *p
	}
	return roots, nil
}

// MoveNode 移动节点到新目录。
func (s *HostsStore) MoveNode(nodeID, newParentID string) error {
	var pid *string
	if newParentID != "" {
		pid = &newParentID
	}
	return s.app.GORM().Model(&Host{}).Where("id = ?", nodeID).
		Update("parent_id", pid).Error
}

// DeleteNode 删除节点（级联删除子节点）。
func (s *HostsStore) DeleteNode(nodeID string) error {
	// 先递归删除所有子节点
	var children []Host
	if err := s.app.GORM().Where("parent_id = ?", nodeID).Find(&children).Error; err != nil {
		return err
	}
	for _, c := range children {
		if err := s.DeleteNode(c.ID); err != nil {
			return err
		}
	}
	return s.app.GORM().Delete(&Host{}, "id = ?", nodeID).Error
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
		ID: h.ID, Name: h.Name, NodeType: h.NodeType,
		ParentID: strPtrVal(h.ParentID),
		Addr: h.Addr, Port: h.Port, User: h.User, AuthType: h.AuthType,
	}, nil
}

func strPtrVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
