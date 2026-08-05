// Package graph 提供 eino Agent Graph 的构建（llm ↔ tools 条件分支循环 + checkpoint）。
package graph

import (
	"context"
	"fmt"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// agentState 是 Graph 的每运行本地状态，仅用于累积消息序列。
// 说明：eino v0.10.0-alpha.13 的 ChatModelNode 输出类型是 *schema.Message，
// Graph 不会自动把节点输出追加到 []*schema.Message，
// 因此回灌循环的完整历史（用户消息 + assistant 提议 + tool 结果）
// 必须由状态显式累积——这与 eino 官方 flow/agent/react 的做法一致。
// 状态会随 checkpoint 持久化/恢复，Resume 时中断点之前的消息不丢。
// 注册见下方 init——eino checkpoint 序列化要求显式注册自定义类型。
type agentState struct {
	Messages []*schema.Message
}

func init() {
	// checkpoint 序列化要求 state 类型注册（否则 Interrupt 时 marshal 报 unknown type）。
	schema.RegisterName[*agentState]("ops_mate_agent_state")
}

// BuildAgentGraph 构建 ops-mate Agent 的 eino Graph。
//
// 拓扑：
//
//	START → llm(ChatModelNode) → Branch:
//	  消息有 ToolCalls → tools(ToolsNode) → llm（回灌循环）
//	  无 ToolCalls → finish(Lambda，取 state 累积的完整消息) → END
//
// 注意三点（由 eino v0.10.0-alpha.13 API 决定）：
//  1. 分支条件接收 *schema.Message（ChatModelNode 的输出类型），不是 []*schema.Message；
//  2. llm 输出 *schema.Message 与 Graph 输出 []*schema.Message 类型不同构，
//     不能直连 END，须经 finish 节点从 state 取出完整消息序列。
//  3. Resume 调用（带 compose.ResumeWithData 的 ctx）时，Invoke 的 input 参数被 eino 忽略——执行由恢复的 checkpoint 驱动，调用方不要指望通过 resume 调用传入新用户消息。
//
// 审批在 SSHTool 内部以 tool.Interrupt 实现；
// Resume 依赖 checkpoint，故编译选项必须带 WithCheckPointStore，
// 调用方每次 Invoke（含 Resume）必须带 WithCheckPointID(sessionID)。
func BuildAgentGraph(
	ctx context.Context,
	chatModel einomodel.BaseChatModel,
	toolsNode *compose.ToolsNode,
	ckptStore compose.CheckPointStore,
) (compose.Runnable[[]*schema.Message, []*schema.Message], error) {
	g := compose.NewGraph[[]*schema.Message, []*schema.Message](
		compose.WithGenLocalState(func(ctx context.Context) *agentState {
			return &agentState{}
		}),
	)

	// pre：把上游输入（首轮为会话输入，回灌轮为 tool 结果）累积进 state，
	// 用完整历史喂给模型；post：把 assistant 回复累积进 state。
	modelPreHandle := func(ctx context.Context, input []*schema.Message, state *agentState) ([]*schema.Message, error) {
		state.Messages = append(state.Messages, input...)
		return state.Messages, nil
	}
	modelPostHandle := func(ctx context.Context, output *schema.Message, state *agentState) (*schema.Message, error) {
		state.Messages = append(state.Messages, output)
		return output, nil
	}

	if err := g.AddChatModelNode("llm", chatModel,
		compose.WithStatePreHandler(modelPreHandle),
		compose.WithStatePostHandler(modelPostHandle),
	); err != nil {
		return nil, fmt.Errorf("add llm node: %w", err)
	}
	if err := g.AddToolsNode("tools", toolsNode); err != nil {
		return nil, fmt.Errorf("add tools node: %w", err)
	}
	if err := g.AddEdge(compose.START, "llm"); err != nil {
		return nil, fmt.Errorf("add start edge: %w", err)
	}

	// finish：从 state 取完整消息序列作为 Graph 输出。
	finish := compose.InvokableLambda(func(ctx context.Context, _ *schema.Message) ([]*schema.Message, error) {
		var msgs []*schema.Message
		if err := compose.ProcessState(ctx, func(ctx context.Context, s *agentState) error {
			msgs = s.Messages
			return nil
		}); err != nil {
			return nil, err
		}
		return msgs, nil
	})
	if err := g.AddLambdaNode("finish", finish); err != nil {
		return nil, fmt.Errorf("add finish node: %w", err)
	}

	// 条件分支：修复旧骨架"无条件路由到 tools"导致的死循环。
	branch := compose.NewGraphBranch(
		func(ctx context.Context, msg *schema.Message) (string, error) {
			if msg != nil && len(msg.ToolCalls) > 0 {
				return "tools", nil
			}
			return "finish", nil
		},
		map[string]bool{"tools": true, "finish": true},
	)
	if err := g.AddBranch("llm", branch); err != nil {
		return nil, fmt.Errorf("add branch: %w", err)
	}
	if err := g.AddEdge("tools", "llm"); err != nil {
		return nil, fmt.Errorf("add loop edge: %w", err)
	}
	if err := g.AddEdge("finish", compose.END); err != nil {
		return nil, fmt.Errorf("add finish edge: %w", err)
	}

	return g.Compile(ctx,
		compose.WithGraphName("ops_mate_agent"),
		compose.WithMaxRunSteps(50),
		compose.WithCheckPointStore(ckptStore),
	)
}
