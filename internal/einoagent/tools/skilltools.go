package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/einoagent/history"
	"ops-mate/internal/skill"
	"ops-mate/internal/sshexec"
	convstore "ops-mate/internal/store/conversations"
)

// SkillTools 提供 load_skill / run_skill_script 两个技能工具。
type SkillTools struct {
	sessionID   string
	executor    sshexec.Exec
	emit        func(sessionID, event string, data any)
	convs       *convstore.ConvStore
	holder      *ToolCallHolder
	skillFor    func(name string) (*skill.Skill, error)
	readMD      func(s *skill.Skill) (string, error)
	scriptPath  func(s *skill.Skill, name string) (string, error)
	scriptNames func(s *skill.Skill) []string
}

// NewSkillTools 构造技能工具。convs 为 nil 时不落库（测试简化）。
// skillFor 按名解析技能；其余字段默认从技能目录读取，测试可覆写。
func NewSkillTools(
	sessionID string,
	executor sshexec.Exec,
	emit func(sessionID, event string, data any),
	convs *convstore.ConvStore,
	holder *ToolCallHolder,
	skillFor func(name string) (*skill.Skill, error),
) *SkillTools {
	t := &SkillTools{
		sessionID: sessionID, executor: executor,
		emit: emit, convs: convs, holder: holder, skillFor: skillFor,
	}
	t.readMD = func(s *skill.Skill) (string, error) {
		data, err := os.ReadFile(filepath.Join(s.Dir, skill.SkillMD))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	t.scriptPath = func(s *skill.Skill, name string) (string, error) {
		if name == "" || strings.ContainsAny(name, `/\`) {
			return "", fmt.Errorf("脚本名非法: %q", name)
		}
		p := filepath.Join(s.Dir, "scripts", name)
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("脚本不存在: %s", name)
		}
		return p, nil
	}
	t.scriptNames = func(s *skill.Skill) []string {
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
	return t
}

// Tools 返回两个技能工具（load_skill / run_skill_script）。
func (t *SkillTools) Tools() []einotool.BaseTool {
	return []einotool.BaseTool{
		&skillTool{st: t, kind: "load"},
		&skillTool{st: t, kind: "run"},
	}
}

// skillTool 把 SkillTools 的两种行为包装为 eino BaseTool。
type skillTool struct {
	st   *SkillTools
	kind string // "load" | "run"
}

func (x *skillTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	if x.kind == "load" {
		return &schema.ToolInfo{
			Name: "load_skill",
			Desc: "加载运维技能的详细指南（SKILL.md 全文与脚本列表）。技能名参考系统提示词中的「已安装运维技能」目录；按指南执行排查/操作。",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"skillName": {Type: schema.String, Desc: "技能名称（目录中的 name）", Required: true},
			}),
		}, nil
	}
	return &schema.ToolInfo{
		Name: "run_skill_script",
		Desc: "在目标资产上执行某运维技能 scripts/ 目录下的脚本（自动上传到临时目录后执行，由脚本 shebang 决定解释器）。仅当技能自带可执行脚本时使用；脚本会先展示给用户审批，批准后才执行。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"skillName": {Type: schema.String, Desc: "技能名称", Required: true},
			"script":    {Type: schema.String, Desc: "scripts/ 下的脚本文件名", Required: true},
			"args":      {Type: schema.String, Desc: "传给脚本的命令行参数"},
			"why":       {Type: schema.String, Desc: "执行原因"},
		}),
	}, nil
}

func (x *skillTool) InvokableRun(ctx context.Context, argsJSON string, _ ...einotool.Option) (string, error) {
	if x.kind == "load" {
		return x.st.loadSkill(ctx, argsJSON)
	}
	return x.st.runSkillScript(ctx, argsJSON)
}

// loadSkill 读取 SKILL.md 全文 + 脚本列表返回模型。
func (t *SkillTools) loadSkill(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		SkillName string `json:"skillName"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "参数解析失败：" + err.Error(), nil
	}
	if args.SkillName == "" {
		return "缺少 skillName 参数", nil
	}
	s, err := t.skillFor(args.SkillName)
	if err != nil {
		return fmt.Sprintf("技能 %q 不存在或未启用", args.SkillName), nil
	}
	md, err := t.readMD(s)
	if err != nil {
		return fmt.Sprintf("读取技能 %q 内容失败：%v", args.SkillName, err), nil
	}
	var sb strings.Builder
	sb.WriteString("技能 ")
	sb.WriteString(s.Name)
	if s.Description != "" {
		sb.WriteString("（" + s.Description + "）")
	}
	sb.WriteString(" 指南：\n")
	sb.WriteString(md)
	if names := t.scriptNames(s); len(names) > 0 {
		sb.WriteString("\n\n可用脚本（用 run_skill_script 执行）：" + strings.Join(names, ", "))
	}
	content := truncateForModel(sb.String())
	t.saveToolMessage("load_skill", content, "load", toolMeta{})
	return content, nil
}

// scriptArgs run_skill_script 的参数。
type scriptArgs struct {
	SkillName string `json:"skillName"`
	Script    string `json:"script"`
	Args      string `json:"args"`
}

// runSkillScript 审批后上传脚本到目标资产执行。
func (t *SkillTools) runSkillScript(ctx context.Context, argsJSON string) (string, error) {
	var args scriptArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "参数解析失败：" + err.Error(), nil
	}
	if args.SkillName == "" || args.Script == "" {
		return "缺少 skillName 或 script 参数", nil
	}
	s, err := t.skillFor(args.SkillName)
	if err != nil {
		return fmt.Sprintf("技能 %q 不存在或未启用", args.SkillName), nil
	}
	scriptPath, err := t.scriptPath(s, args.Script)
	if err != nil {
		return err.Error(), nil
	}

	display := fmt.Sprintf("run_skill_script(%s, %s %s)", args.SkillName, args.Script, args.Args)
	info := commandInfo{Command: display, Why: "", Risk: "high", AssessedRisk: "script"}

	wasInterrupted, _, _ := einotool.GetInterruptState[commandInfo](ctx)
	isTarget, hasData, data := einotool.GetResumeContext[string](ctx)
	if wasInterrupted && isTarget && hasData {
		if data == "rejected" {
			content := "用户拒绝了这次脚本执行，未执行。请不要重复提议同一脚本，换其他方案或询问用户。"
			t.saveToolMessage("run_skill_script", content, "rejected", toolMeta{Command: display, Status: "rejected"})
			return content, nil
		}
		return t.executeScript(ctx, s, scriptPath, args, display)
	}

	if t.emit != nil {
		t.emit(t.sessionID, "ai:command", info)
	}
	return "", einotool.Interrupt(ctx, info)
}

// executeScript 上传并执行技能脚本，输出回灌模型。
func (t *SkillTools) executeScript(ctx context.Context, s *skill.Skill, scriptPath string, args scriptArgs, display string) (string, error) {
	if t.executor == nil {
		return "执行器不可用（SSH 未连接）", nil
	}
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Sprintf("读取脚本失败：%v", err), nil
	}
	if t.emit != nil {
		t.emit(t.sessionID, "run:start", map[string]any{"command": display})
	}
	tmpDir := "/tmp/ops-mate-skill-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	remoteScript := tmpDir + "/" + args.Script
	b64 := base64.StdEncoding.EncodeToString(content)
	writeCmd := fmt.Sprintf("mkdir -p %s && printf '%%s' '%s' | base64 -d > %s && chmod +x %s",
		tmpDir, b64, remoteScript, remoteScript)
	runCmd := strings.TrimSpace(remoteScript + " " + args.Args)

	execCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if out, code, err := runCommand(execCtx, t.executor, writeCmd, nil); err != nil || code != 0 {
		ret := fmt.Sprintf("上传脚本失败（exit=%d）: %s", code, truncateForDisplay(out))
		t.saveToolMessage("run_skill_script", ret, "approved", toolMeta{Command: display, ExitCode: code, Status: "approved"})
		t.emitRunResult(display, out, code, err)
		return ret, nil
	}
	out, code, err := runCommand(execCtx, t.executor, runCmd, nil)
	var ret string
	switch {
	case code != 0:
		ret = fmt.Sprintf("脚本执行失败（exit=%d）\n%s", code, truncateForDisplay(out))
	case err != nil:
		ret = fmt.Sprintf("脚本执行出错：%v", err)
	default:
		ret = truncateForModel(out)
	}
	t.saveToolMessage("run_skill_script", ret, "approved", toolMeta{Command: display, ExitCode: code, Status: "approved"})
	t.emitRunResult(display, out, code, err)
	return ret, nil
}

func (t *SkillTools) emitRunResult(display, output string, exitCode int, execErr error) {
	if t.emit == nil {
		return
	}
	errStr := ""
	if execErr != nil {
		errStr = execErr.Error()
	}
	t.emit(t.sessionID, "run:result", map[string]any{
		"command": display, "output": truncateForDisplay(output),
		"exitCode": exitCode, "error": errStr, "cancelled": false,
	})
}

// saveToolMessage 落库工具消息（配对 tool_call_id），toolName 为 load_skill / run_skill_script，
// approvalStatus 为 "load"/"approved"/"rejected"。
func (t *SkillTools) saveToolMessage(toolName, content, approvalStatus string, meta toolMeta) {
	toolCallID := ""
	if t.holder != nil {
		if tc := t.holder.Take(); tc != nil {
			toolCallID = tc.ID
		}
	}
	if t.convs == nil {
		return
	}
	metaJSON, _ := json.Marshal(meta)
	if err := t.convs.SaveMessage(convstore.Message{
		SessionID:      t.sessionID,
		Role:           history.RoleTool,
		Content:        content,
		ToolCallID:     toolCallID,
		ToolName:       toolName,
		ApprovalStatus: approvalStatus,
		ToolResult:     string(metaJSON),
	}); err != nil {
		// 落库失败不阻断主流程
	}
}
