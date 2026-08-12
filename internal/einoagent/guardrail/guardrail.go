// Package guardrail 提供危险命令护栏（AssessRisk）。
package guardrail

import (
	"regexp"
	"strings"
)

// ============================================================
// 危险命令护栏
// ============================================================

// dangerPatterns 匹配即返回 "high" 风险等级。
var dangerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+-[a-zA-Z]*r[a-zA-Z]*f?.*\s/\s`),       // rm -rf ... /
	regexp.MustCompile(`rm\s+-[a-zA-Z]*r[a-zA-Z]*f?\s+/(\*|\s*$)`), // rm -rf / 或 rm -rf /*
	regexp.MustCompile(`\bmkfs\b`),
	regexp.MustCompile(`\bdd\b.*\bof=/dev/`),
	regexp.MustCompile(`\b(shutdown|poweroff|halt|reboot)\b`),
	regexp.MustCompile(`>\s*/dev/(sd|nvme|hd|vd)`),    // 重定向到块设备
	regexp.MustCompile(`:\(\)\s*\{\s*:\|:&\s*\}\s*;`), // fork bomb
}

// AssessRisk 返回命令风险等级："" 无风险，"high" 危险。
// 引擎层永不硬拒——只用于前端标红与二次确认。
func AssessRisk(command string) string {
	c := strings.TrimSpace(command)
	for _, p := range dangerPatterns {
		if p.MatchString(c) {
			return "high"
		}
	}
	return ""
}

// Action 审批动作建议（Classify 返回）。
type Action string

const (
	ActionAuto    Action = "auto"    // 自动放行（只读）
	ActionApprove Action = "approve" // 人工审批
)

// DefaultReadOnlyCommands 内置只读命令白名单（策略未配置时兜底）。
// 仅收录无副作用命令；每条命令必须不含管道/重定向/命令替换才会命中只读。
var DefaultReadOnlyCommands = []string{
	"ls", "df", "free", "tail", "cat", "ps", "grep", "awk", "top",
	"uptime", "who", "hostname", "date", "du", "stat", "whoami",
	"echo", "env", "uname",
}

// ParseReadOnlyList 把逗号/空白/换行分隔的白名单配置解析为去重、去空白的命令列表。
func ParseReadOnlyList(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '\n' || r == ' ' || r == '\t'
	})
	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	return out
}

// IsReadOnlyCommand 判定命令是否只读：首命令精确命中白名单，且不含
// 管道（|）、重定向（>、>>）、命令替换（$()、反引号）、命令分隔符（;）。
// 任一命中即视为非只读（保守降级）。
func IsReadOnlyCommand(command string, whitelist []string) bool {
	c := strings.TrimSpace(command)
	if c == "" || len(whitelist) == 0 {
		return false
	}
	if strings.ContainsAny(c, "|><`") || strings.Contains(c, "$(") ||
		strings.Contains(c, "&&") || strings.Contains(c, ";") {
		return false
	}
	first := c
	if i := strings.IndexAny(first, " \t"); i >= 0 {
		first = first[:i]
	}
	for _, w := range whitelist {
		if first == strings.TrimSpace(w) {
			return true
		}
	}
	return false
}

// Classify 综合分类：返回风险等级（"high"/"read"/"write"）与建议动作。
// readOnlyWhitelist 为空时回退内置默认白名单。
// 高危命令永远 approve；只读命中 auto；其余 write → approve。
func Classify(command string, readOnlyWhitelist []string) (string, Action) {
	if AssessRisk(command) == "high" {
		return "high", ActionApprove
	}
	wl := readOnlyWhitelist
	if len(wl) == 0 {
		wl = DefaultReadOnlyCommands
	}
	if IsReadOnlyCommand(command, wl) {
		return "read", ActionAuto
	}
	return "write", ActionApprove
}
