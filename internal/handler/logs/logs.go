// Package logs 提供 AI 审计日志的 Wails 绑定 handler。
package logs

import logsstore "ops-mate/internal/store/logs"

// LogsHandler 处理 AI 调用审计日志的前端查询。
type LogsHandler struct {
	logs *logsstore.LogsStore
}

// NewLogsHandler 构造 LogsHandler。
func NewLogsHandler(logs *logsstore.LogsStore) *LogsHandler {
	return &LogsHandler{logs: logs}
}

// ListLogs 返回最近 limit 条审计记录（按时间倒序）。
func (h *LogsHandler) ListLogs(limit int) ([]logsstore.CallLog, error) {
	return h.logs.ListLogs(limit)
}

// TokenSummary 返回全部审计记录的调用次数与 token 用量聚合。
func (h *LogsHandler) TokenSummary() (logsstore.TokenSummary, error) {
	return h.logs.TokenSummary()
}
