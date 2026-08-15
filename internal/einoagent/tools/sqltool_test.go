package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ops-mate/internal/connector"
	"ops-mate/internal/einoagent/testutil"
)

// fakeSQLRunner 实现 connector.QueryRunner，供 SQLTool 执行测试。
type fakeSQLRunner struct {
	queryRes *connector.QueryResult
	execRes  *connector.ExecResult
	queryErr error
	execErr  error
}

func (f *fakeSQLRunner) Query(_ context.Context, _ string) (*connector.QueryResult, error) {
	return f.queryRes, f.queryErr
}
func (f *fakeSQLRunner) Exec(_ context.Context, _ string) (*connector.ExecResult, error) {
	return f.execRes, f.execErr
}

func TestSQLTool_InfoDesc(t *testing.T) {
	tool := NewSQLTool("s1", nil, "sql", nil, nil, NewToolCallHolder())
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "execute_sql" || !strings.Contains(info.Desc, "SQL") {
		t.Errorf("工具信息错误: name=%q desc=%q", info.Name, info.Desc)
	}
}

func TestSQLTool_BadArgsReturnsText(t *testing.T) {
	tool := NewSQLTool("s1", nil, "sql", nil, nil, NewToolCallHolder())
	got, err := tool.InvokableRun(context.Background(), "{bad json")
	if err != nil {
		t.Fatalf("坏 JSON 不应返回 error: %v", err)
	}
	if !strings.Contains(got, "参数解析失败") {
		t.Errorf("期望参数解析失败提示，得到 %q", got)
	}
}

func TestSQLTool_FirstCallInterrupts(t *testing.T) {
	rec := &testutil.EmitRecorder{}
	tool := NewSQLTool("s1", nil, "sql", rec.Emit, nil, NewToolCallHolder())
	_, err := tool.InvokableRun(context.Background(), `{"sql":"UPDATE users SET name='x'","why":"改名"}`)
	if err == nil {
		t.Fatal("写 SQL 首次调用应返回中断错误")
	}
	if !testutil.ContainsEvent(rec.SnapshotEvents(), "ai:command") {
		t.Error("写 SQL 应推送 ai:command 审批卡")
	}
}

func TestSQLTool_AutoReadOnlyRuns(t *testing.T) {
	rec := &testutil.EmitRecorder{}
	tool := NewSQLTool("s1", nil, "sql", rec.Emit, nil, NewToolCallHolder())
	tool.SetApprovalPolicy(true)
	got, err := tool.InvokableRun(context.Background(), `{"sql":"SELECT 1"}`)
	if err != nil {
		t.Fatalf("只读 SQL 自动放行不应返回中断错误: %v", err)
	}
	// runner 为 nil → 回灌"未配置"提示文本（不 panic、不返回 error）
	if !strings.Contains(got, "未配置") {
		t.Errorf("runner 未配置应回灌提示，得到 %q", got)
	}
	if testutil.ContainsEvent(rec.SnapshotEvents(), "ai:command") {
		t.Error("自动放行不应推送 ai:command")
	}
	if !testutil.ContainsEvent(rec.SnapshotEvents(), "run:auto") {
		t.Error("自动放行应推送 run:auto")
	}
}

func TestSQLTool_AutoQueryUsesRunner(t *testing.T) {
	rec := &testutil.EmitRecorder{}
	runner := &fakeSQLRunner{
		queryRes: &connector.QueryResult{Columns: []string{"id"}, Rows: [][]any{{int64(1)}}},
	}
	tool := NewSQLTool("s1", runner, "sql", rec.Emit, nil, NewToolCallHolder())
	tool.SetApprovalPolicy(true)
	got, err := tool.InvokableRun(context.Background(), `{"sql":"SELECT 1","why":"验证"}`)
	if err != nil {
		t.Fatalf("只读 SQL 自动放行不应返回中断错误: %v", err)
	}
	if !strings.Contains(got, "id") || !strings.Contains(got, "1") {
		t.Errorf("应回灌查询结果列与行，得到 %q", got)
	}
}

func TestSQLTool_WriteStillInterruptsEvenAuto(t *testing.T) {
	rec := &testutil.EmitRecorder{}
	tool := NewSQLTool("s1", nil, "sql", rec.Emit, nil, NewToolCallHolder())
	tool.SetApprovalPolicy(true)
	if _, err := tool.InvokableRun(context.Background(), `{"sql":"UPDATE users SET name='x'"}`); err == nil {
		t.Fatal("写 SQL 即使启用自动放行也应中断")
	}
	if !testutil.ContainsEvent(rec.SnapshotEvents(), "ai:command") {
		t.Error("写 SQL 应推送 ai:command 审批卡")
	}
}

func TestSQLTool_ExecWritesResult(t *testing.T) {
	runner := &fakeSQLRunner{execRes: &connector.ExecResult{RowsAffected: 3}}
	got, err := (&SQLTool{runner: runner, guardrailProto: "sql"}).execute(context.Background(), "UPDATE users SET name='x'", "approved")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(got, "[rows_affected=3]") {
		t.Errorf("写结果应回灌受影响行数，得到 %q", got)
	}
}

func TestSQLTool_ExecErrReachesRunResult(t *testing.T) {
	var resultErr string
	emit := func(_sid, event string, data any) {
		if event == "run:result" {
			if m, ok := data.(map[string]any); ok {
				resultErr, _ = m["error"].(string)
			}
		}
	}
	tool := NewSQLTool("s1", &fakeSQLRunner{execErr: errors.New("boom")}, "sql", emit, nil, NewToolCallHolder())
	got, err := tool.execute(context.Background(), "UPDATE users SET name='x'", "approved")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(got, "执行失败") || !strings.Contains(got, "boom") {
		t.Errorf("写执行失败应回灌错误，得到 %q", got)
	}
	if resultErr != "boom" {
		t.Errorf("run:result 的 error 字段应为 boom，得到 %q", resultErr)
	}
}

func TestFormatQueryResult(t *testing.T) {
	res := &connector.QueryResult{Columns: []string{"id", "name"}, Rows: [][]any{{int64(1), "a"}, {int64(2), "b"}}}
	if got := formatQueryResult(res, nil, false); got != "id\tname\n1\ta\n2\tb" {
		t.Errorf("查询结果序列化错误: %q", got)
	}
	if got := formatQueryResult(nil, nil, true); !strings.Contains(got, "取消") {
		t.Errorf("取消应提示，得到 %q", got)
	}
	if got := formatQueryResult(nil, context.DeadlineExceeded, false); !strings.Contains(got, "执行失败") {
		t.Errorf("错误应提示，得到 %q", got)
	}
	if got := formatQueryResult(nil, nil, false); got != "" {
		t.Errorf("空结果应为空串，得到 %q", got)
	}
	if got := formatQueryResult(&connector.QueryResult{Columns: []string{"id"}}, nil, false); got != "id" {
		t.Errorf("无行时仅输出表头，得到 %q", got)
	}
}

func TestFormatExecResult(t *testing.T) {
	if got := formatExecResult(&connector.ExecResult{RowsAffected: 3}, nil, false); got != "[rows_affected=3]" {
		t.Errorf("写结果序列化错误: %q", got)
	}
	if got := formatExecResult(nil, nil, false); got != "" {
		t.Errorf("空结果应为空串，得到 %q", got)
	}
	if got := formatExecResult(nil, nil, true); !strings.Contains(got, "取消") {
		t.Errorf("取消应提示，得到 %q", got)
	}
	if got := formatExecResult(nil, errors.New("boom"), false); !strings.Contains(got, "执行失败") {
		t.Errorf("错误应提示，得到 %q", got)
	}
}
