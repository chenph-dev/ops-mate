package store

import "fmt"

// PastCommand 召回的历史命令记录。
type PastCommand struct {
	Command string `json:"command"`
	Output  string `json:"output"`
}

// RecallContext 注入给 AI 的记忆上下文。
type RecallContext struct {
	PastCommands []PastCommand `json:"pastCommands"`
}

// Recall 按 hostID + 关键词从 commands 表 FTS5 检索这台机器过往命令。
// 取 top-N（默认 5）。关键词做简单分词后 OR 拼接。
func (s *Store) Recall(hostID, question string) (RecallContext, error) {
	q := ftsQuery(question)
	if q == "" {
		return RecallContext{}, nil
	}
	rows, err := s.DB.Query(`
		SELECT c.command, COALESCE(c.output,'')
		FROM commands c
		JOIN conversations v ON v.id = c.session_id
		WHERE v.host_id = ?
		  AND c.rowid IN (SELECT rowid FROM commands_fts WHERE commands_fts MATCH ?)
		ORDER BY c.ts DESC
		LIMIT 5`, hostID, q)
	if err != nil {
		return RecallContext{}, fmt.Errorf("recall query: %w", err)
	}
	defer rows.Close()
	var out RecallContext
	for rows.Next() {
		var pc PastCommand
		if err := rows.Scan(&pc.Command, &pc.Output); err != nil {
			return RecallContext{}, err
		}
		out.PastCommands = append(out.PastCommands, pc)
	}
	return out, rows.Err()
}

// ftsQuery 把自然语言问题转成 FTS5 OR 查询，过滤停用词与过短词。
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
		if r == ' ' || r == ',' || r == '？' || r == '?' || r == '。' {
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
		out += "\"" + tk + "\""
	}
	return out
}
