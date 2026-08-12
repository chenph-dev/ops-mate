package configstore

import (
	"strings"

	"gorm.io/gorm"

	"ops-mate/internal/store"
)

// policyModel 审批策略单行表（id=1），对应 approval_policy 表。
type policyModel struct {
	ID                int    `gorm:"column:id;primaryKey"`
	EnableAuto        bool   `gorm:"column:enable_auto"`
	ReadOnlyWhitelist string `gorm:"column:readonly_whitelist"`
}

func (policyModel) TableName() string { return "approval_policy" }

// ApprovalPolicy 审批策略 DTO。ReadOnlyList 为空表示使用内置默认只读白名单。
type ApprovalPolicy struct {
	EnableAuto   bool     `json:"enableAuto"`
	ReadOnlyList []string `json:"readOnlyList"`
}

// PolicyStore 提供审批策略读写。
type PolicyStore struct {
	app *store.DB
}

// NewPolicyStore 构造 PolicyStore。
func NewPolicyStore(app *store.DB) *PolicyStore {
	return &PolicyStore{app: app}
}

// GetApprovalPolicy 读取策略；无记录时返回默认（开启自动放行 + 空白名单 → 内置默认白名单）。
func (s *PolicyStore) GetApprovalPolicy() (ApprovalPolicy, error) {
	var m policyModel
	err := s.app.GORM().First(&m, 1).Error
	if err == gorm.ErrRecordNotFound {
		return ApprovalPolicy{EnableAuto: true}, nil
	}
	if err != nil {
		return ApprovalPolicy{}, err
	}
	return ApprovalPolicy{
		EnableAuto:   m.EnableAuto,
		ReadOnlyList: splitList(m.ReadOnlyWhitelist),
	}, nil
}

// SaveApprovalPolicy 保存策略（覆盖单行 id=1）。
func (s *PolicyStore) SaveApprovalPolicy(p ApprovalPolicy) error {
	return s.app.GORM().Save(&policyModel{
		ID: 1, EnableAuto: p.EnableAuto,
		ReadOnlyWhitelist: strings.Join(p.ReadOnlyList, ","),
	}).Error
}

// splitList 逗号分隔字符串 → 非空切片（空串返回 nil，调用方回退内置默认）。
func splitList(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
