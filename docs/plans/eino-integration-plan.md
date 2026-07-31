# ops-mate 接入 eino 框架 — 执行方案

## 一、核心发现：eino Interrupt/Resume 完美映射审批流

eino `v0.10.0-alpha.13` 已内置 **Interrupt/Resume 机制**，这是整个方案的关键支撑：

| eino 机制 | ops-mate 映射 |
|---|---|
| `tool.Interrupt(ctx, info)` | SSH 工具暂停，等待人工审批 |
| `compose.Interrupt(ctx, info)` | Graph 节点暂停，保存断点 |
| `compose.ResumeWithData(ctx, id, data)` | 用户批准后恢复执行 |
| `compose.GetInterruptState(ctx)` | 恢复时读取断点状态 |
| `WithInterruptBeforeNodes([])` | 编译期声明中断点 |
| Graph + Branch + ToolsNode | 替代手写状态机 |
| `eino-ext/components/model/*` | 替代手写 HTTP Provider |

**结论：审批流不是"硬塞进 eino"，而是 eino 原生支持的场景。**

---

## 二、目标架构

### 2.1 Graph 拓扑（一个 session 对应一次 Graph 执行）

```
┌─────────────────────────────────────────────────────────────────┐
│                        Graph[Input, Output]                      │
│                                                                  │
│  START ──▶ InjectMemory ──▶ ChatModel ──▶ Branch                │
│                                        │                         │
│                    ┌───────────────────┼───────────────────┐     │
│                    │ 有 ToolCall       │ 无 ToolCall        │     │
│                    ▼                   ▼                    │     │
│              ApproveNode              END                    │     │
│              (Interrupt)                                     │     │
│                    │                                         │     │
│              用户批准后 Resume                                │     │
│                    │                                         │     │
│                    ▼                                         │     │
│              ToolsNode                                       │     │
│              (SSH Execute)                                   │     │
│                    │                                         │     │
│                    └─────────── loop back ──▶ ChatModel       │     │
│                                                                  │
│  State: { sessionID, hostID, history, executor, emit }          │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 分层架构

```
┌───────────────────────────────────────────────────┐
│  Wails Frontend (React)                            │
│  ai:text / ai:command / run:line / run:done        │
└────────────────────┬──────────────────────────────┘
                     │ EventsEmit
┌────────────────────▼──────────────────────────────┐
│  Orchestrator (业务编排层，保留)                     │
│  - Session 管理 / 审批流接口 / 护栏 / 回灌          │
│  - 调用 eino Graph 的 Invoke/Resume                │
└────────────────────┬──────────────────────────────┘
                     │
┌────────────────────▼──────────────────────────────┐
│  eino Graph (编排引擎，新增)                        │
│  - ChatModel 节点 (LLM 调用)                       │
│  - ApproveNode (Interrupt/Resume)                  │
│  - ToolsNode (SSH 工具执行)                        │
│  - Branch (条件路由)                               │
│  - InjectMemory (FTS5 召回注入)                    │
└────────────────────┬──────────────────────────────┘
                     │
┌────────────────────▼──────────────────────────────┐
│  eino-ext Provider (替换手写 HTTP)                  │
│  - ollama / claude / openai (兼容接口)             │
└───────────────────────────────────────────────────┘
```

---

## 三、目录结构变更

### 新增文件

```
internal/
├── einoagent/                    # 新增：eino 集成层
│   ├── graph.go                  #   Graph 构建 + 编译逻辑
│   ├── state.go                  #   Graph State 结构定义
│   ├── tools.go                  #   SSH Tool → eino InvokableTool
│   ├── approval.go               #   ApproveNode (Interrupt/Resume)
│   ├── provider.go               #   eino-ext Provider 工厂
│   ├── message.go                #   schema.Message ↔ llm.Message 转换
│   ├── callback.go               #   Callback 中间件 (日志/审计)
│   └── graph_test.go             #   Graph 编排单元测试
│
├── orchestrator/
│   ├── orchestrator.go           #   重构：管理 Graph 实例 + 审批接口
│   └── guardrail.go              #   保留不变
│
└── llm/
    ├── client.go                 #   保留：LLMClient 接口定义
    ├── claude.go                #   ❌ Phase 2 完成后删除
    └── ollama.go                #   ❌ Phase 2 完成后删除
```

### 保留不变

```
internal/
├── sshexec/executor.go           # SSH 执行器（被 eino Tool 包装）
└── store/                        # 全部保留（FTS5 记忆 + 持久化）
```

---

## 四、分阶段实施计划

### Phase 1：Provider 层替换（1-2 天）

**目标**：用 eino-ext 替代手写 HTTP，统一 Provider 接口。

#### 1.1 新增 `internal/einoagent/provider.go`

```go
package einoagent

import (
    "context"
    "github.com/cloudwego/eino/components/model"
    openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
    claudemodel "github.com/cloudwego/eino-ext/components/model/claude"
    ollamamodel "github.com/cloudwego/eino-ext/components/model/ollama"
)

// NewChatModel 按配置构造 eino ChatModel
func NewChatModel(cfg store.AIConfig) (model.BaseChatModel, error) {
    switch cfg.Provider {
    case "ollama":
        return ollamamodel.NewChatModel(ctx, &ollamamodel.Config{
            BaseURL: cfg.BaseURL, // http://localhost:11434
            Model:   cfg.Model,
        })
    case "claude":
        return claudemodel.NewChatModel(ctx, &claudemodel.Config{
            APIKey:  cfg.APIKey,
            BaseURL: cfg.BaseURL, // https://api.anthropic.com
            Model:   cfg.Model,
        })
    case "openai", "deepseek", "dashscope", "zhipu":
        // 全部通过 OpenAI 兼容接口
        return openaimodel.NewChatModel(ctx, &openaimodel.Config{
            APIKey:  cfg.APIKey,
            BaseURL: cfg.BaseURL,
            Model:   cfg.Model,
        })
    }
}
```

#### 1.2 新增 `internal/einoagent/message.go`

```goeino
package einoagent

import (
    "github.com/cloudwego/eino/schema"
    "ops-mate/internal/llm"
)

// ToEinoMessages 转换 llm.Message → []*schema.Message
func ToEinoMessages(msgs []llm.Message) []*schema.Message {
    // Role 映射: user→Human, assistant→AI, tool→Tool
    // Content 直接映射
    // ToolResult → schema.Message.ToolCallID + Content
}

// FromEinoMessage 转换 *schema.Message → llm.Message
func FromEinoMessage(m *schema.Message) llm.Message { ... }
```

#### 1.3 新增 `internal/einoagent/llm_adapter.go`

```go
package einoagent

// LLMAdapter 将 eino ChatModel 适配为现有 llm.LLMClient 接口
// 实现 Chat(ctx, msgs) (<-chan Chunk, error)
type LLMAdapter struct {
    model  model.BaseChatModel
    sysMsg *schema.Message
}

func (a *LLMAdapter) Chat(ctx context.Context, msgs []llm.Message) (<-chan llm.Chunk, error) {
    einoMsgs := append([]*schema.Message{a.sysMsg}, ToEinoMessages(msgs)...)
    stream, err := a.model.Stream(ctx, einoMsgs)
    // 将 schema.StreamReader 转为 <-chan llm.Chunk
    // 保持 tryParseCommand 逻辑不变
}
```

#### 1.4 修改 `app.go`

```go
// buildLLM 改用 eino Provider
func (a *App) buildLLM() llm.LLMClient {
    cfg, _ := a.store.GetAIConfig()
    m, err := einoagent.NewChatModel(cfg)
    if err != nil { return nil }
    return &einoagent.LLMAdapter{model: m, sysMsg: ...}
}
```

#### 1.5 验收标准

- [ ] 现有测试全部通过
- [ ] Claude / Ollama 对话正常
- [ ] 流式输出正常
- [ ] 新增 Provider 只需改配置（不改代码）

---

### Phase 2：Graph 编排 + 审批流（2-3 天）

**目标**：用 eino Graph 替代手写状态机，Interrupt/Resume 实现审批。

#### 2.1 新增 `internal/einoagent/state.go`

```go
package einoagent

// GraphState 通过 WithGenLocalState 在节点间共享
type GraphState struct {
    SessionID  string
    HostID     string
    History    []llm.Message
    Executor   sshexec.Exec       // SSH 执行器
    Emit       func(sid, event string, data any)  // 前端推送
    RiskLevel  string             // 当前命令风险
    Command    *llm.CommandSuggestion  // 当前待审批命令
}
```

#### 2.2 新增 `internal/einoagent/tools.go`

```go
package einoagent

import (
    "context"
    "encoding/json"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/schema"
    "ops-mate/internal/orchestrator"  // AssessRisk
    "ops-mate/internal/sshexec"
)

// SSHTool 实现 eino InvokableTool
type SSHTool struct {
    executor sshexec.Exec
    emit     func(sid, event string, data any)
}

func (t *SSHTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name:        "execute_command",
        Description: "在目标主机上执行一条 Shell 命令",
        Parameters: map[string]*schema.Parameter{
            "command": {Type: schema.TypeString, Description: "要执行的命令", Required: true},
            "why":     {Type: schema.TypeString, Description: "执行原因"},
        },
    }, nil
}

func (t *SSHTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
    var args struct {
        Command string `json:"command"`
        Why     string `json:"why"`
    }
    json.Unmarshal([]byte(argsJSON), &args)

    // 护栏检查 → 高风险则 Interrupt 等审批
    risk := orchestrator.AssessRisk(args.Command)
    if risk == "high" {
        return "", tool.Interrupt(ctx, map[string]any{
            "command": args.Command, "why": args.Why, "risk": "high",
        })
    }

    // 低风险直接执行
    ch, err := t.executor.Exec(ctx, args.Command)
    if err != nil { return "", err }
    var output string
    for ln := range ch {
        output += ln.Text + "\n"
    }
    return output, nil
}
```

#### 2.3 新增 `internal/einoagent/approval.go`

```go
package einoagent

import (
    "context"
    "github.com/cloudwego/eino/compose"
)

// ApproveNode 是一个 Lambda 节点：中断等待人工审批
func ApproveNode(ctx context.Context, state *GraphState, cmd *llm.CommandSuggestion) error {
    wasInterrupted, hasState, savedState := compose.GetInterruptState[*ApprovalState](ctx)

    if !wasInterrupted {
        // 首次进入 → 保存状态并中断
        state.Command = cmd
        state.RiskLevel = cmd.Risk
        state.Emit(state.SessionID, "ai:command", map[string]any{
            "command": cmd.Command, "why": cmd.Why, "risk": cmd.Risk,
        })
        return compose.StatefulInterrupt(ctx, cmd, &ApprovalState{
            SessionID: state.SessionID,
            Command:   cmd.Command,
        })
    }

    // 恢复后 → 检查是否是本次审批的目标
    isTarget, hasData, data := compose.GetResumeContext[string](ctx)
    if !isTarget {
        return compose.StatefulInterrupt(ctx, cmd, &ApprovalState{
            SessionID: state.SessionID,
            Command:   cmd.Command,
        })
    }

    // 处理审批结果
    if data == "approved" {
        return nil  // 继续执行工具
    }
    // 被拒绝 → 返回特殊标记让 Graph 走拒绝分支
    return ErrCommandRejected
}

type ApprovalState struct {
    SessionID string
    Command   string
}
```

#### 2.4 新增 `internal/einoagent/graph.go`

```go
package einoagent

import (
    "context"
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/schema"
)

// BuildAgentGraph 构建 ops-mate Agent 的 Graph
func BuildAgentGraph(model model.BaseChatModel, tools []tool.InvokableTool) (*compose.Graph[*GraphState, *GraphState], error) {
    g := compose.NewGraph[*GraphState, *GraphState](
        compose.WithGenLocalState(func(ctx context.Context) *GraphState {
            return &GraphState{}
        }),
    )

    // 节点
    g.AddLambdaNode("inject_memory", compose.InvokableLambda(InjectMemoryNode))
    g.AddChatModelNode("llm", model)  // 需要 BindTools
    g.AddLambdaNode("check_response", compose.InvokableLambda(CheckResponseNode))
    g.AddLambdaNode("approve", compose.InvokableLambda(ApproveNode))
    g.AddToolsNode("tools", compose.NewToolNode(ctx, &compose.ToolsNodeConfig{Tools: tools}))

    // 边
    g.AddEdge(compose.START, "inject_memory")
    g.AddEdge("inject_memory", "llm")
    g.AddEdge("llm", "check_response")

    // 条件分支：有工具调用 → 审批；无 → 结束
    branch := compose.NewGraphBranch(func(ctx context.Context, state *GraphState) (string, error) {
        if state.Command != nil {
            return "approve", nil
        }
        return compose.END, nil
    }, map[string]bool{"approve": true, compose.END: true})
    g.AddBranch("check_response", branch)

    g.AddEdge("approve", "tools")
    g.AddEdge("tools", "llm")  // 回灌循环
    g.AddEdge("tools", compose.END)

    return g, nil
}

// 编译选项：声明中断点
func compileOpts() []compose.GraphCompileOption {
    return []compose.GraphCompileOption{
        compose.WithInterruptBeforeNodes([]string{"approve"}),  // 审批前中断
        compose.WithGraphName("ops_mate_agent"),
        compose.WithGraphMaxRunSteps(50),
    }
}
```

#### 2.5 重构 `internal/orchestrator/orchestrator.go`

```go
package orchestrator

type Orchestrator struct {
    store       *store.Store
    graphs      map[string]*compose.Runnable[*einoagent.GraphState, *einoagent.GraphState]
    interrupts  map[string]*compose.InterruptCtx  // sessionID → 中断上下文
    ExecutorFor func(hostID string) Exec
    emit        ...
}

// SendMessage 启动/继续 Graph 执行
func (o *Orchestrator) SendMessage(sid, text string) error {
    s, _ := o.getSession(sid)
    s.History = append(s.History, llm.Message{Role: llm.RoleUser, Content: text})

    state := &einoagent.GraphState{
        SessionID: sid, HostID: s.HostID,
        History: s.History, Executor: o.ExecutorFor(s.HostID),
        Emit: o.emit,
    }

    graph, _ := o.getOrCreateGraph(sid)
    go func() {
        _, err := graph.Invoke(ctx, state)
        if err != nil {
            if info, ok := compose.ExtractInterruptInfo(err); ok {
                // 审批中断 → 保存上下文，等用户操作
                o.interrupts[sid] = info.InterruptContexts[0]
            }
        }
    }()
}

// ApproveCommand 恢复中断的 Graph
func (o *Orchestrator) ApproveCommand(sid, command string) error {
    interruptCtx := o.interrupts[sid]
    graph := o.graphs[sid]

    // 构造带 Resume 的 context
    ctx := compose.ResumeWithData(context.Background(), interruptCtx.InterruptID, "approved")

    go func() {
        _, err := graph.Invoke(ctx, &einoagent.GraphState{...})
        // 处理再次中断或完成
    }()
}

// RejectCommand 恢复 Graph（带拒绝标记）
func (o *Orchestrator) RejectCommand(sid string) error {
    ctx := compose.ResumeWithData(context.Background(), interruptCtx.InterruptID, "rejected")
    // ...
}
```

#### 2.6 验收标准

- [ ] 审批流程：AI 提议 → 前端弹出审批 → 批准后执行 → 回灌
- [ ] 拒绝流程：AI 提议 → 拒绝 → AI 换方案
- [ ] 危险命令自动标红
- [ ] 状态机行为与重构前一致
- [ ] 旧测试通过 + 新增 Graph 测试通过

---

### Phase 3：回调 + 清理（1 天）

#### 3.1 新增 `internal/einoagent/callback.go`

```go
package einoagent

import "github.com/cloudwego/eino/callbacks"

// AuditCallback 命令审计中间件
type AuditCallback struct{}

func (c *AuditCallback) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) {
    // 记录节点开始
}
func (c *AuditCallback) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) {
    // 记录节点结束 + token 用量
}
func (c *AuditCallback) OnError(ctx context.Context, info *callbacks.RunInfo, err error) {
    // 记录错误
}
```

#### 3.2 清理

- [ ] 删除 `internal/llm/claude.go`
- [ ] 删除 `internal/llm/ollama.go`
- [ ] `go mod tidy` 移除未使用依赖
- [ ] 更新 CLAUDE.md 文档

---

## 五、关键设计决策

### 5.1 审批流用 Interrupt 而非外部状态机

**选择**：审批暂停用 `compose.Interrupt` + `compose.ResumeWithData`，而非在 orchestrator 层手写 `AwaitingApproval` 状态。

**理由**：
- eino 的 Interrupt 是原生能力，自动保存断点状态
- Resume 时自动恢复到中断位置，无需手动管理上下文
- 支持序列化断点（可用于崩溃恢复）
- 与 Graph 生命周期绑定，不会出现状态不一致

### 5.2 保留 Orchestrator 业务层

**选择**：Orchestrator 不删除，改为管理 Graph 实例 + 审批接口。

**理由**：
- Session 管理（创建/销毁/持久化）是业务逻辑
- 护栏（guardrail）是业务规则
- FTS5 记忆召回是领域特定策略
- 前端事件推送绑定 Wails

### 5.3 Provider 用适配器模式

**选择**：新增 `LLMAdapter` 将 eino ChatModel 适配到现有 `LLMClient` 接口，而非直接替换接口。

**理由**：
- Phase 1 可平滑过渡，不影响 orchestrator
- 保留 `Chunk.Command` 解析逻辑（兼容现有 prompt）
- Phase 2 后切换到 eino 原生 tool calling，逐步淘汰 Chunk 解析

---

## 六、风险与缓解

| 风险 | 缓解 |
|---|---|
| eino API 不稳定 (alpha) | 锁定版本 v0.10.0-alpha.13；封装适配层隔离变化 |
| Interrupt/Resume 在 Wails 多 goroutine 下的行为 | 充分测试并发场景；Graph 实例 per-session 隔离 |
| 审批中断后 Graph 生命周期管理 | 明确 Graph 实例的创建/销毁时机；context 取消时清理 |
| 流式输出与 Interrupt 的兼容 | 流式模式下 Interrupt 行为需验证；必要时回退到 Invoke |
| 学习成本 | 封装为 `einoagent` 包，对外暴露简洁接口 |

---

## 七、测试策略

### 7.1 单元测试

- `provider.go`：mock HTTP 验证各 Provider 构造
- `message.go`：验证消息转换正确性
- `graph.go`：用 mock ChatModel + stub Tool 验证 Graph 拓扑
- `approval.go`：验证 Interrupt/Resume 状态机

### 7.2 集成测试

- 端到端：SendMessage → AI 提议 → ApproveCommand → 执行 → 回灌
- 拒绝路径：RejectCommand → AI 换方案
- 危险命令：AssessRisk = high → 强制 Interrupt
- 流式：验证 ai:text 事件逐 token 到达

### 7.3 回归

- 现有 `orchestrator_test.go` 全部通过
- 前端功能无退化

---

## 八、时间线

| 阶段 | 内容 | 预估 |
|---|---|---|
| Phase 1 | Provider 替换 + 适配器 | 1-2 天 |
| Phase 2 | Graph 编排 + 审批流 | 2-3 天 |
| Phase 3 | 回调 + 清理 | 1 天 |
| **合计** | | **4-6 天** |

---

## 九、下一步

确认方案后，按 Phase 1 → Phase 2 → Phase 3 顺序执行。每个 Phase 完成后跑全量测试 + 前端验证，通过后再进入下一 Phase。
