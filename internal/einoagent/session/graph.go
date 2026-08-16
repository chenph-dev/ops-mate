package session

import (
	"context"
	"errors"
	"fmt"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"ops-mate/internal/connector"
	"ops-mate/internal/einoagent/callback"
	"ops-mate/internal/einoagent/graph"
	"ops-mate/internal/einoagent/history"
	agentmodel "ops-mate/internal/einoagent/model"
	agenttools "ops-mate/internal/einoagent/tools"
	convstore "ops-mate/internal/store/conversations"
)

// run 执行 Graph（首次或 Resume），处理中断/错误/完成。
func (m *SessionManager) run(s *agentSession, ctx context.Context, input []*schema.Message, resume bool, resumeData string) {
	invokeCtx := ctx
	if resume {
		s.mu.Lock()
		id := s.interruptID
		s.mu.Unlock()
		invokeCtx = compose.ResumeWithData(ctx, id, resumeData)
	}

	// 每次 Invoke 挂载审计日志（模型/工具调用完成落库 ai_call_logs，含 token 用量）。
	// 按会话独立创建 handler，使每条记录能关联 sessionID。
	logHandler := callback.NewLogHandler(m.logs, s.id)
	_, err := s.graph.Invoke(invokeCtx, input,
		compose.WithCheckPointID(s.id),
		compose.WithCallbacks(logHandler))

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		if info, ok := compose.ExtractInterruptInfo(err); ok && len(info.InterruptContexts) > 0 {
			s.interruptID = info.InterruptContexts[0].ID
			// 区分审批类型：create_plan 中断的是计划审批，否则是命令审批
			s.approvalType = "command"
			if _, isPlan := info.InterruptContexts[0].Info.(agenttools.PlanInfo); isPlan {
				s.approvalType = "plan"
			}
			s.state = stAwaitingApproval
			m.emitState(s.id, StateAwaitingApproval)
			return
		}
		s.state = stIdle
		s.interruptID = ""
		// 非中断错误也清理 checkpoint：若残留，下一轮 Invoke 会从上一轮
		// 中断/失败点恢复，导致 state 消息翻倍错乱。
		_ = s.checkpoints.Delete(ctx, s.id)
		// 整轮对话结束：清空 tool call 队列，防止取消/中断遗留混入下一轮导致配对错位。
		// （AwaitingApproval 分支不清——当前待审批 tool_call 仍需 Take。）
		s.toolCalls.Reset()
		if errors.Is(err, context.Canceled) {
			m.emitError(s.id, "本次执行已取消", true)
		} else {
			m.emitError(s.id, "AI 对话失败："+err.Error(), false)
		}
		m.emitState(s.id, StateIdle)
		return
	}
	s.state = stIdle
	s.interruptID = ""
	_ = s.checkpoints.Delete(ctx, s.id)
	s.toolCalls.Reset()
	m.emitState(s.id, StateIdle)
}

// ensureGraph 懒构建/按配置版本重建 Graph。
func (m *SessionManager) ensureGraph(s *agentSession) error {
	m.mu.Lock()
	version := m.configVersion
	policyFor := m.policyFor
	skillFor := m.skillFor
	protocolFor := m.protocolFor
	capabilityFor := m.capabilityFor
	m.mu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.graph != nil && s.builtAt == version {
		return nil
	}

	ctx := context.Background()
	cfg, err := m.cfg.GetAIConfig()
	if err != nil {
		return fmt.Errorf("读取 AI 配置失败: %w", err)
	}
	if cfg.Provider == "" {
		return fmt.Errorf("尚未配置 AI 后端，请先到「AI 配置」页设置")
	}
	base, err := m.modelFactory(ctx, cfg)
	if err != nil {
		return fmt.Errorf("构建模型失败: %w", err)
	}

	protocol := "ssh"
	if protocolFor != nil {
		protocol = protocolFor(s.hostID)
	}
	var capability connector.Capability
	if capabilityFor != nil {
		capability = capabilityFor(s.hostID)
	}

	// 命令工具按能力注册：注册表已登记的数据库驱动（IsDB）→ execute_sql；
	// 否则（ssh/winrm 命令型/未知）→ execute_command（sshexec holder 执行器）。
	var commandTool einotool.BaseTool
	if d := connector.Get(protocol); d != nil && d.IsDB() {
		qr, ok := capability.(connector.QueryRunner)
		if !ok || qr == nil {
			return fmt.Errorf("连接类型 %q 的数据库查询能力不可用，请检查资产配置", protocol)
		}
		sqlTool := agenttools.NewSQLTool(s.id, qr, string(d.SkillPack.Guardrail), m.emit, m.convs, s.toolCalls)
		if policyFor != nil {
			auto, _ := policyFor(s.hostID)
			sqlTool.SetApprovalPolicy(auto)
		}
		commandTool = sqlTool
	} else {
		sshTool := agenttools.NewSSHTool(s.id, s.holder, m.emit, m.convs, s.toolCalls)
		// 命令型驱动按注册表 CommandKind 传协议（guardrail 按协议语义判定），
		// 未注册/未知协议传裸 asset protocol。
		cmdKind := protocol
		if d != nil && d.CommandKind != "" {
			cmdKind = d.CommandKind
		}
		sshTool.SetProtocol(cmdKind)
		if policyFor != nil {
			auto, wl := policyFor(s.hostID)
			sshTool.SetApprovalPolicy(auto, wl)
		}
		commandTool = sshTool
	}
	planTool := agenttools.NewPlanTool(s.id, m.emit, m.convs, s.toolCalls)

	// 工具集合：命令工具 + create_plan，注入技能解析器时追加 load_skill / run_skill_script。
	baseTools := []einotool.BaseTool{commandTool, planTool}
	baseInfos := []*schema.ToolInfo{}
	info, err := commandTool.Info(ctx)
	if err != nil {
		return fmt.Errorf("tool info: %w", err)
	}
	planInfo, err := planTool.Info(ctx)
	if err != nil {
		return fmt.Errorf("plan tool info: %w", err)
	}
	baseInfos = append(baseInfos, info, planInfo)
	if skillFor != nil {
		skillTools := agenttools.NewSkillTools(s.id, s.holder, m.emit, m.convs, s.toolCalls, skillFor)
		for _, t := range skillTools.Tools() {
			baseTools = append(baseTools, t)
			ti, err := t.Info(ctx)
			if err != nil {
				return fmt.Errorf("skill tool info: %w", err)
			}
			baseInfos = append(baseInfos, ti)
		}
	}

	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: baseTools, ExecuteSequentially: true,
	})
	if err != nil {
		return fmt.Errorf("构建工具节点失败: %w", err)
	}

	wrapped := agentmodel.NewStreamingChatModel(base, s.id, m.emit, m.onAssistant(s))
	// 全部工具都暴露给模型：execute_command + create_plan +（可选）load_skill / run_skill_script。
	// 遗漏任一个，模型便不知道其存在、永远不会生成对应 tool_call。
	withTools, err := wrapped.WithTools(baseInfos)
	if err != nil {
		return fmt.Errorf("绑定工具失败: %w", err)
	}

	graph, err := graph.BuildAgentGraph(ctx, withTools, toolsNode, s.checkpoints)
	if err != nil {
		return fmt.Errorf("构建 Graph 失败: %w", err)
	}
	s.graph = graph
	s.builtAt = version
	return nil
}

// onAssistant 返回 assistant 消息落库回调（含 tool_calls 配对记录）。
func (m *SessionManager) onAssistant(s *agentSession) func(msg *schema.Message) {
	return func(msg *schema.Message) {
		_ = m.convs.SaveMessage(convstore.Message{
			SessionID: s.id, Role: history.RoleAssistant,
			Content:   msg.Content,
			ToolCalls: history.ToolCallsToJSON(msg.ToolCalls),
		})
		for i := range msg.ToolCalls {
			s.toolCalls.Add(&msg.ToolCalls[i])
		}
	}
}
