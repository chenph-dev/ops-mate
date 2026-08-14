package tools

import (
	"context"
	"strings"
	"testing"

	"ops-mate/internal/dbexec"
	"ops-mate/internal/einoagent/testutil"
)

func TestSQLTool_InfoDesc(t *testing.T) {
	tool := NewSQLTool("s1", nil, nil, nil, NewToolCallHolder())
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "execute_sql" || !strings.Contains(info.Desc, "SQL") {
		t.Errorf("工具信息错误: name=%q desc=%q", info.Name, info.Desc)
	}
}

func TestSQLTool_BadArgsReturnsText(t *testing.T) {
	tool := NewSQLTool("s1", nil, nil, nil, NewToolCallHolder())
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
	tool := NewSQLTool("s1", nil, rec.Emit, nil, NewToolCallHolder())
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
	tool := NewSQLTool("s1", nil, rec.Emit, nil, NewToolCallHolder())
	tool.SetApprovalPolicy(true)
	got, err := tool.InvokableRun(context.Background(), `{"sql":"SELECT 1"}`)
	if err != nil {
		t.Fatalf("只读 SQL 自动放行不应返回中断错误: %v", err)
	}
	// executor 为 nil → 回灌"未配置"提示文本（不 panic、不返回 error）
	if !strings.Contains(got, "未配置") {
		t.Errorf("executor 未配置应回灌提示，得到 %q", got)
	}
	if testutil.ContainsEvent(rec.SnapshotEvents(), "ai:command") {
		t.Error("自动放行不应推送 ai:command")
	}
	if !testutil.ContainsEvent(rec.SnapshotEvents(), "run:auto") {
		t.Error("自动放行应推送 run:auto")
	}
}

func TestSQLTool_WriteStillInterruptsEvenAuto(t *testing.T) {
	rec := &testutil.EmitRecorder{}
	tool := NewSQLTool("s1", nil, rec.Emit, nil, NewToolCallHolder())
	tool.SetApprovalPolicy(true)
	if _, err := tool.InvokableRun(context.Background(), `{"sql":"UPDATE users SET name='x'"}`); err == nil {
		t.Fatal("写 SQL 即使启用自动放行也应中断")
	}
	if !testutil.ContainsEvent(rec.SnapshotEvents(), "ai:command") {
		t.Error("写 SQL 应推送 ai:command 审批卡")
	}
}

func TestFormatSQLResult(t *testing.T) {
	res := &dbexec.Result{Columns: []string{"id", "name"}, Rows: [][]any{{int64(1), "a"}, {int64(2), "b"}}}
	if got := formatSQLResult(res, nil, false); got != "id\tname\n1\ta\n2\tb" {
		t.Errorf("查询结果序列化错误: %q", got)
	}
	if got := formatSQLResult(&dbexec.Result{RowsAffected: 3}, nil, false); got != "[rows_affected=3]" {
		t.Errorf("写结果序列化错误: %q", got)
	}
	if got := formatSQLResult(nil, nil, true); !strings.Contains(got, "取消") {
		t.Errorf("取消应提示，得到 %q", got)
	}
	if got := formatSQLResult(nil, context.DeadlineExceeded, false); !strings.Contains(got, "执行失败") {
		t.Errorf("错误应提示，得到 %q", got)
	}
}
