package handler

import (
	"os"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ops-mate/internal/skill"
)

// SkillInfo 技能列表 DTO（供前端展示）。
type SkillInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   int64  `json:"createdAt"`
}

// SkillsHandler 处理运维技能维护的前端调用。
type SkillsHandler struct {
	mgr      *skill.Manager
	onChange func() // 上传/启停/删除后回调（InvalidateConfig 重建 Graph）
}

// NewSkillsHandler 构造 SkillsHandler。onChange 可为 nil。
func NewSkillsHandler(mgr *skill.Manager, onChange func()) *SkillsHandler {
	return &SkillsHandler{mgr: mgr, onChange: onChange}
}

// InstallSkill 打开文件对话框选择技能 ZIP，校验安装，返回技能名。
// 用户取消时返回空串、nil。
func (h *SkillsHandler) InstallSkill() (string, error) {
	path, err := wailsruntime.OpenFileDialog(Ctx(), wailsruntime.OpenDialogOptions{
		Title: "选择运维技能 ZIP",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "技能包 (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil || path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s, err := h.mgr.Install(data)
	if err != nil {
		return "", err
	}
	if h.onChange != nil {
		h.onChange()
	}
	return s.Name, nil
}

// ListSkills 返回全部技能。
func (h *SkillsHandler) ListSkills() ([]SkillInfo, error) {
	list, err := h.mgr.List()
	if err != nil {
		return nil, err
	}
	out := make([]SkillInfo, 0, len(list))
	for _, s := range list {
		out = append(out, SkillInfo{
			Name: s.Name, Title: s.Title, Description: s.Description,
			Enabled: s.Enabled, CreatedAt: s.CreatedAt,
		})
	}
	return out, nil
}

// ToggleSkill 启用/停用技能并触发热更新。
func (h *SkillsHandler) ToggleSkill(name string, enabled bool) error {
	if err := h.mgr.SetEnabled(name, enabled); err != nil {
		return err
	}
	if h.onChange != nil {
		h.onChange()
	}
	return nil
}

// DeleteSkill 删除技能并触发热更新。
func (h *SkillsHandler) DeleteSkill(name string) error {
	if err := h.mgr.Delete(name); err != nil {
		return err
	}
	if h.onChange != nil {
		h.onChange()
	}
	return nil
}
