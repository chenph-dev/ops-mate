// Package skill 提供运维技能的管理（ZIP 校验/解压、SKILL.md 解析、磁盘读写）。
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	skillsstore "ops-mate/internal/store/skills"
)

// Skill 技能信息（含磁盘目录）。
type Skill struct {
	Name        string
	Title       string
	Description string
	Enabled     bool
	Dir         string
	CreatedAt   int64
}

// Manager 技能管理：DB 元数据 + 磁盘文件。
type Manager struct {
	store *skillsstore.SkillStore
	root  string // <DataDir>/skills
}

// NewManager 构造 Manager。root 为技能文件根目录（应已创建）。
func NewManager(store *skillsstore.SkillStore, root string) *Manager {
	return &Manager{store: store, root: root}
}

// Install 校验/解压/落盘/入库。返回安装后的技能信息。
func (m *Manager) Install(zipBytes []byte) (*Skill, error) {
	// 先解压到临时目录解析 frontmatter，避免污染正式目录
	tmp, err := os.MkdirTemp(m.root, "install-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	skillDir, err := extractZip(zipBytes, tmp)
	if err != nil {
		return nil, err
	}
	md, err := os.ReadFile(filepath.Join(skillDir, SkillMD))
	if err != nil {
		return nil, fmt.Errorf("缺少 %s: %w", SkillMD, err)
	}
	meta, err := parseFrontmatter(md)
	if err != nil {
		return nil, err
	}
	if _, err := m.store.Get(meta.Name); err == nil {
		return nil, fmt.Errorf("技能 %q 已存在", meta.Name)
	}

	id, err := m.store.Create(meta.Name, meta.Name, meta.Description)
	if err != nil {
		return nil, err
	}
	dst := filepath.Join(m.root, id)
	if err := copyTree(skillDir, dst); err != nil {
		_ = m.store.Delete(meta.Name)
		return nil, err
	}
	return &Skill{
		Name: meta.Name, Title: meta.Name, Description: meta.Description,
		Enabled: true, Dir: dst,
	}, nil}

// List 返回全部技能（按 name 升序）。
func (m *Manager) List() ([]*Skill, error) {
	rows, err := m.store.List()
	if err != nil {
		return nil, err
	}
	out := make([]*Skill, 0, len(rows))
	for i := range rows {
		out = append(out, m.toSkill(&rows[i]))
	}
	return out, nil
}

// Lookup 按 name 解析技能（含磁盘目录）。
func (m *Manager) Lookup(name string) (*Skill, error) {
	r, err := m.store.Get(name)
	if err != nil {
		return nil, err
	}
	return m.toSkill(r), nil
}

// SetEnabled 更新技能启用状态。
func (m *Manager) SetEnabled(name string, enabled bool) error {
	return m.store.SetEnabled(name, enabled)
}

// Delete 删除技能（DB 记录 + 磁盘目录）。
func (m *Manager) Delete(name string) error {
	r, err := m.store.Get(name)
	if err != nil {
		return err
	}
	if err := m.store.Delete(name); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(m.root, r.ID))
}

// Catalog 返回已启用技能的目录文本（name: description），无则空串。
func (m *Manager) Catalog() string {
	rows, err := m.store.ListEnabled()
	if err != nil {
		return ""
	}
	var sb strings.Builder
	for _, r := range rows {
		line := "- " + r.Name
		if r.Description != "" {
			line += ": " + r.Description
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// ReadMarkdown 返回技能 SKILL.md 全文。
func (m *Manager) ReadMarkdown(s *Skill) (string, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, SkillMD))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ScriptPath 校验并返回技能脚本绝对路径；脚本名不得含路径分隔符。
func (m *Manager) ScriptPath(s *Skill, script string) (string, error) {
	if script == "" || strings.ContainsAny(script, `/\`) {
		return "", fmt.Errorf("脚本名非法: %q", script)
	}
	p := filepath.Join(s.Dir, "scripts", script)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("脚本不存在: %s", script)
	}
	return p, nil
}

// ScriptNames 返回技能 scripts/ 目录下的文件名列表（无目录返回 nil）。
func (m *Manager) ScriptNames(s *Skill) []string {
	entries, err := os.ReadDir(filepath.Join(s.Dir, "scripts"))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

func (m *Manager) toSkill(r *skillsstore.Skill) *Skill {
	return &Skill{
		Name: r.Name, Title: r.Title, Description: r.Description,
		Enabled: r.Enabled, Dir: filepath.Join(m.root, r.ID), CreatedAt: r.CreatedAt,
	}
}

// copyTree 递归拷贝目录内容。
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o755)
	})
}
