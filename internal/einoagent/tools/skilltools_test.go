package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/einoagent/testutil"
	"ops-mate/internal/skill"
	"ops-mate/internal/sshexec"
)

func skillWithScripts(t *testing.T, md string, scripts map[string]string) *skill.Skill {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range scripts {
		p := filepath.Join(dir, "scripts", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &skill.Skill{Name: "disk-analysis", Description: "分析磁盘", Enabled: true, Dir: dir}
}

func TestSkillTools_LoadSkillReturnsMarkdown(t *testing.T) {
	s := skillWithScripts(t, "---\nname: disk-analysis\n---\n指南正文：df -h 分析\n", map[string]string{"check.sh": "#!/usr/bin/env bash\necho ok\n"})
	st := NewSkillTools("s1", nil, nil, nil, NewToolCallHolder(),
		func(name string) (*skill.Skill, error) { return s, nil })

	got, err := st.loadSkill(context.Background(), `{"skillName":"disk-analysis"}`)
	if err != nil {
		t.Fatalf("loadSkill: %v", err)
	}
	if !strings.Contains(got, "指南正文") {
		t.Errorf("应含 SKILL.md 正文: %q", got)
	}
	if !strings.Contains(got, "check.sh") {
		t.Errorf("应含脚本列表: %q", got)
	}
}

func TestSkillTools_LoadSkillNotFound(t *testing.T) {
	st := NewSkillTools("s1", nil, nil, nil, NewToolCallHolder(),
		func(name string) (*skill.Skill, error) { return nil, fmt.Errorf("not found") })
	got, err := st.loadSkill(context.Background(), `{"skillName":"ghost"}`)
	if err != nil {
		t.Fatalf("loadSkill 不应返回 error: %v", err)
	}
	if !strings.Contains(got, "不存在") {
		t.Errorf("技能不存在应回灌提示: %q", got)
	}
}

func TestSkillTools_RunScriptFirstCallInterrupts(t *testing.T) {
	s := skillWithScripts(t, "---\nname: disk-analysis\n---\n", map[string]string{"check.sh": "#!/bin/bash\necho ok"})
	rec := &testutil.EmitRecorder{}
	st := NewSkillTools("s1", &testutil.FakeExec{}, rec.Emit, nil, NewToolCallHolder(),
		func(name string) (*skill.Skill, error) { return s, nil })

	_, err := st.runSkillScript(context.Background(), `{"skillName":"disk-analysis","script":"check.sh","args":""}`)
	if err == nil {
		t.Fatal("首次调用应中断等待审批")
	}
	found := false
	for _, e := range rec.SnapshotEvents() {
		if e == "ai:command" {
			found = true
		}
	}
	if !found {
		t.Error("应推送 ai:command 审批事件")
	}
}

func TestSkillTools_RunScriptPathTraversalRejected(t *testing.T) {
	s := skillWithScripts(t, "---\nname: disk-analysis\n---\n", nil)
	st := NewSkillTools("s1", nil, nil, nil, NewToolCallHolder(),
		func(name string) (*skill.Skill, error) { return s, nil })
	got, err := st.runSkillScript(context.Background(), `{"skillName":"disk-analysis","script":"../evil","args":""}`)
	if err != nil {
		t.Fatalf("不应返回 error: %v", err)
	}
	if !strings.Contains(got, "非法") {
		t.Errorf("路径穿越脚本名应被拒绝: %q", got)
	}
}

func TestSkillTools_RunScriptMissingScript(t *testing.T) {
	s := skillWithScripts(t, "---\nname: disk-analysis\n---\n", nil)
	st := NewSkillTools("s1", nil, nil, nil, NewToolCallHolder(),
		func(name string) (*skill.Skill, error) { return s, nil })
	got, err := st.runSkillScript(context.Background(), `{"skillName":"disk-analysis","script":"nope.sh","args":""}`)
	if err != nil {
		t.Fatalf("不应返回 error: %v", err)
	}
	if !strings.Contains(got, "不存在") {
		t.Errorf("脚本缺失应回灌提示: %q", got)
	}
}

func TestSkillTools_ExecuteScriptUploadsAndRuns(t *testing.T) {
	s := skillWithScripts(t, "---\nname: disk-analysis\n---\n", map[string]string{"check.sh": "#!/bin/bash\nls -la"})
	ex := &testutil.FakeExec{Lines: []sshexec.Line{{Stream: "stdout", Text: "output-line"}}}
	rec := &testutil.EmitRecorder{}
	holder := NewToolCallHolder()
	holder.Add(&schema.ToolCall{ID: "call_1", Function: schema.FunctionCall{Name: "run_skill_script"}})
	st := NewSkillTools("s1", ex, rec.Emit, nil, holder,
		func(name string) (*skill.Skill, error) { return s, nil })

	scriptPath, _ := st.scriptPath(s, "check.sh")
	got, err := st.executeScript(context.Background(), s, scriptPath, scriptArgs{SkillName: "disk-analysis", Script: "check.sh", Args: ""}, "run_skill_script(disk-analysis, check.sh)")
	if err != nil {
		t.Fatalf("executeScript: %v", err)
	}
	if !strings.Contains(got, "output-line") {
		t.Errorf("输出应回灌: %q", got)
	}
	cmds := ex.Commands()
	if len(cmds) != 2 {
		t.Fatalf("应执行 2 条命令（写文件+执行），得到 %d: %v", len(cmds), cmds)
	}
	if !strings.Contains(cmds[0], "base64") || !strings.Contains(cmds[0], "chmod") {
		t.Errorf("第一条命令应为上传: %q", cmds[0])
	}
	if !strings.Contains(cmds[1], "check.sh") {
		t.Errorf("第二条命令应为执行脚本: %q", cmds[1])
	}
}

// 恶意技能 ZIP 的脚本名若含 shell 元字符（x; rm -rf .），必须被 scriptPath
// 白名单拒绝，不能命中后拼进 shell 命令执行。
func TestSkillTools_RunScriptShellMetaNameRejected(t *testing.T) {
	s := skillWithScripts(t, "---\nname: disk-analysis\n---\n",
		map[string]string{"x; rm -rf .": "#!/bin/bash\necho ok"})
	st := NewSkillTools("s1", nil, nil, nil, NewToolCallHolder(),
		func(name string) (*skill.Skill, error) { return s, nil })
	got, err := st.runSkillScript(context.Background(),
		`{"skillName":"disk-analysis","script":"x; rm -rf .","args":""}`)
	if err != nil {
		t.Fatalf("不应返回 error: %v", err)
	}
	if !strings.Contains(got, "非法") {
		t.Errorf("含 shell 元字符的脚本名应被拒绝，得到: %q", got)
	}
}

// run_skill_script 的 args 必须被 shell 单引号转义为字面量，
// 分号/命令替换等不得被远端 shell 解释。
func TestSkillTools_ExecuteScriptArgsShellQuoted(t *testing.T) {
	s := skillWithScripts(t, "---\nname: disk-analysis\n---\n",
		map[string]string{"check.sh": "#!/bin/bash\nls -la"})
	ex := &testutil.FakeExec{Lines: []sshexec.Line{{Stream: "stdout", Text: "out"}}}
	holder := NewToolCallHolder()
	holder.Add(&schema.ToolCall{ID: "call_1", Function: schema.FunctionCall{Name: "run_skill_script"}})
	st := NewSkillTools("s1", ex, nil, nil, holder,
		func(name string) (*skill.Skill, error) { return s, nil })

	scriptPath, _ := st.scriptPath(s, "check.sh")
	_, err := st.executeScript(context.Background(), s, scriptPath,
		scriptArgs{SkillName: "disk-analysis", Script: "check.sh", Args: "--path=/tmp/x; rm -rf /"}, "")
	if err != nil {
		t.Fatalf("executeScript: %v", err)
	}
	cmds := ex.Commands()
	if len(cmds) < 2 {
		t.Fatalf("应执行 2 条命令（上传+执行），得到 %d: %v", len(cmds), cmds)
	}
	runCmd := cmds[len(cmds)-1]
	if !strings.Contains(runCmd, "'--path=/tmp/x; rm -rf /'") {
		t.Errorf("args 应被单引号包裹为字面量，得到: %q", runCmd)
	}
	if !strings.Contains(runCmd, "'") {
		t.Errorf("runCmd 应含引号转义: %q", runCmd)
	}
}
