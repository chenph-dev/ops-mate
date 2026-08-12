package termctx

import (
	"regexp"
	"strings"
)

// 上限常量。
const (
	// MaxTotalBytes 清洗后文本总长上限（与模型回灌输出上限一致，防上下文膨胀）。
	MaxTotalBytes = 8 * 1024
	// MaxLineBytes 单行长度上限。
	MaxLineBytes = 1024
)

// ansiRe 匹配常见 ANSI 转义：CSI（颜色/光标）、OSC（标题等）、其它控制。
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\].*?(?:\x07|\x1b\\)|\x1b[>=]|\x1b[()][0-9A-B]`)

// promptRe 匹配 shell 提示符开头的行（命令回显），并捕获其后输入的命令。形如：
//
//	root@web01:~$ df -h   →  df -h
//	$ uptime              →  uptime
//	# whoami              →  whoami
//	> next                →  next
//
// 仅匹配强模式（含提示符特征且后跟空白），避免误删普通输出。
var promptRe = regexp.MustCompile(`^(?:[a-zA-Z0-9_.-]+@[a-zA-Z0-9_.-]+:[^$#>\n]*)?[$#>][ \t]+(.*)$`)

// Clean 把终端原始输出清洗为可注入模型的文本：
//  1. 去 ANSI 转义序列；
//  2. 从命令行回显中提取输入的命令（root@host:~$ ls → "> ls"），空提示符行跳过；
//  3. 合并相邻重复行；
//  4. 截断超长行（MaxLineBytes）与总长（MaxTotalBytes）；
//  5. 空输入或清洗后为空返回空串。
func Clean(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	s := ansiRe.ReplaceAllString(string(raw), "")
	lines := strings.Split(s, "\n")

	out := make([]string, 0, len(lines))
	var prev string
	total := 0
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		if m := promptRe.FindStringSubmatch(line); m != nil {
			// 命令回显行：保留输入的命令（"root@host:~$ ls" → "> ls"）；
			// 空提示符（无命令）跳过，不更新 prev。
			cmd := strings.TrimSpace(m[1])
			if cmd == "" {
				continue
			}
			line = "> " + cmd
		}
		if line == prev {
			// 连续重复行：合并
			continue
		}
		if len(line) > MaxLineBytes {
			line = line[:MaxLineBytes]
		}
		if total+len(line)+1 > MaxTotalBytes {
			break
		}
		out = append(out, line)
		prev = line
		total += len(line) + 1
	}
	return strings.Join(out, "\n")
}
