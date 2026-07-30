package orchestrator

import (
	"regexp"
	"strings"
)

// 危险模式：匹配即标 "high"（标红 + 二次确认，引擎不硬拒）。
var dangerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+-[a-zA-Z]*r[a-zA-Z]*f?.*\s/\s`),       // rm -rf ... /
	regexp.MustCompile(`rm\s+-[a-zA-Z]*r[a-zA-Z]*f?\s+/\s*$`),      // rm -rf /
	regexp.MustCompile(`\bmkfs\b`),
	regexp.MustCompile(`\bdd\b.*\bof=/dev/`),
	regexp.MustCompile(`\b(shutdown|poweroff|halt|reboot)\b`),
	regexp.MustCompile(`>\s*/dev/(sd|nvme|hd|vd)`),                 // 重定向到块设备
	regexp.MustCompile(`:\(\)\s*\{\s*:\|:&\s*\}\s*;`),               // fork bomb
}

// AssessRisk 返回命令风险等级："" 无风险，"high" 危险。
// 引擎层永不硬拒——这里只用于前端标红与二次确认。
func AssessRisk(command string) string {
	c := strings.TrimSpace(command)
	for _, p := range dangerPatterns {
		if p.MatchString(c) {
			return "high"
		}
	}
	return ""
}
