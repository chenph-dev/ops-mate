package einoagent

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"

	"ops-mate/internal/sshexec"
	"ops-mate/internal/store"
)

// GraphState 通过 eino Graph 的 WithGenLocalState 在节点间共享。
// 一个 session 对应一次 Graph 执行，State 贯穿整个生命周期。
type GraphState struct {
	SessionID string
	HostID    string
	History   []Message
	Executor  sshexec.Exec
	Emit      func(sessionID, event string, data any)

	// 审批流中间状态
	PendingCommand *CommandSuggestion
	RiskLevel      string
	Rejected       bool // 刚被拒绝，需回到 ChatModel 换方案
}

// NewGraphState 构造 Graph 初始状态。
func NewGraphState(sessionID, hostID string, executor sshexec.Exec, emit func(string, string, any)) *GraphState {
	return &GraphState{
		SessionID: sessionID,
		HostID:    hostID,
		History:   []Message{},
		Executor:  executor,
		Emit:      emit,
	}
}

// BuildAgentGraph 构建 ops-mate Agent 的 eino Graph。
//
// 拓扑（遵循 eino 原生 tool calling 模式）：
//
//	START → InjectMemory → ChatModel → [has tool calls?]
//	                                        │
//	                            ┌───────────┼───────────┐
//	                            │ Yes        │ No
//	                            ▼            ▼
//	                       ToolsNode        END
//	                            │
//	                            └──────→ ChatModel (loop)
//
// 审批流完全在 SSHTool.InvokableRun 内部通过 tool.Interrupt 实现。
// ChatModel 需通过 WithTools(tools) 绑定相同的 tools，使模型能生成 tool calls。
func BuildAgentGraph(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	tools []tool.BaseTool,
	st *store.Store,
) (compose.Runnable[*GraphState, *GraphState], error) {
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{Tools: tools})
	if err != nil {
		return nil, err
	}

	g := compose.NewGraph[*GraphState, *GraphState](
		compose.WithGenLocalState(func(ctx context.Context) *GraphState {
			return &GraphState{}
		}),
	)

	// --- 节点 ---
	g.AddLambdaNode("inject_memory", compose.InvokableLambda(func(ctx context.Context, state *GraphState) (*GraphState, error) {
		return InjectMemoryNode(ctx, state, st)
	}))
	g.AddChatModelNode("llm", chatModel)
	g.AddToolsNode("tools", toolsNode)

	// --- 边 ---
	g.AddEdge(compose.START, "inject_memory")
	g.AddEdge("inject_memory", "llm")

	// llm → tools（有 tool calls 时）或 END（无 tool calls 时）
	// eino 的 ChatModelNode + Branch 自动根据 tool calls 存在与否路由
	g.AddBranch("llm", compose.NewGraphBranch(
		func(ctx context.Context, state *GraphState) (string, error) {
			return "tools", nil
		},
		map[string]bool{"tools": true},
	))

	// tools 执行完 → 回到 llm（回灌结果）
	g.AddEdge("tools", "llm")

	return g.Compile(ctx,
		compose.WithGraphName("ops_mate_agent"),
		compose.WithMaxRunSteps(50),
	)
}

// InjectMemoryNode 从 FTS5 召回历史命令，注入到 history 中。
func InjectMemoryNode(ctx context.Context, state *GraphState, st *store.Store) (*GraphState, error) {
	if st == nil {
		return state, nil
	}
	recall, err := st.Recall(state.HostID, lastUserText(state.History))
	if err != nil || len(recall.PastCommands) == 0 {
		return state, err
	}
	note := Message{
		Role:    RoleUser,
		Content: pastCommandsNote(recall.PastCommands),
	}
	state.History = append([]Message{note}, state.History...)
	return state, nil
}

func lastUserText(msgs []Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == RoleUser {
			return msgs[i].Content
		}
	}
	return ""
}

func pastCommandsNote(pcs []store.PastCommand) string {
	note := "该主机过去执行过的相关命令记录（供参考）：\n"
	for _, c := range pcs {
		note += "- " + c.Command + "\n"
	}
	return note
}
