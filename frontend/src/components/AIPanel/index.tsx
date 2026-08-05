import { useCallback, useEffect, useRef, useState } from "react";
import type { TextAreaRef } from "antd/es/input/TextArea";
import { GetAIConfig } from "@wailsjs/go/handler/AIConfigHandler";
import type { configstore, convstore } from "@wailsjs/go/models";
import type {
  ApprovalStatus,
  CommandSuggestion,
  Message,
  SessionState,
} from "./types";
import { STATE_LABEL } from "./types";
import PanelHeader from "./PanelHeader";
import MessageList from "./MessageList";
import PanelInput from "./PanelInput";

export interface AIPanelProps {
  activeSession: string | null;
  messages: Message[];
  conversations: convstore.Conversation[];
  streamingText: string;
  pendingCommand: CommandSuggestion | null;
  commandStatus: ApprovalStatus | null;
  sessionState: SessionState;
  lastError: string | null;
  runningCommand: string | null;
  runElapsed: number;
  hostName: string;
  collapsed: boolean;
  onRefreshConversations: () => Promise<void>;
  onSwitchConversation: (sid: string) => Promise<void>;
  onDeleteConversation: (sid: string) => Promise<void>;
  onToggleCollapse: () => void;
  onSendMessage: (text: string) => Promise<void>;
  onApprove: (command: string) => Promise<void>;
  onReject: () => Promise<void>;
  onCancel: () => Promise<void>;
  onNewConversation: () => Promise<void>;
}

export default function AIPanel({
  activeSession,
  messages,
  conversations,
  streamingText,
  pendingCommand,
  commandStatus,
  sessionState,
  lastError,
  runningCommand,
  runElapsed,
  hostName,
  collapsed,
  onRefreshConversations,
  onSwitchConversation,
  onDeleteConversation,
  onToggleCollapse,
  onSendMessage,
  onApprove,
  onReject,
  onCancel,
  onNewConversation,
}: AIPanelProps): React.JSX.Element | null {
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [aiCfg, setAiCfg] = useState<configstore.AIConfig | null>(null);
  const [cfgLoading, setCfgLoading] = useState(true);
  const [resizeHover, setResizeHover] = useState(false);
  const [panelWidth, setPanelWidth] = useState<number>(() => {
    const saved = Number(localStorage.getItem("ai-panel-width"));
    return saved >= 280 && saved <= 600 ? saved : 420;
  });
  const resizeRef = useRef<{ startX: number; startW: number } | null>(null);
  const inputRef = useRef<TextAreaRef>(null);

  // 左边缘拖拽调整宽度：mousedown 挂 window 监听，mouseup 移除，实时持久化。
  const onResizeStart = useCallback(
    (e: React.MouseEvent<HTMLDivElement>): void => {
      e.preventDefault();
      resizeRef.current = { startX: e.clientX, startW: panelWidth };
      const onMove = (ev: MouseEvent): void => {
        const r = resizeRef.current;
        if (!r) return;
        // 右侧固定：向左拖动（clientX 减小）增加宽度
        const next = Math.min(
          Math.max(r.startW + (r.startX - ev.clientX), 280),
          600,
        );
        setPanelWidth(next);
        localStorage.setItem("ai-panel-width", String(next));
      };
      const onUp = (): void => {
        resizeRef.current = null;
        window.removeEventListener("mousemove", onMove);
        window.removeEventListener("mouseup", onUp);
      };
      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onUp);
    },
    [panelWidth],
  );

  const configured = !!aiCfg && !!aiCfg.provider && !!aiCfg.model;
  const busy = sessionState === "Thinking" || sessionState === "Running";
  const inputDisabled =
    busy || sessionState === "AwaitingApproval" || !configured;

  // 展开抽屉时拉取 AI 配置：已配置在标题显示模型；未配置给出提醒与跳转。
  // 从「AI 配置」页保存后返回本页，AIPanel 重新挂载，collapsed 回到初始态，
  // 再次展开即拉到最新配置（热更新）。
  useEffect(() => {
    if (collapsed) return;
    let alive = true;
    GetAIConfig()
      .then((cfg) => {
        if (alive) setAiCfg(cfg ?? null);
      })
      .catch(() => {
        if (alive) setAiCfg(null);
      })
      .finally(() => {
        if (alive) setCfgLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [collapsed]);

  const handleSend = async (): Promise<void> => {
    const text = input.trim();
    if (!text || sending || inputDisabled) return;
    setInput("");
    setSending(true);
    try {
      await onSendMessage(text);
    } finally {
      setSending(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent): void => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      void handleSend();
    }
  };

  // 折叠态：入口已移到终端右上角工具栏（TerminalHeader 的 AI 开关按钮），
  // 本组件不占位，仅展开时渲染并排面板。
  if (collapsed) return null;

  const stateMeta = sessionState ? STATE_LABEL[sessionState] : null;

  return (
    <div
      style={{
        width: panelWidth,
        height: "100%",
        flexShrink: 0,
        display: "flex",
        flexDirection: "column",
        position: "relative",
        background: "var(--antd-color-bg-elevated)",
        borderLeft: "1px solid var(--antd-color-border)",
        boxShadow: "var(--antd-box-shadow)",
      }}
    >
      {/* 左边缘拖拽条：调整面板宽度（与终端并排，终端自动让出空间） */}
      <div
        onMouseDown={onResizeStart}
        onMouseEnter={() => setResizeHover(true)}
        onMouseLeave={() => setResizeHover(false)}
        title="拖动调整宽度"
        style={{
          position: "absolute",
          left: 0,
          top: 0,
          bottom: 0,
          width: 5,
          cursor: "ew-resize",
          background: resizeHover
            ? "var(--antd-color-primary)"
            : "transparent",
          opacity: resizeHover ? 0.5 : 0,
          zIndex: 2,
        }}
      />
      <PanelHeader
        hostName={hostName}
        aiCfg={aiCfg}
        configured={configured}
        cfgLoading={cfgLoading}
        stateMeta={stateMeta}
        sessionState={sessionState}
        conversations={conversations}
        activeSession={activeSession}
        onSwitchConversation={onSwitchConversation}
        onDeleteConversation={onDeleteConversation}
        onRefreshConversations={onRefreshConversations}
        onCancel={onCancel}
        onNewConversation={onNewConversation}
        onToggleCollapse={onToggleCollapse}
      />
      <MessageList
        messages={messages}
        streamingText={streamingText}
        pendingCommand={pendingCommand}
        commandStatus={commandStatus}
        lastError={lastError}
        busy={busy}
        configured={configured}
        cfgLoading={cfgLoading}
        runningCommand={runningCommand}
        runElapsed={runElapsed}
        onApprove={onApprove}
        onReject={onReject}
        onSelectSuggestion={(text) => {
          setInput(text);
          requestAnimationFrame(() => inputRef.current?.focus());
        }}
      />
      <PanelInput
        ref={inputRef}
        input={input}
        sending={sending}
        inputDisabled={inputDisabled}
        configured={configured}
        onInputChange={setInput}
        onSend={handleSend}
        onKeyDown={handleKeyDown}
      />
    </div>
  );
}
