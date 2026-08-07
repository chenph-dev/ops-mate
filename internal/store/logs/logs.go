// Package logsstore 提供 AI 调用审计日志的读写与聚合。
package logsstore

import (
	"fmt"
	"time"

	"ops-mate/internal/store"
	"ops-mate/internal/store/crypto"
)

// CallLog 单次 AI 组件调用（模型/工具）的审计记录（DTO）。
type CallLog struct {
	ID          string `json:"id"`
	SessionID   string `json:"sessionId"`
	Ts          int64  `json:"ts"`
	Component   string `json:"component"` // "model" | "tool"
	Name        string `json:"name"`      // eino 节点名，如 "llm"
	Provider    string `json:"provider"`  // 实现类型，如 "OpenAI"
	TokensIn    int    `json:"tokensIn"`
	TokensOut   int    `json:"tokensOut"`
	TokensTotal int    `json:"tokensTotal"`
	DurationMS  int64  `json:"durationMs"`
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
}

// TokenSummary token 用量聚合（审计页统计卡片）。
type TokenSummary struct {
	TotalCalls  int `json:"totalCalls"`
	ModelCalls  int `json:"modelCalls"`
	ToolCalls   int `json:"toolCalls"`
	TokensIn    int `json:"tokensIn"`
	TokensOut   int `json:"tokensOut"`
	TokensTotal int `json:"tokensTotal"`
}

// aiCallLog GORM 模型，对应 ai_call_logs 表。
type aiCallLog struct {
	ID          string `gorm:"column:id;primaryKey"`
	SessionID   string `gorm:"column:session_id"`
	Ts          int64  `gorm:"column:ts"`
	Component   string `gorm:"column:component"`
	Name        string `gorm:"column:name"`
	Provider    string `gorm:"column:provider"`
	TokensIn    int    `gorm:"column:tokens_in"`
	TokensOut   int    `gorm:"column:tokens_out"`
	TokensTotal int    `gorm:"column:tokens_total"`
	DurationMS  int64  `gorm:"column:duration_ms"`
	OK          bool   `gorm:"column:ok"`
	Error       string `gorm:"column:error"`
}

func (aiCallLog) TableName() string { return "ai_call_logs" }

// LogsStore 提供审计日志读写。
type LogsStore struct {
	app *store.DB
}

// NewLogsStore 构造 LogsStore。
func NewLogsStore(app *store.DB) *LogsStore {
	return &LogsStore{app: app}
}

// SaveLog 落库一条审计记录。
func (s *LogsStore) SaveLog(c CallLog) error {
	return s.app.GORM().Create(&aiCallLog{
		ID: crypto.NewID(), SessionID: c.SessionID, Ts: time.Now().Unix(),
		Component: c.Component, Name: c.Name, Provider: c.Provider,
		TokensIn: c.TokensIn, TokensOut: c.TokensOut, TokensTotal: c.TokensTotal,
		DurationMS: c.DurationMS, OK: c.OK, Error: c.Error,
	}).Error
}

// ListLogs 返回最近 limit 条审计记录（按时间倒序）。
func (s *LogsStore) ListLogs(limit int) ([]CallLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var rows []aiCallLog
	if err := s.app.GORM().Order("ts DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]CallLog, 0, len(rows))
	for _, r := range rows {
		out = append(out, toCallLog(r))
	}
	return out, nil
}

// TokenSummary 统计全部审计记录的调用次数与 token 用量。
func (s *LogsStore) TokenSummary() (TokenSummary, error) {
	var total, modelCalls, toolCalls int64
	var inT, outT, totalT int64
	if err := s.app.GORM().Model(&aiCallLog{}).Count(&total).Error; err != nil {
		return TokenSummary{}, err
	}
	if err := s.app.GORM().Model(&aiCallLog{}).Where("component = ?", "model").Count(&modelCalls).Error; err != nil {
		return TokenSummary{}, err
	}
	if err := s.app.GORM().Model(&aiCallLog{}).Where("component = ?", "tool").Count(&toolCalls).Error; err != nil {
		return TokenSummary{}, err
	}
	if err := s.app.GORM().Model(&aiCallLog{}).
		Select("COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0), COALESCE(SUM(tokens_total),0)").
		Row().Scan(&inT, &outT, &totalT); err != nil {
		return TokenSummary{}, fmt.Errorf("sum tokens: %w", err)
	}
	return TokenSummary{
		TotalCalls: int(total), ModelCalls: int(modelCalls), ToolCalls: int(toolCalls),
		TokensIn: int(inT), TokensOut: int(outT), TokensTotal: int(totalT),
	}, nil
}

func toCallLog(r aiCallLog) CallLog {
	return CallLog{
		ID: r.ID, SessionID: r.SessionID, Ts: r.Ts,
		Component: r.Component, Name: r.Name, Provider: r.Provider,
		TokensIn: r.TokensIn, TokensOut: r.TokensOut, TokensTotal: r.TokensTotal,
		DurationMS: r.DurationMS, OK: r.OK, Error: r.Error,
	}
}
