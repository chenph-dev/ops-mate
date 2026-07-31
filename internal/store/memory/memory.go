// Package memorystore 提供基于 FTS5 的历史命令召回，用于 AI 记忆注入。
package memorystore

import (
	"fmt"
	"strings"

	"ops-mate/internal/store"
)

// PastCommand 召回的历史命令记录。
type PastCommand struct {
	Command string `json:"command"`
	Output  string `json:"output"`
}

// RecallContext 注入给 AI 的记忆上下文。
type RecallContext struct {
	PastCommands []PastCommand `json:"pastCommands"`
}

// MemoryStore 提供历史命令召回操作。
type MemoryStore struct {
	app *store.DB
}

// NewMemoryStore 构造 MemoryStore。
func NewMemoryStore(app *store.DB) *MemoryStore {
	return &MemoryStore{app: app}
}

// Recall 按 hostID + 关键词从 commands 表 FTS5 检索这台机器过往命令。
// 取 top-N（默认 5）。关键词做简单分词后 OR 拼接。
func (s *MemoryStore) Recall(hostID, question string) (RecallContext, error) {
	q := ftsQuery(question)
	if q == "" {
		return RecallContext{}, nil
	}
	var pcs []PastCommand
	err := s.app.GORM().Raw(`
		SELECT c.command, COALESCE(c.output,'')
		FROM commands c
		JOIN conversations v ON v.id = c.session_id
		WHERE v.host_id = ?
		  AND c.rowid IN (SELECT rowid FROM commands_fts WHERE commands_fts MATCH ?)
		ORDER BY c.ts DESC
		LIMIT 5`, hostID, q).Scan(&pcs).Error
	if err != nil {
		return RecallContext{}, fmt.Errorf("recall query: %w", err)
	}
	return RecallContext{PastCommands: pcs}, nil
}

// ftsQuery 把自然语言问题转成 FTS5 OR 查询，过滤停用词与过短词。
// 对每个 token 用双引号包裹并转义内部引号，防止 FTS5 注入。
func ftsQuery(s string) string {
	stop := map[string]bool{"the": true, "a": true, "an": true, "is": true,
		"为什么": true, "怎么": true, "怎么回事": true}
	var tokens []string
	cur := ""
	flush := func() {
		if cur != "" && !stop[cur] && len(cur) > 1 {
			tokens = append(tokens, cur)
		}
		cur = ""
	}
	for _, r := range s {
		// 将 FTS5 特殊字符视为分隔符，避免注入
		if r == ' ' || r == ',' || r == '？' || r == '?' || r == '。' ||
			r == '"' || r == '*' || r == '(' || r == ')' || r == ':' || r == '^' {
			flush()
			continue
		}
		cur += string(r)
	}
	flush()
	out := ""
	for i, tk := range tokens {
		if i > 0 {
			out += " OR "
		}
		// 转义内部双引号（FTS5 用 "" 转义 "）
		out += "\"" + strings.ReplaceAll(tk, "\"", "\"\"") + "\""
	}
	return out
}
