package dbexec

import (
	"context"

	"ops-mate/internal/connector"
)

// connectorQueryRunner 把 Executor 适配为 connector.QueryRunner + ObjectBrowser。
type connectorQueryRunner struct {
	e *Executor
}

func (a *connectorQueryRunner) Query(ctx context.Context, query string) (*connector.QueryResult, error) {
	r, err := a.e.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	return &connector.QueryResult{Columns: r.Columns, Rows: r.Rows}, nil
}

func (a *connectorQueryRunner) Exec(ctx context.Context, query string) (*connector.ExecResult, error) {
	r, err := a.e.Exec(ctx, query)
	if err != nil {
		return nil, err
	}
	return &connector.ExecResult{RowsAffected: r.RowsAffected}, nil
}

// Tree 把 schema 表/列转为对象树（表 → 列）。
func (a *connectorQueryRunner) Tree(ctx context.Context) ([]connector.ObjectNode, error) {
	s, err := a.e.Schema(ctx)
	if err != nil {
		return nil, err
	}
	nodes := make([]connector.ObjectNode, 0, len(s.Tables))
	for _, t := range s.Tables {
		n := connector.ObjectNode{Name: t.Name, Type: "table"}
		for _, c := range t.Columns {
			n.Children = append(n.Children, connector.ObjectNode{Name: c.Name, Type: c.DataType})
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

// paramString 从 Config.Params 取字符串参数，缺失返回空串。
func paramString(cfg connector.Config, key string) string {
	if v, ok := cfg.Params[key].(string); ok {
		return v
	}
	return ""
}

// dbSkillPrompt 数据库连接的协议提示词片段（DBA 语义，注入系统提示词）。
const dbSkillPrompt = `你是数据库运维智能体，帮助用户诊断与操作目标数据库。你拥有 execute_sql 工具：
1. 串行执行：每轮只调用一次 execute_sql、只提议一条 SQL，执行完并确认结果后再提议下一条。
2. 每条 SQL 都会先展示给用户审批，批准后才执行；只根据工具返回的真实结果做分析。
3. 优先只读查询（SELECT/SHOW/DESC/EXPLAIN）；写操作（INSERT/UPDATE/DELETE/CREATE 等）必须先说明影响范围，高危操作（DROP/TRUNCATE/ALTER）必须强调不可恢复。
4. 用户拒绝某条 SQL 时不要重复提议同一条；已有足够信息时直接用文本给出结论。`

// registerDBDriver 注册一个"需要 host 的数据库驱动"（mysql/postgres）。
func registerDBDriver(protocol, name, driver string) {
	connector.Register(&connector.Driver{
		Protocol:  protocol,
		Name:      name,
		NeedsHost: true,
		Params: []connector.ParamSchema{
			{Key: "database", Label: "数据库", Type: connector.ParamString, Required: true, Placeholder: "myapp"},
		},
		Capabilities: []string{"query", "objectTree"},
		SkillPack:    connector.SkillPack{Prompt: dbSkillPrompt, Guardrail: "sql"},
		New: func(cfg connector.Config) (connector.Capability, error) {
			ex := NewExecutor(Host{
				Driver: driver, Addr: cfg.Addr, Port: cfg.Port,
				User: cfg.User, Password: cfg.Password,
				Database: paramString(cfg, "database"),
			})
			return &connectorQueryRunner{e: ex}, nil
		},
	})
}

// init 自动注册（import dbexec 即生效，供 resolver / 前端按 protocol 查询）。
func init() {
	registerDBDriver("mysql", "MySQL", "mysql")
	registerDBDriver("postgres", "PostgreSQL", "postgres")
}
