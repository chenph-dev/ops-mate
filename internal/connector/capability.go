package connector

import "context"

// QueryResult 结构化查询返回（列 + 行，值为 JSON 友好类型）。
type QueryResult struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// ExecResult 非查询执行返回。
type ExecResult struct {
	RowsAffected int64 `json:"rowsAffected"`
}

// QueryRunner 结构化查询能力（数据库 / ES 等）。
type QueryRunner interface {
	Query(ctx context.Context, query string) (*QueryResult, error)
	Exec(ctx context.Context, query string) (*ExecResult, error)
}

// Pingable 连接可测试能力（TestConnection / 探活）。
type Pingable interface {
	Ping(ctx context.Context) error
}
