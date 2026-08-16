package connector

import "strings"

// IsQuery 判断 SQL 首关键字是否为查询类（走 QueryRunner.Query 返回行集）。
// 仅按首关键字粗分；WITH 开头的写语句（如 CTE + UPDATE）会保守走 Query，
// 但 Query 对无行结果返回空 Rows，不影响正确性。
func IsQuery(sqlText string) bool {
	switch firstKeyword(sqlText) {
	case "select", "show", "desc", "describe", "explain", "with", "pragma", "values":
		return true
	}
	return false
}

// firstKeyword 提取 SQL 的首个关键字（小写）。
func firstKeyword(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t\r\n"); i >= 0 {
		s = s[:i]
	}
	return strings.ToLower(s)
}
