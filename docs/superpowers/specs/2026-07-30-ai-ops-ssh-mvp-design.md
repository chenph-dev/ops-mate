# ops-mate AI 辅助运维工具 — SSH MVP 设计

- **状态**：草案（待用户审阅）
- **日期**：2026-07-30
- **范围**：ops-mate 的 AI 辅助运维工具首版（v1 / MVP）

---

## 1. 背景与产品定位

ops-mate 是一个基于 Wails v2 的桌面应用（Go 后端 + React/TypeScript 前端，单二进制）。目标是做成一个 **AI 辅助运维工具**。

### 1.1 产品全景愿景（长期）

最终产品应覆盖运维工作的多个方面：远程主机、Kubernetes 集群、数据库、监控等。这些是若干个独立子系统，分阶段交付，每个子系统各自走"设计 → 规划 → 实现"流程。

### 1.2 本 spec 的范围：SSH MVP（v1）

v1 只做 **SSH 远程 Linux 主机** 的 AI 辅助运维。K8s、数据库、监控等作为独立模块在后续 spec 中加入。

### 1.3 核心交互模型：半自动执行

AI 不直接操作机器，也不只是只读顾问。采用"半自动执行"：

```
用户提问 → AI 分析 → AI 提议命令 → 用户【批准 / 修改 / 拒绝】
  → 应用经 SSH 执行 → 结果回传 AI → AI 继续多轮分析 → …
```

每个命令的执行必须经人类批准。最终判断权在人。

### 1.4 关键决策一览

| 维度 | 决策 |
|---|---|
| AI 角色 | 半自动执行（提议→批准→执行→回灌多轮） |
| 运维对象（v1） | SSH 远程 Linux 主机 |
| AI 后端 | 可配置：云端 API（Claude/OpenAI）/ 本地模型（Ollama），设置里选 |
| SSH 凭据管理 | 应用内主机表，用户保存主机信息；凭据加密列存储 |
| AI 上下文来源 | 纯对话 + 命令闭环；v1 不做仪表盘/自动采集 |
| 持久化 | 统一 SQLite（`modernc.org/sqlite`，纯 Go 无 CGO） |
| 记忆 | 跨会话相关命令检索注入；FTS5 关键词检索，不引入向量库 |
| 危险命令护栏 | 引擎层永不硬拒；标红 + 显式二次确认；用户可手动拒绝任意命令 |
| 架构路线 | 方案 A：Go 后端为中心，前端仅 UI |

---

## 2. 总体架构

三段式 + 状态机结构，全部重活在 Go 后端，前端是纯 UI。

```
┌──────────────────────── 前端 (React/TS, 仅 UI) ────────────────────────┐
│  对话页(/chat)   主机页(/hosts)   设置页(/settings)   命令批准卡       │
└───────────────▲───────────────────────────────────────────▲────────────┘
                │ Wails Bind (RPC) + Events (流式输出)      │
┌───────────────┴──────────────────────── Go 后端 ─────────┴───────────┐
│                                                                       │
│  ConversationOrchestrator（会话状态机，核心）                          │
│   └─ Idle → AwaitingApproval → Running → FeedingBack → Idle …          │
│                                                                       │
│  LLMClient 接口（可配置 AI 后端）                                      │
│   ├─ CloudProvider (Claude / OpenAI，HTTP + SSE 流式)                  │
│   └─ LocalProvider (Ollama，HTTP)                                      │
│                                                                       │
│  Executor 接口                                                         │
│   └─ SSHExecutor (golang.org/x/crypto/ssh)（流式输出）                 │
│      [未来: K8sExecutor / DBExecutor 作为新实现加入]                   │
│                                                                       │
│  Memory（记忆检索）— 从 messages + commands 表 FTS5 检索注入           │
│                                                                       │
│  DBStore（SQLite，唯一持久层）                                          │
│   ├─ hosts / ai_config / conversations / messages / commands          │
│   └─ 敏感列(凭据、API Key) AES-GCM 加密                                │
└───────────────────────────────────────────────────────────────────────┘
```

### 2.1 关键边界

- **前端零敏感数据**：私钥、密码、API Key 全程只在 Go 进程内流转；前端只看到主机名与连接状态。AI 提议的命令文本会发给前端供批准，但执行所用的真实凭据不经过 WebView。
- **AI 与执行解耦**：AI 层只产出"建议命令的结构化意图"，执行器层负责真正执行。未来新增 K8s 只需加一个 `Executor` 实现 + 在 AI prompt 侧加一种意图类型。
- **状态机是核心收敛点**：半自动执行的关键是 `AwaitingApproval` 中间态——AI 返回命令时不直接执行，而是暂停会话、推给前端等批准。前端、后端、AI 三方围绕此状态机协作。

### 2.2 模块职责

每个模块单一职责、接口清晰、可独立测试：

| 模块 | 职责 |
|---|---|
| `ConversationOrchestrator` | 会话状态机、多轮编排、记忆注入、命令护栏 |
| `LLMClient` | 统一 AI 调用抽象（流式） |
| `Executor` | 命令执行抽象（流式输出） |
| `Memory` | 跨会话相关上下文检索 |
| `DBStore` | 唯一持久层 + 敏感列加密 |

---

## 3. 组件与接口

### 3.1 `DBStore`（SQLite，唯一持久层）

驱动：`modernc.org/sqlite`（纯 Go，无 CGO，利于 Wails 跨平台编译）。数据库文件放用户数据目录（Windows `%APPDATA%/ops-mate/ops-mate.db`）。

Schema：

```
hosts          (id, name, addr, port, user, auth_encrypted, created_at)
                 -- addr/port/user 明文；密码/私钥 AES-GCM 加密存 auth_encrypted
ai_config      (id, provider, model, base_url, api_key_encrypted)
                 -- API Key 加密列存储
conversations  (id, host_id, title, created_at, updated_at)
messages       (id, session_id, role, content, tool_result, ts)
commands       (id, session_id, command, exit_code, output, ts)
```

加密密钥来源：OS 密钥环（Windows Credential Manager / macOS Keychain）取主密钥；不可用时回退到"用户首次启动设的口令 + scrypt 派生"。加密只保护 at-rest 敏感列，DB 仍是单文件，备份/迁移简单。

经 Wails Bind 暴露给前端的接口（敏感字段不回传明文）：

```
// 主机
ListHosts() []HostMeta                 // 只返回 id/name/addr，不含凭据
SaveHost(h Host) (id string, err error)
DeleteHost(id string) error
TestConnection(id string) (ok bool, err error)

// AI 配置
GetAIConfig() AIConfig                 // api_key 不回传明文
SaveAIConfig(c AIConfig) error

// 会话
ListConversations(hostID string) []Conversation
NewConversation(hostID string) (sessionID string, err error)
LoadConversation(id string) (Conversation, error)
DeleteConversation(id string) error
```

### 3.2 `LLMClient` 接口（可配置 AI 后端）

```
type LLMClient interface {
    // 流式返回 AI 文本与结构化意图
    Chat(ctx context.Context, msgs []Message) (stream <-chan Chunk, err error)
}
```

两类实现（按 `provider` 字段实例化，各自对接不同 API）：
- 云端：`ClaudeProvider`（Anthropic Messages API）、`OpenAIProvider`（OpenAI Chat Completions），均走流式 SSE。
- 本地：`OllamaProvider`（Ollama HTTP）。

三类共同实现 `LLMClient` 接口。v1 可先只实现其中之一打通链路，其余作为同类实现补齐。

`DBStore.GetAIConfig()` 的选择决定实例化哪个。

**Prompt 约束**：用一个系统提示词约束 AI 只能返回两类内容：
1. 普通文本（分析、说明）；
2. 结构化"建议命令"块（JSON，字段：`command`、`why`、`risk`）。

### 3.3 `Executor` 接口

```
type Executor interface {
    Exec(ctx context.Context, hostID, command string) (stream <-chan Line, err error)
}
```

`SSHExecutor` 基于 `golang.org/x/crypto/ssh`，逐行流式输出 stdout/stderr，通过 Wails `EventsEmit` 推到前端；带超时与可取消。v1 只此一个实现。

### 3.4 `Memory`

```
type Memory interface {
    Recall(ctx context.Context, hostID, currentQuestion string) (Context, error)
}
```

v1 记忆策略（简单、可控）：
- **同会话全量历史**：当前 session 的 messages 直接进 prompt（多轮对话天然支持）。
- **跨会话相关命令**：按 `hostID` + 关键词从 `commands` 表 FTS5 检索这台机器过去执行过的命令及结果，取 top-N 注入。
- **不引入向量库**：用 SQLite FTS5 即可；记忆效果不够时再上 embedding。

### 3.5 `ConversationOrchestrator`（会话状态机，核心）

状态枚举：

```
Idle → AwaitingApproval → Running → FeedingBack → Idle …
```

绑定到一个 `hostID`。Go 侧维护会话历史（`[]Message`，含用户输入、AI 回复、已执行命令及输出），并同步落库 `messages` / `commands`。

暴露给前端：

```
SendMessage(hostID, text string) error        // 发起/继续对话
ApproveCommand(sessionID, command string) error  // 批准（可传改后的命令）
RejectCommand(sessionID) error                 // 拒绝→让 AI 换方案
CancelRun(sessionID) error                     // 中止正在执行的命令
```

所有结果通过事件推回前端：

| 事件 | 含义 |
|---|---|
| `ai:text` | AI 文本片段（流式） |
| `ai:command` | 提议的命令 + 理由 + 风险 |
| `run:line` | 执行输出行（流式） |
| `run:done` | 执行结束 + 退出码 |
| `session:state` | 状态机迁移 |

### 3.6 前端组件（页面，遵循现有 `menuConfig` 单一真相源）

在现有路由（首页 / 关于 / 设置）基础上，v1 增补：
- **对话页 `/chat`**（核心）：消息流 + 输入框；AI 提议命令时内联渲染"命令批准卡"（命令、理由、风险标记、批准/拒绝/修改按钮）；执行输出实时流式追加。左侧"会话历史列表"可切换/重开历史会话。
- **主机页 `/hosts`**：主机表（列表/新增/编辑/删除/测试连接）。凭据字段只在录入时输入，列表里不回显。每台主机可展开看该机器历史命令记录。
- **设置页 `/settings`**（已存在占位，填充）：AI 后端选择 + API Key + 模型 + BaseURL。

### 3.7 数据模型

会话历史 `Message`：`{role: user|assistant|tool, content, toolResult?}`。执行结果作为 `role: tool` 消息回灌 AI，使多轮分析基于真实执行输出。

---

## 4. 数据流

以"这台机器为什么 CPU 飙高"为例：

```
前端(对话页)       Orchestrator        LLMClient      SSHExecutor     DBStore      Memory
   │                  │                  │               │             │            │
1.输入"CPU为何高"────▶│                                                                   │
   │(host=web-01)     │── Recall(hostID,"CPU") ───────────────────────────────────▶│
   │                  │◀──── 相关历史: 上次执行过 top, 发现 go 进程 ──────────────────│
   │                  │── 拼prompt(系统词+历史+问题) ─▶│                              │
   │                  │                  │── Chat(流式)─▶ Claude/OpenAI/Ollama      │
   │◀── ai:text(流式) ───────────────────│                                          │
   │                  │◀── AI: 文本"我先看看进程"+命令块{top -bn1, why, risk:low}    │
   │                  │── 落 messages(assistant) ───────────────────────────▶│      │
   │◀── ai:command(命令卡: top -bn1, 风险低) ────│                                  │
   │ 状态→AwaitingApproval                                                          │
2.点[批准]──────────▶│ ApproveCommand(session,"top -bn1")                               │
   │ 状态→Running     │── 取主机凭据 ─────────────────────────────────────▶│        │
   │                  │◀──── auth(解密) ───────────────────────────────────── │        │
   │                  │── Exec(hostID,"top -bn1") ─────────▶│                     │
   │◀── run:line(逐行流式) ─────────────────────────────────│                     │
   │◀── run:done(exitCode=0) ───────────────────────────────│                     │
   │                  │── 落 commands(命令+输出+exitCode) ────────────────▶│        │
   │ 状态→FeedingBack │── 输出作 tool 消息 + 重新 Recall → 调 AI ─▶│                │
   │◀── ai:text(AI分析:"是 go 进程占满,建议看日志") ─────────│                       │
   │◀── ai:command(journalctl -u myapp --since "5 min ago") ──│                      │
   │ 状态→AwaitingApproval (回到第2步，可继续批准/拒绝/修改)                          │
```

关键点：

1. **流式贯穿**：AI 文本与命令输出都流式，前端边收边渲染。用 Wails `EventsEmit` 单向推。
2. **批准点 = 暂停点**：AI 返回命令块后会话进 `AwaitingApproval`，不再自动往下。三个选择：
   - 批准：原样执行；
   - 修改：前端弹编辑框，改完 `ApproveCommand` 传改后命令；
   - 拒绝：`RejectCommand` → Orchestrator 给 AI 回"用户拒绝了此命令，请换方案"，AI 重新提议。
3. **结果回灌 = 多轮**：执行输出作为 `role: tool` 消息追加进历史，再调一次 AI，AI 基于真实输出继续。循环直到 AI 给出纯文本结论（无命令块）= 会话回 `Idle`，本轮结束。
4. **记忆贯穿**：每次调 AI 前 `Recall`——本会话历史（多轮）+ 该机器过往命令记录。
5. **可取消**：`CancelRun` 在 Running 态中止执行中的 SSH 命令（ctx 取消）。
6. **危险命令护栏**（见 §5）。

---

## 5. 危险命令护栏

引擎层**永不硬拒**，用户对任意命令都可手动批准/修改/拒绝。护栏只做提示与摩擦，不夺权：

- AI 提议命令经一道静态风险扫描；命中危险模式（`rm -rf /`、`mkfs`、`dd if=`、`shutdown`/`reboot`、重定向到块设备 `> /dev/sd*` 等）→ 命令卡标红 + 显示风险理由 + 要求**显式二次确认**。
- 三种操作（批准/修改/拒绝）始终可用。
- 拒绝时回灌 AI 一句"用户拒绝此命令，请换方案"。
- 护栏规则放配置，可扩展。

---

## 6. 错误处理

| 场景 | 处理 |
|---|---|
| SSH 连不上（超时/认证失败/网络） | `TestConnection`/`Exec` 返回结构化错误，前端在命令卡位置显示"连接失败：<原因>"，会话回 `AwaitingApproval`，可改命令或重试；不崩溃、不清空对话。 |
| AI 后端不可用（Key 错/限流/超时/本地模型未起） | `Chat` 流式通道先发一条 `ai:text` 错误事件（如"AI 后端不可用：401"），会话回 `Idle`，对话保留用户那条消息但不写入 AI 回复；设置页可改后端。 |
| 命令执行超时/被取消 | ctx 超时或 `CancelRun` → 停止采集，`run:done` 带 `exitCode=-1/取消`，已收输出保留落库，AI 可基于部分输出继续。 |
| 数据库写入失败 | Orchestrator 落库失败时记日志，不阻断主流程（会话仍在内存继续）。 |
| AI 返回非预期格式 | 容错解析：解析不出命令块当普通文本回复展示；解析出但字段不全（缺 `command`）则忽略命令、只展示文本部分。 |
| 凭据解密失败（密钥环变更/DB 损坏） | 该主机标记"凭据不可用"，前端提示重新录入，不影响其他主机。 |

原则：错误以事件回传前端、本地化展示，绝不静默吞掉、绝不让应用白屏。每个失败都对应一个明确的用户下一步。

---

## 7. 测试

按模块可独立测试的边界设计，重 Go 轻 UI：

- **`DBStore`**：临时 SQLite 文件跑全量 CRUD；断言加密列"能存能取、明文不落盘"（grep DB 文件找不到密码明文）、FTS5 检索召回。
- **`LLMClient`**：`httptest` 起假 SSE 服务器模拟 Claude/OpenAI/Ollama，测流式解析、错误码、超时；不调真实 API。
- **`SSHExecutor`**：用 `testcontainers` 起本地 SSH 容器（或 `golang.org/x/crypto/ssh` 测试 server）测执行、流式输出、取消、超时。
- **`ConversationOrchestrator`**（核心）：注入 fake `LLMClient` + fake `Executor` + 内存 `DBStore`，表驱动测试覆盖状态机所有迁移：`Idle→AwaitingApproval→Running→FeedingBack→Idle`、批准/修改/拒绝三分支、危险命令标红、错误回传、记忆注入命中。
- **护栏**：单独测危险模式匹配与标红逻辑。
- **前端**：对话卡渲染、批准/拒绝/修改交互用 Vitest + Testing Library；流式事件用 mock Wails runtime。Wails Bind 集成在 `wails dev` 手测，不写自动化。

---

## 8. 实现顺序（建议，供后续规划参考）

1. SQLite + `DBStore`（schema、CRUD、加密列、FTS5）。
2. `SSHExecutor` + 主机 CRUD 前端（先打通"保存主机→测试连接→执行命令"最小链路）。
3. `LLMClient`（先一个 Provider）+ 设置页。
4. `ConversationOrchestrator` 状态机 + 对话页前端 + 命令批准卡。
5. `Memory` 跨会话检索注入。
6. 护栏、错误处理收尾、测试补齐。

---

## 9. 范围外（v1 不做，留待后续 spec）

- Kubernetes / 数据库 / 监控仪表盘等运维对象。
- 命令执行结果的可视化仪表盘、自动指标采集。
- 向量检索（embedding）记忆。
- 自动 Agent（AI 自主多步执行、无需逐步批准）。
- 多用户 / 团队协作 / 服务端同步。
