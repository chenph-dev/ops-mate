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

// ObjectNode 对象树节点（数据库 schema、Redis key、K8s 资源等）。
type ObjectNode struct {
	Name     string       `json:"name"`
	Type     string       `json:"type"`
	Children []ObjectNode `json:"children,omitempty"`
}

// QueryRunner 结构化查询能力（数据库 / ES 等）。
type QueryRunner interface {
	Query(ctx context.Context, query string) (*QueryResult, error)
	Exec(ctx context.Context, query string) (*ExecResult, error)
}

// ObjectBrowser 对象树 / 元数据浏览能力。
type ObjectBrowser interface {
	Tree(ctx context.Context) ([]ObjectNode, error)
}

// CommandRunner 命令执行能力（ssh / redis-cli / kubectl 等）。
type CommandRunner interface {
	Run(ctx context.Context, command string) (string, error)
}
