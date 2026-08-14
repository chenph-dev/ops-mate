# ops-mate

**ops-mate** is a desktop application for SSH/WinRM server operations and automated ops management, built with [Wails v2](https://wails.io/) (Go backend + React/TypeScript frontend). It combines a classic SSH/SFTP/WinRM client with an **AI-powered ops agent** that can plan and execute troubleshooting or remediation tasks on your servers — with human approval at every step.

![ops-mate 主界面](images/home.png)

## Features

### 🖥️ Host Management
- Organize hosts into a folder **tree** (create / rename / move / delete nodes)
- Manage connection info for **SSH** and **WinRM** hosts (host, port, user, auth)
- **Test connection** before saving; run ad-hoc commands
- Passwords/secrets are **encrypted at rest**

### 🪟 Windows Hosts (WinRM)
- Connect to Windows hosts via **WinRM** (HTTP 5985 / HTTPS 5986, password auth)
- Run ad-hoc **PowerShell** commands from a command panel
- **Launch Remote Desktop (RDP)** with one click — password pre-filled via Windows DPAPI
- The AI agent operates Windows hosts in **PowerShell**, with protocol-aware guardrails

![Windows 主机界面](images/home_win.png)

### 💻 SSH Terminal
- Full interactive terminal (xterm.js, WebGL/canvas rendering) — **SSH hosts only**
- Dynamic **resize**, search, copy/paste, command history
- Multiple concurrent host sessions

### 📁 SFTP File Transfer
- Browse remote directories, create / delete / rename — **SSH hosts only**
- **Upload / download** with a concurrent queue
- Per-task **progress** with pause / resume / cancel

### 🤖 AI Ops Agent
- Chat with an LLM agent that has a direct view of your selected host
- **`execute_command` tool** — the agent proposes concrete **shell (SSH) / PowerShell (WinRM)** commands; **every command must be approved by you before it runs**
- **Plan mode (`create_plan`)** — for complex multi-step tasks the agent first submits an execution plan (goal + step list) for your approval, then executes step by step
- **Guardrails** — dangerous operations are flagged and always require explicit approval; protocol-aware patterns for **Linux** (`rm -rf /`, `dd`, `reboot`…) and **Windows** (`format`, `del /s`, `rd /s`, `diskpart clean`, `shutdown /s`…)
- Conversation history per host, with rename / delete

### ⚙️ Model Configuration
- Bring your own LLM: provider, model, base URL, API key (**encrypted at rest**)
- Supports OpenAI-compatible protocols (OpenAI / DeepSeek / Qwen(通义) / Zhipu(智谱) / Moonshot / Volcengine, etc.), **Anthropic Claude**, and local **Ollama**

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Shell | Wails v2 (Go + WebView2) |
| Backend | Go, [eino](https://github.com/cloudwego/eino) agent framework, eino-ext model adapters, [masterzen/winrm](https://github.com/masterzen/winrm) (WinRM client) |
| Storage | SQLite via `modernc.org/sqlite` (pure Go, no CGO) + GORM + golang-migrate |
| Frontend | React 19, TypeScript, Vite, Ant Design 6, react-router-dom 7, xterm.js |

## Architecture

```
┌────────────────────────── Frontend (React) ─────────────────────────┐
│  HostsPage ► HostTree / Terminal / SFTP / WinRmPanel / AIPanel      │
└──────────────────────────────────┬──────────────────────────────────┘
                                   │  Wails bindings (window.go)
┌──────────────────────────────────▼──────────────────────────────────┐
│  Go Backend (Wails)                                                 │
│  handlers: Hosts · Terminal · Sftp · Sessions · Rdp · AIConfig      │
│  internal/einoagent: model(provider) · tools · session · guardrail  │
│  internal executors: sshexec · winrmexec (Exec interface)           │
│  internal/store:     hosts · conversations · config · memory        │
└─────────────────────────────────────────────────────────────────────┘
```

- **Bindings**: exported methods on Go handler structs are auto-bound and callable from the frontend via `@wailsjs/go/**` (auto-generated, never edit).
- **Routing**: `react-router-dom` with `HashRouter` (required for embedded Wails assets). Single source of truth in `frontend/src/components/AppLayout/menuConfig.tsx`.
- **Frameless window**: custom title-bar drag region built in the frontend.
- **AI events**: the agent streams events (`ai:text`, `ai:command`, `ai:plan`, `run:*`) to the frontend over Wails events to drive the approval UI.

## Development

Prerequisites: [Go](https://go.dev/dl/), [Node.js](https://nodejs.org/), [pnpm](https://pnpm.io/), and the [Wails CLI](https://wails.io/docs/gettingstarted/installation).

```bash
# Live development (hot reload for Go + frontend)
wails dev

# Frontend-only (standalone Vite, no Go bridge)
cd frontend && pnpm dev
```

## Building

```bash
# Production build
wails build

# Build for a specific platform
wails build -platform windows/amd64
wails build -platform darwin/universal
```

## Project Layout

```
├── main.go                  # Wails app entry, binding
├── internal/
│   ├── handler/             # Wails-bound API handlers (hosts, terminal, sftp, sessions, skills, logs, rdp, ai config, approval policy)
│   ├── einoagent/           # AI agent (model/provider, tools, session, guardrail, prompt)
│   ├── sshexec/             # SSH command executor (unified Exec interface)
│   ├── winrmexec/           # WinRM command executor (same Exec interface)
│   ├── sftp/                # SFTP transfer engine (concurrent queue)
│   ├── skill/               # Ops skill management + remote script execution
│   ├── termctx/             # Terminal context ring buffer (AI context injection)
│   └── store/               # SQLite persistence (hosts, conversations, config, memory, logs, skills)
├── frontend/
│   ├── src/                 # React app (pages, components, hooks)
│   └── wailsjs/             # auto-generated Wails bindings (do not edit)
└── wails.json               # Wails project config
```

## License

[MIT](LICENSE) © ops-mate contributors
