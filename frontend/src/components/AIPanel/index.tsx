import { useCallback, useEffect, useRef, useState } from "react";
import { MessageOutlined } from "@ant-design/icons";
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
import HistoryDrawer from "./HistoryDrawer";

export interface AIPanelProps {
  activeSession: string | null;
  messages: Message[];
  conversations: convstore.Conversation[];
  streamingText: string;
  pendingCommand: CommandSuggestion | null;
  commandStatus: ApprovalStatus | null;
  sessionState: SessionState;
  lastError: string | null;
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
}: AIPanelProps): React.JSX.Element {
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [aiCfg, setAiCfg] = useState<configstore.AIConfig | null>(null);
  const [cfgLoading, setCfgLoading] = useState(true);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [resizeHover, setResizeHover] = useState(false);
  const [panelHeight, setPanelHeight] = useState<number>(() => {
    const saved = Number(localStorage.getItem("ai-panel-height"));
    return saved >= 160 && saved <= 800 ? saved : 360;
  });
  const resizeRef = useRef<{ startY: number; startH: number } | null>(null);

  // 顶部手柄拖动调整高度：mousedown 挂 window 监听，mouseup 移除，实时持久化。
  const onResizeStart = useCallback(
    (e: React.MouseEvent<HTMLDivElement>): void => {
      e.preventDefault();
      resizeRef.current = { startY: e.clientY, startH: panelHeight };
      const onMove = (ev: MouseEvent): void => {
        const r = resizeRef.current;
        if (!r) return;
        // bottom 定位：向上拖动（clientY 减小）增加高度
        const next = Math.min(
          Math.max(r.startH + (r.startY - ev.clientY), 160),
          window.innerHeight - 160,
        );
        setPanelHeight(next);
        localStorage.setItem("ai-panel-height", String(next));
      };
      const onUp = (): void => {
        resizeRef.current = null;
        window.removeEventListener("mousemove", onMove);
        window.removeEventListener("mouseup", onUp);
      };
      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onUp);
    },
    [panelHeight],
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

  // 悬浮态：右下角小按钮
  if (collapsed) {
    return (
      <div
        onClick={onToggleCollapse}
        style={{
          position: "absolute",
          bottom: 40,
          right: 12,
          background: "var(--antd-color-bg-elevated)",
          border: "1px solid var(--antd-color-border)",
          borderRadius: 12,
          padding: "8px 14px",
          cursor: "pointer",
          display: "flex",
          alignItems: "center",
          gap: 6,
          boxShadow: "var(--antd-box-shadow-secondary)",
          zIndex: 100,
          fontSize: 13,
          color: "var(--antd-color-text)",
        }}
      >
        <MessageOutlined style={{ color: "var(--antd-color-primary)" }} />
        <span>智能终端</span>
      </div>
    );
  }

  const stateMeta = sessionState ? STATE_LABEL[sessionState] : null;

  return (
    <div
      style={{
        position: "absolute",
        bottom: 0,
        left: 5,
        right: 0,
        height: panelHeight,
        display: "flex",
        flexDirection: "column",
        background: "var(--antd-color-bg-elevated)",
        borderTop: "1px solid var(--antd-color-border)",
        boxShadow: "var(--antd-box-shadow)",
        zIndex: 99,
        borderTopLeftRadius: 8,
        borderTopRightRadius: 8,
      }}
    >
      <PanelHeader
        hostName={hostName}
        aiCfg={aiCfg}
        configured={configured}
        cfgLoading={cfgLoading}
        stateMeta={stateMeta}
        sessionState={sessionState}
        resizeHover={resizeHover}
        onResizeStart={onResizeStart}
        onResizeHoverChange={setResizeHover}
        onCancel={onCancel}
        onOpenHistory={() => {
          setHistoryOpen(true);
          void onRefreshConversations();
        }}
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
        onApprove={onApprove}
        onReject={onReject}
      />
      <PanelInput
        input={input}
        sending={sending}
        inputDisabled={inputDisabled}
        configured={configured}
        onInputChange={setInput}
        onSend={handleSend}
        onKeyDown={handleKeyDown}
      />
      <HistoryDrawer
        open={historyOpen}
        onClose={() => setHistoryOpen(false)}
        conversations={conversations}
        activeSession={activeSession}
        onSwitchConversation={onSwitchConversation}
        onDeleteConversation={onDeleteConversation}
      />
    </div>
  );
}
