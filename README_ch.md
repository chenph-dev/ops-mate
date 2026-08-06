# ops-mate

**ops-mate** 是一款基于 [Wails v2](https://wails.io/)（Go 后端 + React/TypeScript 前端）的 SSH 运维管理桌面应用。它把经典的 SSH/SFTP 客户端与 **AI 运维智能体** 结合，智能体可以规划并执行服务器上的诊断/修复任务——每一步都需要你审批确认。

![ops-mate 主界面](images/home.png)

## 功能特性

### 🖥️ 主机管理
- 文件夹**树状**组织主机（新建 / 重命名 / 移动 / 删除节点）
- 管理 SSH 连接信息（主机、端口、用户、认证方式）
- 保存前可**测试连接**；支持临时执行命令
- 密码等敏感信息**加密落盘**

### 💻 SSH 终端
- 完整交互式终端（xterm.js，WebGL/Canvas 渲染）
- 动态**调整大小**、搜索、复制粘贴、命令历史
- 支持多台主机并发会话

### 📁 SFTP 文件传输
- 浏览远程目录，新建 / 删除 / 重命名
- **上传 / 下载** 并发队列
- 每个任务独立**进度**，支持暂停 / 继续 / 取消

### 🤖 AI 运维智能体
- 与能"看到"当前选中主机的 LLM 智能体对话
- **`execute_command` 工具**：智能体提出具体 Shell 命令，**每条命令都必须经你批准后才执行**
- **计划模式（`create_plan`）**：面对复杂多步任务，智能体先提交执行计划（目标 + 步骤列表）供你审批，批准后按计划逐步执行
- **安全护栏**：危险操作（删除、重启、关机、格式化等）会明确提示风险，且始终要求显式审批
- 每台主机独立对话历史，支持重命名 / 删除

### ⚙️ 模型配置
- 自带大模型：供应商、模型、Base URL、API Key（**加密落盘**）
- 支持 OpenAI 兼容协议（OpenAI / DeepSeek / 通义 / 智谱 / Moonshot / 火山引擎等）、**Anthropic Claude**、以及本地 **Ollama**

## 技术栈

| 层 | 技术 |
|----|------|
| 外壳 | Wails v2（Go + WebView2） |
| 后端 | Go、[eino](https://github.com/cloudwego/eino) 智能体框架、eino-ext 模型适配器 |
| 存储 | `modernc.org/sqlite`（纯 Go，无需 CGO）+ GORM + golang-migrate |
| 前端 | React 19、TypeScript、Vite、Ant Design 6、react-router-dom 7、xterm.js |

## 架构

```
┌─────────────────────────── 前端（React）───────────────────────────┐
│  HostsPage ──► HostTree / Terminal / SFTP 面板 / AIPanel / PlanCard │
└───────────────────────────────┬────────────────────────────────────┘
                                │  Wails 绑定（window.go）
┌───────────────────────────────▼────────────────────────────────────┐
│                         Go 后端（Wails）                             │
│  handlers: Hosts · Terminal · Sftp · Sessions · AIConfig             │
│  internal/einoagent: model(provider) · tools · session · guardrail  │
│  internal/store:     hosts · conversations · config · memory(SQLite)│
└─────────────────────────────────────────────────────────────────────┘
```

- **绑定机制**：Go handler 结构体上的导出方法自动绑定，前端通过 `@wailsjs/go/**` 调用（自动生成，勿手改）。
- **路由**：`react-router-dom` 使用 `HashRouter`（嵌入式 Wails 资源要求）。单一数据源位于 `frontend/src/components/AppLayout/menuConfig.tsx`。
- **无边框窗口**：前端实现自定义标题栏拖拽区域。
- **AI 事件**：智能体通过 Wails 事件向前端推送流式事件（`ai:text`、`ai:command`、`ai:plan`、`run:*`），驱动审批界面。

## 开发

前置依赖：[Go](https://go.dev/dl/)、[Node.js](https://nodejs.org/)、[pnpm](https://pnpm.io/)、[Wails CLI](https://wails.io/docs/gettingstarted/installation)。

```bash
# 实时开发（Go 与前端热重载）
wails dev

# 仅前端（独立 Vite，无 Go 桥接）
cd frontend && pnpm dev
```

## 构建

```bash
# 生产构建
wails build

# 指定平台
wails build -platform windows/amd64
wails build -platform darwin/universal
```

## 目录结构

```
├── main.go                  # Wails 应用入口、绑定
├── internal/
│   ├── handler/             # Wails 绑定的 API handler（主机、终端、SFTP、会话、AI 配置）
│   ├── einoagent/           # AI 智能体（model/provider、tools、session、guardrail、history）
│   ├── sftp/                # SFTP 传输引擎（并发队列）
│   └── store/               # SQLite 持久化（主机、会话、配置、记忆）
├── frontend/
│   ├── src/                 # React 应用（页面、组件、hooks）
│   └── wailsjs/             # 自动生成的 Wails 绑定（勿手改）
└── wails.json               # Wails 项目配置
```

## 开源协议

[MIT](LICENSE) © ops-mate contributors
