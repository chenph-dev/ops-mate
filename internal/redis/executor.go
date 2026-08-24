// Package redis 注册 Redis 连接器（KindDB 型，实现 QueryRunner + Pingable）。
package redis

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"ops-mate/internal/connector"
)

// executor 实现 connector.QueryRunner + connector.Pingable。
type executor struct {
	client *redis.Client
}

// NewExecutor 从配置构造 Redis 执行器（懒连接，首次命令时才建立）。
// host/port/password 优先取通用字段（cfg.Addr/Port/Password，与 mysql/postgres
// 同源，前端填在通用区块）；Params 仅承载 redis 特有参数（db 编号），
// 兼容旧设计把 host/port/password 放进 Params 的情况（回退读取）。
func NewExecutor(cfg connector.Config) (connector.Capability, error) {
	host := cfg.Addr
	if host == "" {
		host = paramString(cfg, "host")
	}
	if host == "" {
		return nil, fmt.Errorf("缺少 Redis 地址")
	}
	port := cfg.Port
	if port == 0 {
		port = paramInt(cfg, "port", 6379)
	}
	password := cfg.Password
	if p := paramString(cfg, "password"); p != "" {
		password = p
	}
	db := paramInt(cfg, "db", 0)

	opt := &redis.Options{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		// 运维场景避免网络抖动下写命令被自动重试重复执行（如 LPUSH 重复入队）。
		MaxRetries: 0,
	}
	return &executor{client: redis.NewClient(opt)}, nil
}

// Ping 测试连接。
func (e *executor) Ping(ctx context.Context) error {
	return e.client.Ping(ctx).Err()
}

// Query 执行 Redis 命令并以表格形式返回结果。
// 注意：Redis 命令不是 SQL，但为了复用 execute_sql 工具接口，
// 我们将命令原样发送给 Redis，结果格式化为列+行。Redis 无查询/写语法之分，
// 统一走 Query（Do+formatResult）；Exec 仅保留以满足 QueryRunner 接口。
func (e *executor) Query(ctx context.Context, query string) (*connector.QueryResult, error) {
	args, err := parseCommand(query)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("空命令")
	}

	val, err := e.client.Do(ctx, args...).Result()
	if err != nil {
		if err == redis.Nil {
			// key 不存在（如 GET missing）：以 "(nil)" 标记，避免误判为失败。
			return &connector.QueryResult{
				Columns: []string{"result"},
				Rows:    [][]any{{"(nil)"}},
			}, nil
		}
		return nil, err
	}
	return formatResult(val), nil
}

// Exec 执行写命令（DEL/LPUSH 等），返回受影响数量。
// 仅当命令返回值可解析为整数时提供 rows_affected（如 DEL 的删除数）；
// 其余（如 SET 返回 OK）由统一走 Query 的路径展示。
func (e *executor) Exec(ctx context.Context, query string) (*connector.ExecResult, error) {
	args, err := parseCommand(query)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("空命令")
	}
	n, err := e.client.Do(ctx, args...).Int64()
	if err != nil {
		return nil, err
	}
	return &connector.ExecResult{RowsAffected: n}, nil
}

// Close 关闭底层连接池（会话/面板结束时可调用）。
func (e *executor) Close() error {
	return e.client.Close()
}

// paramString 从 Config.Params 取字符串参数，缺失返回空串。
func paramString(cfg connector.Config, key string) string {
	if v, ok := cfg.Params[key].(string); ok {
		return v
	}
	return ""
}

// paramInt 从 Config.Params 取整数参数（字符串/数值均可），缺失返回 def。
func paramInt(cfg connector.Config, key string, def int) int {
	v, ok := cfg.Params[key]
	if !ok {
		return def
	}
	switch t := v.(type) {
	case string:
		if t == "" {
			return def
		}
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return def
		}
		return n
	case int:
		return t
	case float64:
		return int(t)
	}
	return def
}

// parseCommand 将 "GET foo" / `SET msg "hello world"` 拆分为参数。
// 支持单/双引号包裹含空格的取值，引号内可用反斜杠转义引号。
func parseCommand(input string) ([]any, error) {
	var args []any
	var cur strings.Builder
	var inQuote byte // '"' 或 '\''，0=不在引号内
	started := false

	flush := func() {
		if started {
			args = append(args, cur.String())
			cur.Reset()
			started = false
		}
	}

	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch {
		case inQuote != 0:
			if ch == inQuote {
				inQuote = 0
			} else if ch == '\\' && i+1 < len(input) {
				i++
				cur.WriteByte(input[i])
			} else {
				cur.WriteByte(ch)
			}
		case ch == ' ' || ch == '\t':
			flush()
		case ch == '"' || ch == '\'':
			inQuote = ch
			started = true
		case ch == '\\' && i+1 < len(input):
			i++
			cur.WriteByte(input[i])
			started = true
		default:
			cur.WriteByte(ch)
			started = true
		}
	}
	flush()
	if inQuote != 0 {
		return nil, fmt.Errorf("命令存在未闭合的引号")
	}
	return args, nil
}

// formatResult 将 Redis 返回值格式化为表格。
func formatResult(val any) *connector.QueryResult {
	switch v := val.(type) {
	case string:
		return &connector.QueryResult{
			Columns: []string{"value"},
			Rows:    [][]any{{v}},
		}
	case int64:
		return &connector.QueryResult{
			Columns: []string{"value"},
			Rows:    [][]any{{strconv.FormatInt(v, 10)}},
		}
	case []string:
		rows := make([][]any, 0, len(v))
		for i, s := range v {
			rows = append(rows, []any{i + 1, s})
		}
		return &connector.QueryResult{
			Columns: []string{"index", "value"},
			Rows:    rows,
		}
	case []any:
		rows := make([][]any, 0, len(v))
		for i, item := range v {
			rows = append(rows, []any{i + 1, item})
		}
		return &connector.QueryResult{
			Columns: []string{"index", "value"},
			Rows:    rows,
		}
	case map[string]string:
		// 排序键保证 HGETALL 等结果行序稳定。
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		rows := make([][]any, 0, len(v))
		for _, k := range keys {
			rows = append(rows, []any{k, v[k]})
		}
		return &connector.QueryResult{
			Columns: []string{"key", "value"},
			Rows:    rows,
		}
	default:
		return &connector.QueryResult{
			Columns: []string{"value"},
			Rows:    [][]any{{fmt.Sprintf("%v", v)}},
		}
	}
}
