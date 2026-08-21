// Package hoststore 管理 SSH 资产与目录的树形结构。
package hoststore

import (
	"encoding/json"
	"fmt"
	"time"

	"ops-mate/internal/store"
	"ops-mate/internal/store/crypto"
)

// Host GORM 模型，对应 hosts 表。
// node_type='folder' 时 addr/port/user 为空；node_type='host' 时为实际资产。
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
	AutoApprove   string  `gorm:"column:auto_approve"`
	Protocol      string  `gorm:"column:protocol"`
	RdpPort       int     `gorm:"column:rdp_port"`
	ParamsJSON    string  `gorm:"column:params_json"`
	CreatedAt     int64   `gorm:"column:created_at"`
}

func (Host) TableName() string { return "hosts" }

// HostInput 资产录入数据。Secret 为密码或私钥 PEM 明文。
type HostInput struct {
	Name        string `json:"name"`
	ParentID    string `json:"parentId"` // 父目录 ID，空串表示根级
	Addr        string `json:"addr"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	AuthType    string `json:"authType"`
	Secret      string `json:"secret"`
	AutoApprove string `json:"autoApprove"` // inherit | on | off
	Protocol    string         `json:"protocol"`
	RdpPort     int            `json:"rdpPort"`
	Params      map[string]any `json:"params,omitempty"`
}

// HostMeta 资产列表项（不含凭据）。
type HostMeta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	NodeType    string `json:"nodeType"`
	ParentID    string `json:"parentId"`
	Addr        string `json:"addr"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	AuthType    string `json:"authType"`
	AutoApprove string `json:"autoApprove"`
	Protocol    string `json:"protocol"`
	RdpPort     int            `json:"rdpPort"`
	Params      map[string]any `json:"params"`
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
	Addr        string `json:"addr,omitempty"`
	Port        int    `json:"port,omitempty"`
	User        string `json:"user,omitempty"`
	AuthType    string `json:"authType,omitempty"`
	AutoApprove string `json:"autoApprove,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	RdpPort     int            `json:"rdpPort,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
}

// HostsStore 提供资产/目录管理操作。
type HostsStore struct {
	app *store.DB
}

// NewHostsStore 构造 HostsStore。
func NewHostsStore(app *store.DB) *HostsStore {
	return &HostsStore{app: app}
}

// SaveHost 保存资产（node_type='host'）。
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
		AutoApprove: autoApproveOrDefault(in.AutoApprove),
		Protocol:    normalizeProtocol(in),
		RdpPort:     rdpPortOrDefault(in.RdpPort),
		ParamsJSON:  paramsJSON(in),
		CreatedAt:   time.Now().Unix(),
	}).Error
	if err != nil {
		return "", fmt.Errorf("insert host: %w", err)
	}
	return id, nil
}

// UpdateHost 更新资产信息（节点编辑）。secret 为空则保留原凭据，非空则重新加密。
func (s *HostsStore) UpdateHost(id string, in HostInput) error {
	// 编辑未提供 params 时保留已存 params_json，避免静默清空
	// database/filePath/hostKeyFingerprint（SSH TOFU 指纹）等已有参数。
	paramsJSON := paramsJSON(in)
	if len(in.Params) == 0 {
		var existing Host
		if err := s.app.GORM().First(&existing, "id = ?", id).Error; err == nil {
			paramsJSON = existing.ParamsJSON
		}
	}
	updates := map[string]any{
		"name": in.Name, "addr": in.Addr, "port": in.Port,
		"user": in.User, "auth_type": in.AuthType,
		"auto_approve": autoApproveOrDefault(in.AutoApprove),
		"protocol":     normalizeProtocol(in),
		"rdp_port":     rdpPortOrDefault(in.RdpPort),
		"params_json":  paramsJSON,
	}
	if in.Secret != "" {
		enc, err := s.app.Encrypt([]byte(in.Secret))
		if err != nil {
			return fmt.Errorf("encrypt auth: %w", err)
		}
		updates["auth_encrypted"] = enc
	}
	return s.app.GORM().Model(&Host{}).Where("id = ?", id).Updates(updates).Error
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
		params := metaParams(*h)
		nodeMap[h.ID] = &TreeNode{
			ID: h.ID, Key: h.ID, Name: h.Name, NodeType: h.NodeType,
			ParentID: strPtrVal(h.ParentID),
			Addr:     h.Addr, Port: h.Port, User: h.User, AuthType: h.AuthType,
			AutoApprove: h.AutoApprove,
			Protocol:    h.Protocol, RdpPort: h.RdpPort,
			Params:      params,
		}
	}

	// 构建树：第一遍收集父 -> 子节点指针关系（不拷贝值，避免孙节点丢失）。
	childrenOf := make(map[string][]*TreeNode)
	rootsPtr := make([]*TreeNode, 0)
	for i := range all {
		h := &all[i]
		node := nodeMap[h.ID]
		if h.ParentID == nil || *h.ParentID == "" {
			rootsPtr = append(rootsPtr, node)
		} else if _, ok := nodeMap[*h.ParentID]; ok {
			childrenOf[*h.ParentID] = append(childrenOf[*h.ParentID], node)
		}
	}

	// 第二遍：递归装配子树（先填孙节点再值拷贝，保证嵌套层级完整）。
	var assemble func(n *TreeNode) TreeNode
	assemble = func(n *TreeNode) TreeNode {
		out := *n
		out.Children = make([]TreeNode, 0, len(childrenOf[n.ID]))
		for _, c := range childrenOf[n.ID] {
			out.Children = append(out.Children, assemble(c))
		}
		return out
	}

	// 解引用指针切片为值切片
	roots := make([]TreeNode, len(rootsPtr))
	for i, p := range rootsPtr {
		roots[i] = assemble(p)
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

// RenameNode 重命名节点（目录或资产通用）。
func (s *HostsStore) RenameNode(nodeID, name string) error {
	return s.app.GORM().Model(&Host{}).Where("id = ?", nodeID).
		Update("name", name).Error
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

// HostMetaByID 取单资产元数据。
func (s *HostsStore) HostMetaByID(id string) (*HostMeta, error) {
	var h Host
	if err := s.app.GORM().First(&h, "id = ?", id).Error; err != nil {
		return nil, err
	}
	meta := hostToMeta(h)
	return &meta, nil
}

// GetAutoApprove 返回资产自动放行覆盖（"inherit"/"on"/"off"）。
func (s *HostsStore) GetAutoApprove(id string) (string, error) {
	var h Host
	if err := s.app.GORM().First(&h, "id = ?", id).Error; err != nil {
		return "", err
	}
	return autoApproveOrDefault(h.AutoApprove), nil
}

// HostKeyFingerprint 返回资产已信任的 SSH 主机密钥 SHA256 指纹；未建立信任返回空串。
func (s *HostsStore) HostKeyFingerprint(id string) (string, error) {
	var h Host
	if err := s.app.GORM().First(&h, "id = ?", id).Error; err != nil {
		return "", err
	}
	if v, ok := metaParams(h)["hostKeyFingerprint"].(string); ok {
		return v, nil
	}
	return "", nil
}

// SaveHostKeyFingerprint 持久化资产首次连接的 SSH 主机密钥指纹（TOFU 信任）。
// 写入资产 params，不触发协议/凭据等字段变更。
func (s *HostsStore) SaveHostKeyFingerprint(id, fingerprint string) error {
	var h Host
	if err := s.app.GORM().First(&h, "id = ?", id).Error; err != nil {
		return err
	}
	params := metaParams(h)
	params["hostKeyFingerprint"] = fingerprint
	b, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}
	return s.app.GORM().Model(&Host{}).Where("id = ?", id).
		Update("params_json", string(b)).Error
}

func strPtrVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// autoApproveOrDefault 空值归一为 "inherit"（避免零值覆盖列默认）。
func autoApproveOrDefault(v string) string {
	if v == "" {
		return "inherit"
	}
	return v
}

// rdpPortOrDefault normalizes empty RDP port to default 3389.
func rdpPortOrDefault(v int) int {
	if v == 0 {
		return 3389
	}
	return v
}

// normalizeProtocol 连接类型单层化：空 → ssh。
func normalizeProtocol(in HostInput) string {
	if in.Protocol == "" {
		return "ssh"
	}
	return in.Protocol
}

// paramsJSON 序列化资产专属参数。
func paramsJSON(in HostInput) string {
	params := in.Params
	if params == nil {
		params = map[string]any{}
	}
	b, err := json.Marshal(params)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// metaParams 解析 params_json 为 map。
func metaParams(h Host) map[string]any {
	m := map[string]any{}
	_ = json.Unmarshal([]byte(h.ParamsJSON), &m)
	return m
}

// hostToMeta 把 Host 行转为 HostMeta。
func hostToMeta(h Host) HostMeta {
	return HostMeta{
		ID: h.ID, Name: h.Name, NodeType: h.NodeType,
		ParentID: strPtrVal(h.ParentID),
		Addr:     h.Addr, Port: h.Port, User: h.User, AuthType: h.AuthType,
		AutoApprove: h.AutoApprove,
		Protocol:    h.Protocol, RdpPort: h.RdpPort,
		Params:      metaParams(h),
	}
}
