package einoagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/sshexec"
	convstore "ops-mate/internal/store/conversations"
)

const (
	// modelOutputLimit 回灌模型的输出上限（防上下文爆炸）。
	modelOutputLimit = 8 * 1024
	// displayOutputLimit 前端展示与落库的输出上限。
	displayOutputLimit = 64 * 1024
)

// commandInfo 审批卡数据：推送给前端，同时作为 Interrupt info。
type commandInfo struct {
	Command      string `json:"command"`
	Why          string `json:"why"`
	Risk         string `json:"risk"`
	AssessedRisk string `json:"assessedRisk"`
}

// toolCallHolder 按先进先出保存待执行的 tool call 队列，
// 供 SSHTool 落库 tool 消息时配对 tool_call_id。
// 由 StreamingChatModel 的 onAssistant 回调写入（一次模型回复可含多个 tool_calls）。
// ToolsNode 按 tool_calls 顺序逐个执行（ExecuteSequentially），审批/恢复也逐个进行，
// 因此 Take() 取出的顺序与工具执行顺序一致，保证每个 tool 消息配对正确的 tool_call_id。
type toolCallHolder struct {
	mu  sync.Mutex
	tcs []*schema.ToolCall
}

func newToolCallHolder() *toolCallHolder { return &toolCallHolder{} }

// Add 追加一个待执行的 tool call。
func (h *toolCallHolder) Add(tc *schema.ToolCall) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tcs = append(h.tcs, tc)
}

// Take 弹出并返回队首的 tool call；队列为空返回 nil。
func (h *toolCallHolder) Take() *schema.ToolCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.tcs) == 0 {
		return nil
	}
	tc := h.tcs[0]
	h.tcs = h.tcs[1:]
	return tc
}

// SSHTool 把 SSH 执行器包装为 eino InvokableTool。
// 全量审批：每条命令（含低风险）都先 emit ai:command 再 tool.Interrupt，
// Resume 数据 = 批准命令字符串（可能被用户编辑）或 "rejected"。
type SSHTool struct {
	sessionID string
	executor  sshexec.Exec // 实际为 per-session executorHolder
	emit      func(sessionID, event string, data any)
	convs     *convstore.ConvStore
	holder    *toolCallHolder
}

// NewSSHTool 构造 SSH 工具。convs 为 nil 时不落库（测试简化）。
func NewSSHTool(
	sessionID string,
	executor sshexec.Exec,
	emit func(sessionID, event string, data any),
	convs *convstore.ConvStore,
	holder *toolCallHolder,
) *SSHTool {
	return &SSHTool{
		sessionID: sessionID, executor: executor,
		emit: emit, convs: convs, holder: holder,
	}
}

// Info 返回工具元信息，供 ChatModel 生成 tool call。
func (t *SSHTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "execute_command",
		Desc: "在目标 Linux 主机上执行一条 Shell 命令。命令会先展示给用户审批，批准后才执行。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {Type: schema.String, Desc: "要执行的 Shell 命令", Required: true},
			"why":     {Type: schema.String, Desc: "执行该命令的原因"},
		}),
	}, nil
}

// InvokableRun 实现全量审批 + 执行。
func (t *SSHTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Command string `json:"command"`
		Why     string `json:"why"`
	}
	// 契约：永不返回 error——坏参数也转为文本回灌模型，避免整轮对话失败。
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "命令参数解析失败：" + err.Error() + "。请重新以 JSON 格式提议命令。", nil
	}

	risk := AssessRisk(args.Command)
	// AssessedRisk 是守卫的原始判定（"" = 干净）；Risk 是展示标签（"" 归一为 "low"）。
	riskLabel := risk
	if riskLabel == "" {
		riskLabel = "low"
	}
	info := commandInfo{
		Command: args.Command, Why: args.Why,
		Risk: riskLabel, AssessedRisk: risk,
	}

	wasInterrupted, _, _ := tool.GetInterruptState[commandInfo](ctx)
	if !wasInterrupted {
		// 首次调用：任意风险等级都中断等审批（全量审批）。
		if t.emit != nil {
			t.emit(t.sessionID, "ai:command", info)
		}
		return "", tool.Interrupt(ctx, info)
	}

	// 恢复路径
	isTarget, hasData, data := tool.GetResumeContext[string](ctx)
	if !isTarget || !hasData {
		// 不是本次审批目标（或无数据）：重新中断，让 Resume 信号传播到正确位置。
		return "", tool.Interrupt(ctx, info)
	}
	// 协议约定：resume 数据为保留字 "rejected" 表示拒绝，其余字符串视为（可能被用户编辑过的）待执行命令。
	if data == "rejected" {
		content := "用户拒绝了这条命令，未执行。请不要重复提议同一条命令，换其他方案或向用户询问更多信息。"
		if t.convs != nil {
			// 拒绝也必须落库 tool 消息，否则 assistant 的 tool_calls 没有配对结果，
			// 历史回放时模型会拒绝整个会话。
			t.saveToolMessage(content, "rejected")
		}
		return content, nil
	}
	if strings.TrimSpace(data) == "" {
		// 批准数据为空同样落库配对结果，避免产生孤立 tool_calls。
		content := "批准数据为空，命令未执行。请重新提议命令。"
		if t.convs != nil {
			t.saveToolMessage(content, "rejected")
		}
		return content, nil
	}
	// data = 批准后的最终命令（用户可能编辑过）
	return t.execute(ctx, data)
}

// execute 执行命令：推送事件、落库、返回回灌模型的文本（永不返回 error，
// 执行失败也转为文本回灌，让 AI 看到并可换方案）。
func (t *SSHTool) execute(ctx context.Context, command string) (string, error) {
	if t.emit != nil {
		t.emit(t.sessionID, "run:start", map[string]any{"command": command})
	}

	output, exitCode, execErr := runCommand(ctx, t.executor, command)
	cancelled := ctx.Err() != nil
	display := truncateForDisplay(output)

	if t.emit != nil {
		errStr := ""
		if execErr != nil {
			errStr = execErr.Error()
		}
		t.emit(t.sessionID, "run:result", map[string]any{
			"command":   command,
			"output":    display,
			"exitCode":  exitCode,
			"error":     errStr,
			"cancelled": cancelled,
		})
	}

	var content string
	switch {
	case cancelled:
		content = "命令被取消。已产生的输出：\n" + display
	case execErr != nil:
		content = "命令执行失败：" + execErr.Error()
	default:
		content = display
		if exitCode != 0 {
			content += fmt.Sprintf("\n[exit_code=%d]", exitCode)
		}
	}

	// 落库保留较完整输出（≤64KB 展示截断），回灌模型用 8KB 截断——历史回放时模型看到的 tool 结果可能比原轮次更多，属预期。
	if t.convs != nil {
		t.saveToolMessage(content, "approved")
		if err := t.convs.SaveCommand(t.sessionID, command, exitCode, display); err != nil {
			log.Printf("einoagent: save command record: %v", err)
		}
	}

	return truncateForModel(content), nil
}

// saveToolMessage 落库一条 tool 消息，配对当前 Take 出的 tool call，
// 并记录审批状态（"approved" 已批准执行 / "rejected" 已拒绝）。
// 批准执行、拒绝、空批准数据等所有结束审批的路径都必须调用，
// 否则 assistant 的 tool_calls 在历史中孤立，模型端会拒绝整个请求。
func (t *SSHTool) saveToolMessage(content, approvalStatus string) {
	toolCallID := ""
	if tc := t.holder.Take(); tc != nil {
		toolCallID = tc.ID
	}
	if err := t.convs.SaveMessage(convstore.Message{
		SessionID:      t.sessionID,
		Role:           RoleTool,
		Content:        content,
		ToolCallID:     toolCallID,
		ToolName:       "execute_command",
		ApprovalStatus: approvalStatus,
	}); err != nil {
		log.Printf("einoagent: save tool message: %v", err)
	}
}

// runCommand 收集命令输出。返回：输出文本（截断至 displayOutputLimit）、退出码（-1 = 未知/失败/取消）、执行层错误。
// sshexec 仅在非零退出时发 {Stream:"exit", Text:"exit_code=N"} 行。
// 注意：超出上限后停止累积但必须继续排空通道（拿 exit 行 + 避免执行器管道协程阻塞）。
func runCommand(ctx context.Context, ex sshexec.Exec, command string) (string, int, error) {
	if ex == nil {
		return "", -1, fmt.Errorf("执行器未配置")
	}
	ch, err := ex.Exec(ctx, command)
	if err != nil {
		return "", -1, err
	}
	var sb strings.Builder
	exitCode := 0
	for ln := range ch {
		if ln.Stream == "exit" {
			if n, perr := strconv.Atoi(strings.TrimPrefix(ln.Text, "exit_code=")); perr == nil {
				exitCode = n
			}
			continue
		}
		if sb.Len() < displayOutputLimit {
			sb.WriteString(ln.Text)
			sb.WriteString("\n")
		}
	}
	if ctx.Err() != nil {
		return sb.String(), -1, nil
	}
	return sb.String(), exitCode, nil
}

// runeSafeCut 把字节下标回退到 rune 边界，避免截断多字节字符。
func runeSafeCut(s string, n int) int {
	if n >= len(s) {
		return len(s)
	}
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return n
}

// truncateForModel 回灌模型截断：保留头尾各一半，中间省略。
func truncateForModel(s string) string {
	if len(s) <= modelOutputLimit {
		return s
	}
	half := modelOutputLimit / 2
	return s[:runeSafeCut(s, half)] +
		"\n...[输出过长，已省略 " + strconv.Itoa(len(s)-modelOutputLimit) + " 字节]...\n" +
		s[runeSafeCut(s, len(s)-half):]
}

// truncateForDisplay 展示/落库截断：保留头部。
func truncateForDisplay(s string) string {
	if len(s) <= displayOutputLimit {
		return s
	}
	return s[:runeSafeCut(s, displayOutputLimit)] + "\n...[输出过长，已截断]"
}
