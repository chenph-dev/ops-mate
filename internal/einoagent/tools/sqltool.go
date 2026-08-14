package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/dbexec"
	"ops-mate/internal/einoagent/guardrail"
	"ops-mate/internal/einoagent/history"
	convstore "ops-mate/internal/store/conversations"
)

// SQLTool 把数据库执行器包装为 eino InvokableTool（execute_sql，jdbc 协议）。
// 审批分级：只读查询（SELECT 族）命中自动放行时直连执行；写操作与高危 DDL
// 每条 SQL 先 emit ai:command 再 einotool.Interrupt 等待人工审批。
// Resume 数据 = 批准后的 SQL 文本（可能被用户编辑）或保留字 "rejected"。
type SQLTool struct {
	sessionID string
	executor  *dbexec.Executor // 实际为 per-session 注入的数据库执行器
	emit      func(sessionID, event string, data any)
	convs     *convstore.ConvStore
	holder    *ToolCallHolder

	// 审批分级：enableAuto=true 且 SQL 命中只读关键字时自动放行（不 Interrupt）。
	enableAuto bool
}

// NewSQLTool 构造 SQL 工具。convs 为 nil 时不落库（测试简化）。
func NewSQLTool(
	sessionID string,
	executor *dbexec.Executor,
	emit func(sessionID, event string, data any),
	convs *convstore.ConvStore,
	holder *ToolCallHolder,
) *SQLTool {
	return &SQLTool{
		sessionID: sessionID, executor: executor,
		emit: emit, convs: convs, holder: holder,
	}
}

// SetApprovalPolicy 设置审批分级。enableAuto=false（默认）时全部 SQL 走人工审批。
func (t *SQLTool) SetApprovalPolicy(enableAuto bool) {
	t.enableAuto = enableAuto
}

// Info 返回工具元信息，供 ChatModel 生成 tool call。
func (t *SQLTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "execute_sql",
		Desc: "在目标数据库上执行一条 SQL 语句，先展示给用户审批，批准后才执行。" +
			"优先使用只读查询（SELECT），写操作（INSERT/UPDATE/DELETE/DDL）必须说明影响范围与风险。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"sql": {Type: schema.String, Desc: "要执行的 SQL 语句", Required: true},
			"why": {Type: schema.String, Desc: "执行该 SQL 的原因"},
		}),
	}, nil
}

// InvokableRun 实现全量审批 + 执行（SQL 语义）。
func (t *SQLTool) InvokableRun(ctx context.Context, argsJSON string, _ ...einotool.Option) (string, error) {
	var args struct {
		SQL string `json:"sql"`
		Why string `json:"why"`
	}
	// 契约：永不返回 error——坏参数也转为文本回灌模型，避免整轮对话失败。
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "SQL 参数解析失败：" + err.Error() + "。请重新以 JSON 格式提议 SQL。", nil
	}

	risk, action := guardrail.ClassifyForProtocol(args.SQL, nil, "jdbc")
	riskLabel := risk
	if riskLabel == "" {
		riskLabel = "low"
	}
	info := commandInfo{
		Command: args.SQL, Why: args.Why,
		Risk: riskLabel, AssessedRisk: risk,
	}

	// 恢复调用判定：该工具在本次运行确实被中断过，且 resume 数据匹配当前 SQL。
	wasInterrupted, _, _ := einotool.GetInterruptState[commandInfo](ctx)
	isTarget, hasData, data := einotool.GetResumeContext[string](ctx)
	if wasInterrupted && isTarget && hasData {
		if data == "rejected" {
			content := "用户拒绝了这条 SQL，未执行。请不要重复提议同一条 SQL，换其他方案或向用户询问更多信息。"
			if t.convs != nil {
				t.saveToolMessage(content, "rejected",
					toolMeta{Command: args.SQL, Status: "rejected"})
			}
			return content, nil
		}
		if strings.TrimSpace(data) == "" {
			content := "批准数据为空，SQL 未执行。请重新提议 SQL。"
			if t.convs != nil {
				t.saveToolMessage(content, "rejected",
					toolMeta{Command: args.SQL, Status: "rejected"})
			}
			return content, nil
		}
		return t.execute(ctx, data, "approved")
	}

	// 自动放行：仅首次调用路径生效（恢复路径已 return）。命中只读 → 直连执行。
	if t.enableAuto && action == guardrail.ActionAuto {
		if t.emit != nil {
			t.emit(t.sessionID, "run:auto", map[string]any{"command": args.SQL})
		}
		return t.execute(ctx, args.SQL, "auto")
	}

	if t.emit != nil {
		t.emit(t.sessionID, "ai:command", info)
	}
	return "", einotool.Interrupt(ctx, info)
}

// execute 执行 SQL：推送事件、落库、返回回灌模型的文本（永不返回 error）。
func (t *SQLTool) execute(ctx context.Context, sqlText, approvalStatus string) (string, error) {
	if t.executor == nil {
		return "数据库执行器未配置，无法执行 SQL。请确认资产为 JDBC 协议且凭据可用。", nil
	}
	start := time.Now()
	if t.emit != nil {
		t.emit(t.sessionID, "run:start", map[string]any{"command": sqlText})
	}

	var res *dbexec.Result
	var execErr error
	if dbexec.IsQuery(sqlText) {
		res, execErr = t.executor.Query(ctx, sqlText)
	} else {
		res, execErr = t.executor.Exec(ctx, sqlText)
	}
	cancelled := ctx.Err() != nil
	display := formatSQLResult(res, execErr, cancelled)
	meta := toolMeta{
		Command: sqlText, DurationMS: time.Since(start).Milliseconds(),
		Status: approvalStatus, Cancelled: cancelled,
	}

	if t.convs != nil {
		t.saveToolMessage(display, approvalStatus, meta)
		if err := t.convs.SaveCommand(t.sessionID, sqlText, 0, display); err != nil {
			log.Printf("einoagent: save sql command record: %v", err)
		}
	}

	if t.emit != nil {
		errStr := ""
		if execErr != nil {
			errStr = execErr.Error()
		}
		t.emit(t.sessionID, "run:result", map[string]any{
			"command":   sqlText,
			"output":    display,
			"exitCode":  0,
			"error":     errStr,
			"cancelled": cancelled,
		})
	}

	return truncateForModel(display), nil
}

// saveToolMessage 落库一条 execute_sql 的 tool 消息（配对 tool_call_id）。
func (t *SQLTool) saveToolMessage(content, approvalStatus string, meta toolMeta) {
	toolCallID := ""
	if tc := t.holder.Take(); tc != nil {
		toolCallID = tc.ID
	}
	metaJSON, _ := json.Marshal(meta)
	if err := t.convs.SaveMessage(convstore.Message{
		SessionID:      t.sessionID,
		Role:           history.RoleTool,
		Content:        content,
		ToolCallID:     toolCallID,
		ToolName:       "execute_sql",
		ApprovalStatus: approvalStatus,
		ToolResult:     string(metaJSON),
	}); err != nil {
		log.Printf("einoagent: save sql tool message: %v", err)
	}
}

// formatSQLResult 把结构化结果序列化为文本回灌模型/展示：
// 查询类输出列名 + 制表符分隔行；写类输出受影响行数。
func formatSQLResult(res *dbexec.Result, execErr error, cancelled bool) string {
	var b strings.Builder
	switch {
	case cancelled:
		b.WriteString("SQL 执行被取消。")
	case execErr != nil:
		fmt.Fprintf(&b, "SQL 执行失败：%v", execErr)
	case res == nil:
		return ""
	default:
		if len(res.Columns) > 0 {
			b.WriteString(strings.Join(res.Columns, "\t"))
			b.WriteString("\n")
			for _, row := range res.Rows {
				parts := make([]string, len(row))
				for i, v := range row {
					parts[i] = fmt.Sprintf("%v", v)
				}
				b.WriteString(strings.Join(parts, "\t"))
				b.WriteString("\n")
			}
		}
		if res.RowsAffected > 0 {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "[rows_affected=%d]", res.RowsAffected)
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}
