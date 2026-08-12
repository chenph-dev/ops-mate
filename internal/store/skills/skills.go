// Package skillsstore 管理运维技能元数据（文件落盘在 <DataDir>/skills/<id>/）。
package skillsstore

import (
	"time"

	"ops-mate/internal/store"
	"ops-mate/internal/store/crypto"
)

// Skill 技能元数据（对应 skills 表）。
type Skill struct {
	ID          string `gorm:"column:id;primaryKey"`
	Name        string `gorm:"column:name;uniqueIndex"`
	Title       string `gorm:"column:title"`
	Description string `gorm:"column:description"`
	Enabled     bool   `gorm:"column:enabled"`
	CreatedAt   int64  `gorm:"column:created_at"`
	UpdatedAt   int64  `gorm:"column:updated_at"`
}

func (Skill) TableName() string { return "skills" }

// SkillStore 提供技能元数据 CRUD。
type SkillStore struct {
	app *store.DB
}

// NewSkillStore 构造 SkillStore。
func NewSkillStore(app *store.DB) *SkillStore {
	return &SkillStore{app: app}
}

// Create 插入技能记录（默认启用）；name 冲突返回错误。
func (s *SkillStore) Create(name, title, description string) (string, error) {
	id := crypto.NewID()
	m := Skill{
		ID: id, Name: name, Title: title, Description: description,
		Enabled: true, CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(),
	}
	if err := s.app.GORM().Create(&m).Error; err != nil {
		return "", err
	}
	return id, nil
}

// Get 按 name 取技能；不存在返回 gorm.ErrRecordNotFound。
func (s *SkillStore) Get(name string) (*Skill, error) {
	var m Skill
	if err := s.app.GORM().Where("name = ?", name).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByID 按 id 取技能（删除文件时用）。
func (s *SkillStore) GetByID(id string) (*Skill, error) {
	var m Skill
	if err := s.app.GORM().First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// List 按 name 升序返回全部技能。
func (s *SkillStore) List() ([]Skill, error) {
	var out []Skill
	if err := s.app.GORM().Order("name").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// ListEnabled 返回已启用技能（按 name 升序）。
func (s *SkillStore) ListEnabled() ([]Skill, error) {
	var out []Skill
	if err := s.app.GORM().Where("enabled = ?", true).Order("name").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// SetEnabled 更新启用状态与时间戳。
func (s *SkillStore) SetEnabled(name string, enabled bool) error {
	return s.app.GORM().Model(&Skill{}).Where("name = ?", name).
		Update("enabled", enabled).Update("updated_at", time.Now().Unix()).Error
}

// Delete 按 name 删除技能记录。
func (s *SkillStore) Delete(name string) error {
	return s.app.GORM().Delete(&Skill{}, "name = ?", name).Error
}
