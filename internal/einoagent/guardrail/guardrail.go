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

// sqlCommentBlockRe / sqlCommentLineRe 用于归一化前剥离 SQL 注释，
// 防止 INTO/**/OUTFILE、FOR\tUPDATE、LOAD/**/FILE 等注释/空白变体绕过危险模式判定。
var (
	sqlCommentBlockRe = regexp.MustCompile(`(?s)/\*.*?\*/`)
	sqlCommentLineRe  = regexp.MustCompile(`(?m)--.*$|(?m)#.*$`)
)

// sensitiveReadPatterns 只读命令参数命中任一即降级为人工审批。
// 聚焦凭据/密钥/认证信息文件，避免自动放行把敏感内容读出来回灌模型。
var sensitiveReadPatterns = []string{
	"/etc/shadow", "/etc/passwd", "/etc/security",
	"/var/run/secrets", "/run/secrets",
	".ssh", "id_rsa", "id_ed25519", "authorized_keys", "known_hosts",
	"kubeconfig", ".kube", ".aws", "credentials", ".env", ".netrc",
	".pgpass", ".npmrc", ".pypirc", ".gitconfig",
	"bash_history", "zsh_history",
}

// shellDequoteReplacer 剥离单/双引号与反斜杠，防 cat /etc/sh\adow、cat /etc/sh''adow 绕过。
var shellDequoteReplacer = strings.NewReplacer("'", "", `"`, "", `\`, "")

// AssessRiskForProtocol 按协议返回命令风险等级："" 无风险，"high" 危险。
// protocol 为 "winrm" 时额外套用 Windows 危险模式；"sql" 走 SQL 语义。
func AssessRiskForProtocol(command, protocol string) string {
	c := strings.TrimSpace(command)
	if strings.EqualFold(protocol, "sql") {
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
// 注意：不含 env（直接输出全部环境变量，可能含密钥）。敏感读取（cat /etc/shadow 等）
// 由 IsReadOnlyCommand 的参数审计额外降级，不在此白名单层处理。
var DefaultReadOnlyCommands = []string{
	"ls", "df", "free", "tail", "cat", "ps", "grep", "awk", "top",
	"uptime", "who", "hostname", "date", "du", "stat", "whoami",
	"echo", "uname",
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

// IsReadOnlyCommand 判定命令是否只读：首命令精确命中白名单，不含
// 管道（|）、重定向（>、>>）、命令替换（$()、反引号）、命令分隔符（;），
// 且参数未触及敏感文件/目录（见 sensitiveReadPatterns）。
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
			return !containsSensitiveReadArg(c)
		}
	}
	return false
}

// containsSensitiveReadArg 判断只读命令参数是否触及敏感文件/目录。
// 先剥离引号与反斜杠转义，防 cat /etc/sh\adow、cat /etc/sh''adow 绕过。
func containsSensitiveReadArg(command string) bool {
	dequoted := shellDequoteReplacer.Replace(strings.ToLower(command))
	for _, p := range sensitiveReadPatterns {
		if strings.Contains(dequoted, p) {
			return true
		}
	}
	return false
}

// ClassifyForProtocol 按协议综合分类：返回风险等级（"high"/"read"/"write"）与建议动作。
// readOnlyWhitelist 为空时回退协议对应的内置默认白名单。
// 高危命令永远 approve；只读命中 auto；其余 write → approve。
// sql 协议按 SQL 关键字语义分类（只读查询 auto、高危 DDL approve、其余写 approve）。
func ClassifyForProtocol(command string, readOnlyWhitelist []string, protocol string) (string, Action) {
	if strings.EqualFold(protocol, "sql") {
		return classifySQL(command)
	}
	if strings.EqualFold(protocol, "redis") {
		return classifyRedis(command)
	}
	if strings.EqualFold(protocol, "es") {
		return classifyES(command)
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

// redisDangerPatterns Redis 高危命令（命中即 high/approve）。
// 命令统一转大写后匹配（Redis 命令惯例大写）。
var redisDangerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\s*FLUSH(ALL|DB)\b`),
	regexp.MustCompile(`^\s*SHUTDOWN\b`),
	regexp.MustCompile(`^\s*CONFIG\s+(SET|REWRITE)\b`),
	regexp.MustCompile(`^\s*(SLAVEOF|REPLICAOF)\b`),
	regexp.MustCompile(`^\s*MONITOR\b`),
	regexp.MustCompile(`^\s*DEBUG\b`),
	regexp.MustCompile(`^\s*CLIENT\s+PAUSE\b`),
}

// redisReadOnly Redis 只读命令（首关键字精确匹配，命中即 auto）。
// 仅收录无副作用命令；写命令（SET/DEL/HSET/EXPIRE 等）默认 write/approve。
var redisReadOnly = map[string]bool{
	// string
	"get": true, "mget": true, "getrange": true, "strlen": true,
	"exists": true, "type": true, "ttl": true, "pttl": true,
	// hash
	"hget": true, "hmget": true, "hgetall": true, "hkeys": true, "hvals": true,
	"hlen": true, "hexists": true,
	// list
	"llen": true, "lindex": true, "lrange": true,
	// set
	"smembers": true, "scard": true, "sismember": true, "srandmember": true,
	"sinter": true, "sunion": true, "sdiff": true,
	// zset
	"zrange": true, "zrevrange": true, "zrangebyscore": true, "zrevrangebyscore": true,
	"zrangebylex": true, "zcard": true, "zscore": true, "zcount": true,
	"zrank": true, "zrevrank": true, "zmscore": true,
	// server / info / scan / introspection
	"info": true, "ping": true, "time": true, "dbsize": true, "echo": true,
	"scan": true, "hscan": true, "sscan": true, "zscan": true, "keys": true,
	"object": true, "memory": true, "command": true,
}

// classifyRedis 按 Redis 命令分类（redis 协议）：
// 高危命令（FLUSHALL/FLUSHDB/SHUTDOWN/CONFIG SET 等）→ high/approve；
// 只读命令（GET/HGETALL/LRANGE 等）→ read/auto；
// 其余写命令 → write/approve。多命令（换行/分号分隔）保守人工审批。
func classifyRedis(command string) (string, Action) {
	c := strings.ToUpper(strings.TrimSpace(command))
	if strings.Contains(c, ";") || strings.Contains(c, "\n") {
		return "write", ActionApprove
	}
	for _, p := range redisDangerPatterns {
		if p.MatchString(c) {
			return "high", ActionApprove
		}
	}
	if redisReadOnly[redisKeyword(c)] {
		return "read", ActionAuto
	}
	return "write", ActionApprove
}

// redisKeyword 提取命令首关键字（输入已大写）。
func redisKeyword(s string) string {
	if i := strings.IndexAny(s, " \t\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}

// esReadOnlyPrefixes ES 只读系统 API 前缀（GET + 命中即 read/auto）。
var esReadOnlyPrefixes = []string{
	"_cat", "_cluster", "_nodes", "_stats", "_mapping", "_settings", "_aliases",
}

// classifyES 按 ES REST 查询分类（es 协议）：
// 高危（DELETE 任何目标、_all、_delete_by_query/_update_by_query/_reindex）→ high/approve；
// 只读（GET + 系统 API 或 /_search）→ read/auto；其余写操作 → write/approve。
func classifyES(command string) (string, Action) {
	c := strings.TrimSpace(command)
	if c == "" {
		return "write", ActionApprove
	}
	first := c
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = first[:i]
	}
	first = strings.TrimSpace(first)
	method, path := "GET", strings.TrimSpace(first)
	if i := strings.IndexByte(first, ' '); i > 0 {
		upper := strings.ToUpper(first[:i])
		switch upper {
		case "GET", "POST", "PUT", "DELETE", "HEAD":
			method, path = upper, strings.TrimSpace(first[i+1:])
		}
	}
	lowerPath := strings.ToLower(path)
	if method == "DELETE" || strings.Contains(lowerPath, "_all") ||
		strings.Contains(lowerPath, "_delete_by_query") ||
		strings.Contains(lowerPath, "_update_by_query") ||
		strings.Contains(lowerPath, "_reindex") {
		return "high", ActionApprove
	}
	if method == "GET" {
		for _, pre := range esReadOnlyPrefixes {
			if strings.HasPrefix(lowerPath, pre) {
				return "read", ActionAuto
			}
		}
		if strings.HasSuffix(lowerPath, "/_search") {
			return "read", ActionAuto
		}
	}
	return "write", ActionApprove
}

// classifySQL 按 SQL 首关键字分类（sql 协议）：
// 只读查询（SELECT/SHOW/DESC/EXPLAIN 等）→ auto；高危 DDL（DROP/TRUNCATE/ALTER）→ high/approve；
// 其余（INSERT/UPDATE/DELETE/CREATE/GRANT 及一切写）→ write/approve。
// 保守策略：WITH 开头的 CTE（可能是写语句）与含分号的多语句一律人工审批，不自动放行。
// 判定前先 normalizeSQL 剥离注释/折叠空白，防止注释与空白变体绕过护栏。
func classifySQL(sqlText string) (string, Action) {
	norm := normalizeSQL(sqlText)
	if strings.Contains(norm, ";") {
		return "write", ActionApprove
	}
	kw := sqlKeyword(norm)
	switch kw {
	case "select", "show", "desc", "describe", "explain", "use", "pragma", "values":
		// 命中只读关键字仍须降级检查：SELECT INTO OUTFILE / LOAD_FILE / FOR UPDATE
		// 以及 PRAGMA journal_mode= / writable_schema= 是写或危险操作，不得自动放行。
		// 去空白/下划线后匹配，覆盖 INTO/**/OUTFILE、INTO\tOUTFILE、LOAD_FILE/LOAD FILE 变体。
		compact := strings.Map(func(r rune) rune {
			if r == ' ' || r == '_' {
				return -1
			}
			return r
		}, norm)
		if strings.Contains(compact, "INTOOUTFILE") ||
			strings.Contains(compact, "LOADFILE") ||
			strings.Contains(compact, "FORUPDATE") {
			return "write", ActionApprove
		}
		if kw == "pragma" &&
			(strings.Contains(norm, "=") ||
				strings.Contains(norm, "JOURNAL_MODE") ||
				strings.Contains(norm, "WRITABLE_SCHEMA")) {
			return "write", ActionApprove
		}
		return "read", ActionAuto
	case "drop", "truncate", "alter":
		return "high", ActionApprove
	default:
		return "write", ActionApprove
	}
}

// normalizeSQL 归一化 SQL 供护栏判定：剥离 /*...*/、--、# 注释，
// 折叠空白（含换行与注释残留空洞）为大写单空格串。注释剥除后
// SEL/**/ECT 可还原为 SELECT、INTO/**/OUTFILE 还原为 INTO OUTFILE。
func normalizeSQL(sqlText string) string {
	s := sqlCommentBlockRe.ReplaceAllString(sqlText, " ")
	s = sqlCommentLineRe.ReplaceAllString(s, " ")
	return strings.ToUpper(strings.Join(strings.Fields(s), " "))
}

// sqlKeyword 提取 SQL 首关键字（小写，忽略前导空白）。
func sqlKeyword(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t\r\n"); i >= 0 {
		s = s[:i]
	}
	return strings.ToLower(s)
}
