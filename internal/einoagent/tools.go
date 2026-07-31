package einoagent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/sshexec"
)

// commandInfo 传递给 Interrupt 的前端信息。
type commandInfo struct {
	Command      string `json:"command"`
	Why          string `json:"why"`
	Risk         string `json:"risk"`
	AssessedRisk string `json:"assessedRisk"`
}

// SSHTool 将 ops-mate 的 SSH 执行器包装为 eino InvokableTool。
//
// 审批流完全在 InvokableRun 内部通过 tool.Interrupt 实现：
//   - 首次调用：高风险 → Interrupt（暂停 Graph，通知前端）
//   - 恢复后：检查 ResumeData → 批准则执行，拒绝则返回提示
//
// 这符合 eino 设计：工具自行决定是否暂停，而非外部节点控制。
type SSHTool struct {
	executor  sshexec.Exec
	emit      func(sessionID, event string, data any)
	sessionID string
}

// NewSSHTool 构造 SSH 工具。
func NewSSHTool(sessionID string, executor sshexec.Exec, emit func(string, string, any)) *SSHTool {
	return &SSHTool{
		executor:  executor,
		emit:      emit,
		sessionID: sessionID,
	}
}

// Info 返回工具元信息，供 ChatModel 生成 tool call。
func (t *SSHTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "execute_command",
		Desc: "在目标 Linux 主机上执行一条 Shell 命令。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {Type: schema.String, Desc: "要执行的命令", Required: true},
			"why":     {Type: schema.String, Desc: "执行该命令的原因"},
		}),
	}, nil
}

// InvokableRun 实现审批流 + 命令执行。
//
// 流程：
//  1. 解析参数
//  2. 首次调用（非恢复）且高风险 → tool.Interrupt（暂停等审批）
//  3. 恢复后 → 检查 ResumeData：批准→执行，拒绝→返回提示
func (t *SSHTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Command string `json:"command"`
		Why     string `json:"why"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", err
	}

	// 检查是否是恢复后的调用
	wasInterrupted, _, _ := tool.GetInterruptState[commandInfo](ctx)

	if !wasInterrupted {
		// 首次调用
		risk := AssessRisk(args.Command)
		if risk == "high" {
			// 高风险 → Interrupt 等审批
			info := commandInfo{
				Command:      args.Command,
				Why:          args.Why,
				Risk:         "high",
				AssessedRisk: risk,
			}
			t.emit(t.sessionID, "ai:command", info)
			t.emit(t.sessionID, "session:state", "AwaitingApproval")
			return "", tool.Interrupt(ctx, info)
		}
		// 低风险 → 直接执行
		return t.execute(ctx, args.Command)
	}

	// 恢复后 → 检查是否是本次审批目标
	isTarget, hasData, resumeData := tool.GetResumeContext[string](ctx)
	if !isTarget {
		// 不是本次目标，重新 Interrupt
		info := commandInfo{Command: args.Command, Why: args.Why, Risk: "high"}
		return "", tool.Interrupt(ctx, info)
	}

	if hasData && resumeData == "rejected" {
		return "用户拒绝了这条命令，未执行。", nil
	}

	// 批准 → 执行
	return t.execute(ctx, args.Command)
}

func (t *SSHTool) execute(ctx context.Context, command string) (string, error) {
	if t.executor == nil {
		return "", fmt.Errorf("执行器未配置")
	}
	ch, err := t.executor.Exec(ctx, command)
	if err != nil {
		return "", err
	}
	var output string
	for ln := range ch {
		output += ln.Text + "\n"
	}
	return output, nil
}
