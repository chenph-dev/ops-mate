// Package es 注册 Elasticsearch 连接器（KindDB 型，实现 QueryRunner + Pingable）。
// 使用轻量 net/http 客户端（ES 操作均为 REST JSON），复用 connector.QueryRunner 统一语义。
package es

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"ops-mate/internal/connector"
)

// maxResultSize 单次搜索返回的最大命中条数。
const maxResultSize = 100

// executor 实现 connector.QueryRunner + connector.Pingable。
type executor struct {
	client   *http.Client
	baseURL  string
	user     string
	password string
	apiKey   string
}

// NewExecutor 从配置构造 ES 执行器（HTTP 客户端）。
// host/port/password 优先取通用字段（与 mysql/redis 同源）；apiKey/skipVerify 取 Params。
func NewExecutor(cfg connector.Config) (connector.Capability, error) {
	host := cfg.Addr
	if host == "" {
		host = paramString(cfg, "host")
	}
	if host == "" {
		return nil, fmt.Errorf("缺少 Elasticsearch 地址")
	}
	port := cfg.Port
	if port == 0 {
		port = paramInt(cfg, "port", 9200)
	}
	password := cfg.Password
	if p := paramString(cfg, "password"); p != "" {
		password = p
	}
	scheme := "http"
	if s := paramString(cfg, "scheme"); s != "" {
		scheme = s
	}

	tr := &http.Transport{}
	if paramBool(cfg, "skipVerify") {
		// 用户显式配置跳过 HTTPS 证书校验（自签证书场景）。
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &executor{
		client:   &http.Client{Transport: tr, Timeout: 15 * time.Second},
		baseURL:  fmt.Sprintf("%s://%s:%d", scheme, host, port),
		user:     cfg.User,
		password: password,
		apiKey:   paramString(cfg, "apiKey"),
	}, nil
}

// Ping 测试连接：GET / 返回 2xx 即成功。
func (e *executor) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+"/", nil)
	if err != nil {
		return err
	}
	e.auth(req)
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Elasticsearch 连接失败: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Query 执行 ES 命令并返回表格结果。
// 命令格式：首行 REST 路径（可选 "METHOD " 前缀，默认 GET），后续行为 JSON body（存在则 POST）。
// 示例：`_cat/indices`、`_cluster/health`、`logs-*/_search\n{"query":{"match_all":{}}}`。
func (e *executor) Query(ctx context.Context, query string) (*connector.QueryResult, error) {
	cmd, err := parseCommand(query)
	if err != nil {
		return nil, err
	}
	resp, err := e.do(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return formatResponse(resp)
}

// Exec 执行 ES 写操作（POST index/delete/reindex 等），返回受影响数。
func (e *executor) Exec(ctx context.Context, query string) (*connector.ExecResult, error) {
	cmd, err := parseCommand(query)
	if err != nil {
		return nil, err
	}
	if cmd.method == http.MethodGet {
		cmd.method = http.MethodPost // 写操作默认 POST
	}
	resp, err := e.do(ctx, cmd)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Result string `json:"result"`
	}
	if json.Unmarshal(resp, &parsed) == nil {
		switch parsed.Result {
		case "created", "updated", "deleted", "indexed":
			return &connector.ExecResult{RowsAffected: 1}, nil
		}
	}
	return &connector.ExecResult{RowsAffected: 0}, nil
}

// esCommand 解析后的 ES 请求。
type esCommand struct {
	method string
	path   string
	body   string
}

// parseCommand 解析 ES 命令文本（首行路径 + 可选 METHOD 前缀 + 后续 body）。
func parseCommand(query string) (esCommand, error) {
	lines := strings.SplitN(strings.TrimSpace(query), "\n", 2)
	first := strings.TrimSpace(lines[0])
	method, path := http.MethodGet, first
	if i := strings.IndexByte(first, ' '); i > 0 {
		switch strings.ToUpper(first[:i]) {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodHead:
			method, path = strings.ToUpper(first[:i]), strings.TrimSpace(first[i+1:])
		}
	}
	if path == "" {
		return esCommand{}, fmt.Errorf("空命令")
	}
	body := ""
	if len(lines) == 2 {
		body = strings.TrimSpace(lines[1])
	}
	return esCommand{method: method, path: path, body: body}, nil
}

// do 执行 HTTP 请求并读取响应体。
func (e *executor) do(ctx context.Context, cmd esCommand) ([]byte, error) {
	url := e.baseURL + "/" + strings.TrimPrefix(cmd.path, "/")
	var reader io.Reader
	if cmd.body != "" {
		reader = bytes.NewBufferString(cmd.body)
	}
	req, err := http.NewRequestWithContext(ctx, cmd.method, url, reader)
	if err != nil {
		return nil, err
	}
	e.auth(req)
	if cmd.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 500))
	}
	return raw, nil
}

func (e *executor) auth(req *http.Request) {
	if e.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+e.apiKey)
	} else if e.user != "" {
		req.SetBasicAuth(e.user, e.password)
	}
}

// formatResponse 把 ES 响应 JSON 转列+行。
func formatResponse(raw []byte) (*connector.QueryResult, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		// 非 JSON（如纯文本 cat）：单行
		return &connector.QueryResult{
			Columns: []string{"result"},
			Rows:    [][]any{{strings.TrimSpace(string(raw))}},
		}, nil
	}
	switch data := v.(type) {
	case []any: // JSON 数组（_cat/indices?format=json）
		return arrayResult(data), nil
	case map[string]any:
		if hits, ok := extractHits(data); ok { // _search 响应
			return hitsResult(hits), nil
		}
		return objectResult(data), nil
	default:
		return &connector.QueryResult{Columns: []string{"value"}, Rows: [][]any{{string(raw)}}}, nil
	}
}

// extractHits 从 _search 响应提取 hits.hits 数组。
func extractHits(data map[string]any) ([]any, bool) {
	h, ok := data["hits"].(map[string]any)
	if !ok {
		return nil, false
	}
	hh, ok := h["hits"].([]any)
	if !ok {
		return nil, false
	}
	return hh, true
}

// arrayResult JSON 数组 → 列 = 所有对象 key 并集（排序保证稳定），行 = 值。
func arrayResult(arr []any) *connector.QueryResult {
	fieldSet := map[string]bool{}
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for k := range obj {
			fieldSet[k] = true
		}
	}
	cols := make([]string, 0, len(fieldSet))
	for k := range fieldSet {
		cols = append(cols, k)
	}
	sort.Strings(cols)

	rows := make([][]any, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		row := make([]any, len(cols))
		for i, c := range cols {
			row[i] = flat(obj[c])
		}
		rows = append(rows, row)
	}
	return &connector.QueryResult{Columns: cols, Rows: rows}
}

// objectResult JSON 对象 → 单行（列 = key 排序）。
func objectResult(obj map[string]any) *connector.QueryResult {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	row := make([]any, len(keys))
	for i, k := range keys {
		row[i] = flat(obj[k])
	}
	return &connector.QueryResult{Columns: keys, Rows: [][]any{row}}
}

// hitsResult hits 数组 → 列 = _index/_id/_score + _source 字段并集。
func hitsResult(hits []any) *connector.QueryResult {
	var cols []string
	seen := map[string]bool{}
	addCol := func(c string) {
		if !seen[c] {
			seen[c] = true
			cols = append(cols, c)
		}
	}
	// 收集 _source 字段并排序，保证列顺序稳定（map 迭代无序）
	fieldSet := map[string]bool{}
	prepared := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		hit, ok := h.(map[string]any)
		if !ok {
			continue
		}
		src, _ := hit["_source"].(map[string]any)
		for k := range src {
			fieldSet[k] = true
		}
		prepared = append(prepared, hit)
	}
	fields := make([]string, 0, len(fieldSet))
	for k := range fieldSet {
		fields = append(fields, k)
	}
	sort.Strings(fields)

	addCol("_index")
	addCol("_id")
	addCol("_score")
	for _, k := range fields {
		addCol(k)
	}
	rows := make([][]any, 0, len(prepared))
	for _, hit := range prepared {
		row := make([]any, len(cols))
		src, _ := hit["_source"].(map[string]any)
		for i, c := range cols {
			switch c {
			case "_index":
				row[i] = hit["_index"]
			case "_id":
				row[i] = hit["_id"]
			case "_score":
				row[i] = hit["_score"]
			default:
				row[i] = flat(src[c])
			}
		}
		rows = append(rows, row)
	}
	return &connector.QueryResult{Columns: cols, Rows: rows}
}

// flat 把嵌套值转 JSON 字符串（ResultGrid/CSV 兼容），标量原样返回。
func flat(v any) any {
	switch v.(type) {
	case nil, string, float64, bool:
		return v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

// truncate 截断长文本（错误信息用）。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
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
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(t), "%d", &n); err == nil {
			return n
		}
	case int:
		return t
	case float64:
		return int(t)
	}
	return def
}

// paramBool 从 Config.Params 取布尔参数（bool/字符串），缺失返回 false。
func paramBool(cfg connector.Config, key string) bool {
	v, ok := cfg.Params[key]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || t == "1"
	}
	return false
}
