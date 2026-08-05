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
