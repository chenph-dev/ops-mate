package dbexec

import (
	"context"

	"ops-mate/internal/connector"
)

// connectorQueryRunner 把 Executor 适配为 connector.QueryRunner + Pingable。
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

func (a *connectorQueryRunner) Ping(ctx context.Context) error {
	return a.e.Ping(ctx)
}

// paramString 从 Config.Params 取字符串参数，缺失返回空串。
func paramString(cfg connector.Config, key string) string {
	if v, ok := cfg.Params[key].(string); ok {
		return v
	}
	return ""
}

// dbSkillPrompt 数据库连接的协议提示词片段（DBA 语义，注入系统提示词）。
const dbSkillPrompt = `目标数据库。你拥有 execute_sql 工具，用于在目标数据库执行一条 SQL 语句。严格遵循以下规则：
1. 串行执行：每轮只调用一次 execute_sql、只提议一条 SQL。绝不要把多条语句写进同一次回复或同一次工具调用；执行完一条并确认结果后，才能提议下一条。
2. 等待结果：每次 SQL 执行后，必须等 execute_sql 的结果（tool 消息中的真实输出与受影响行数）返回，基于该结果分析后，再决定是否需要执行下一条。不要在没有看到任何结果时就连续提议。
3. 你提议的每条 SQL 都会先展示给用户审批，批准后才会执行。不要假设已经执行；只根据 tool 消息中的真实执行结果做分析。
4. 优先使用只读查询（SELECT/SHOW/DESC/EXPLAIN）；写操作（INSERT/UPDATE/DELETE/CREATE 等）必须先明确说明影响范围，高危操作（DROP/TRUNCATE/ALTER）必须强调不可恢复。
5. 用户拒绝某条 SQL 时，不要重复提议同一条；换其他方案或向用户询问更多信息。
6. 当你已有足够信息回答时，直接用文本给出结论，不要再提议 SQL。
7. 计划模式：面对复杂/多步任务（需要多条 SQL 诊断/修复），先调用 create_plan 提交执行计划（目标 + 步骤列表）供用户审批，批准后再按计划逐步执行（每步仍通过 execute_sql 提议等待审批）。简单单条任务直接使用 execute_sql，不要使用 create_plan。`

// registerDBDriver 注册一个"需要 host 的数据库驱动"（mysql/postgres）。
func registerDBDriver(protocol, name, driver string) {
	connector.Register(&connector.Driver{
		Protocol:  protocol,
		Name:      name,
		NeedsHost: true,
		Params: []connector.ParamSchema{
			{Key: "database", Label: "数据库", Type: connector.ParamString, Required: true, Placeholder: "myapp"},
		},
		SkillPack: connector.SkillPack{Prompt: dbSkillPrompt, Guardrail: "sql"},
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
