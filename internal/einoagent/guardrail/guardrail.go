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

// windowsDangerPatterns WinRM 额外匹配的危险 PowerShell/cmd 模式。
var windowsDangerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bformat\s+[a-zA-Z]:`),
	regexp.MustCompile(`(?i)\bdel\s+.*\/s`),
	regexp.MustCompile(`(?i)\brd\s+.*\/s`),
	regexp.MustCompile(`(?i)\brmdir\s+.*\/s`),
	regexp.MustCompile(`(?i)\bshutdown\s+\/[sr]`),
	regexp.MustCompile(`(?i)Remove-Item.*(-Recurse.*-Force|-Force.*-Recurse)`),
	regexp.MustCompile(`(?i)(?:\bdiskpart\b|\bselect\s+disk\b)[\s\S]*\bclean\b`),
}

// AssessRisk 返回命令风险等级："" 无风险，"high" 危险。
// 引擎层永不硬拒——只用于前端标红与二次确认。
// 为向后兼容，沿用 Linux 语义。
func AssessRisk(command string) string {
	return AssessRiskForProtocol(command, "ssh")
}

// AssessRiskForProtocol 按协议返回命令风险等级："" 无风险，"high" 危险。
// protocol 为 "winrm" 时额外套用 Windows 危险模式；"jdbc"/"sql" 走 SQL 语义。
func AssessRiskForProtocol(command, protocol string) string {
	c := strings.TrimSpace(command)
	if strings.EqualFold(protocol, "jdbc") || strings.EqualFold(protocol, "sql") {
		risk, _ := classifySQL(c)
		if risk == "high" {
			return "high"
		}
		return ""
	}
	for _, p := range dangerPatterns {
		if p.MatchString(c) {
			return "high"
		}
	}
	if strings.EqualFold(protocol, "winrm") {
		for _, p := range windowsDangerPatterns {
			if p.MatchString(c) {
				return "high"
			}
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

// DefaultReadOnlyCommandsWindows WinRM 资产内置只读 PowerShell/cmd 命令白名单。
var DefaultReadOnlyCommandsWindows = []string{
	"Get-Command", "Get-Process", "Get-Service", "Get-EventLog", "Get-WinEvent",
	"Get-Content", "Get-ChildItem", "Get-NetIPAddress", "Get-Date",
	"ipconfig", "systeminfo", "hostname", "whoami", "netstat", "tasklist",
	"Test-Connection",
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
// 为向后兼容，沿用 Linux（ssh）语义。
func Classify(command string, readOnlyWhitelist []string) (string, Action) {
	return ClassifyForProtocol(command, readOnlyWhitelist, "ssh")
}

// ClassifyForProtocol 按协议综合分类：返回风险等级（"high"/"read"/"write"）与建议动作。
// readOnlyWhitelist 为空时回退协议对应的内置默认白名单。
// 高危命令永远 approve；只读命中 auto；其余 write → approve。
// jdbc/sql 协议按 SQL 关键字语义分类（只读查询 auto、高危 DDL approve、其余写 approve）。
func ClassifyForProtocol(command string, readOnlyWhitelist []string, protocol string) (string, Action) {
	if strings.EqualFold(protocol, "jdbc") || strings.EqualFold(protocol, "sql") {
		return classifySQL(command)
	}
	if AssessRiskForProtocol(command, protocol) == "high" {
		return "high", ActionApprove
	}
	wl := readOnlyWhitelist
	if len(wl) == 0 {
		if strings.EqualFold(protocol, "winrm") {
			wl = DefaultReadOnlyCommandsWindows
		} else {
			wl = DefaultReadOnlyCommands
		}
	}
	if IsReadOnlyCommand(command, wl) {
		return "read", ActionAuto
	}
	return "write", ActionApprove
}

// classifySQL 按 SQL 首关键字分类（jdbc 协议）：
// 只读查询（SELECT/SHOW/DESC/EXPLAIN 等）→ auto；高危 DDL（DROP/TRUNCATE/ALTER）→ high/approve；
// 其余（INSERT/UPDATE/DELETE/CREATE/GRANT 及一切写）→ write/approve。
// 保守策略：WITH 开头的 CTE（可能是写语句）与含分号的多语句一律人工审批，不自动放行。
func classifySQL(sqlText string) (string, Action) {
	if strings.Contains(sqlText, ";") {
		return "write", ActionApprove
	}
	kw := sqlKeyword(sqlText)
	switch kw {
	case "select", "show", "desc", "describe", "explain", "use", "pragma", "values":
		// 命中只读关键字仍须降级检查：SELECT INTO OUTFILE / LOAD_FILE / FOR UPDATE
		// 以及 PRAGMA journal_mode= / writable_schema= 是写或危险操作，不得自动放行。
		upper := strings.ToUpper(sqlText)
		if strings.Contains(upper, "INTO OUTFILE") ||
			strings.Contains(upper, "LOAD_FILE") ||
			strings.Contains(upper, "FOR UPDATE") {
			return "write", ActionApprove
		}
		if kw == "pragma" &&
			(strings.Contains(upper, "=") ||
				strings.Contains(upper, "JOURNAL_MODE") ||
				strings.Contains(upper, "WRITABLE_SCHEMA")) {
			return "write", ActionApprove
		}
		return "read", ActionAuto
	case "drop", "truncate", "alter":
		return "high", ActionApprove
	default:
		return "write", ActionApprove
	}
}

// sqlKeyword 提取 SQL 首关键字（小写，忽略前导空白）。
func sqlKeyword(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t\r\n"); i >= 0 {
		s = s[:i]
	}
	return strings.ToLower(s)
}
