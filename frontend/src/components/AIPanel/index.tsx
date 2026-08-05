import { Button, Input, Spin, Tag, Tooltip } from "antd";
import {
  MessageOutlined,
  CompressOutlined,
  SendOutlined,
  PlusOutlined,
  StopOutlined,
} from "@ant-design/icons";
import { useEffect, useRef, useState } from "react";
import type { convstore } from "@wailsjs/go/models";
import type { CommandSuggestion, SessionState } from "@/hooks/useSessions";
import CommandCard from "@/components/CommandCard";

type Message = convstore.Message;

interface AIPanelProps {
  messages: Message[];
  streamingText: string;
  pendingCommand: CommandSuggestion | null;
  sessionState: SessionState;
  lastError: string | null;
  hostName: string;
  collapsed: boolean;
  onToggleCollapse: () => void;
  onSendMessage: (text: string) => Promise<void>;
  onApprove: (command: string) => Promise<void>;
  onReject: () => Promise<void>;
  onCancel: () => Promise<void>;
  onNewConversation: () => Promise<void>;
}

/** 从 assistant 消息的 toolCalls JSON 解析出命令建议（历史回放用）。 */
function parseToolCallCommand(msg: Message): CommandSuggestion | null {
  if (!msg.toolCalls) return null;
  try {
    const calls = JSON.parse(msg.toolCalls) as Array<{
      arguments: string;
    }>;
    if (calls.length === 0) return null;
    const args = JSON.parse(calls[0].arguments) as {
      command?: string;
      why?: string;
    };
    if (!args.command) return null;
    return {
      command: args.command,
      why: args.why ?? "",
      risk: "",
      assessedRisk: "",
    };
  } catch {
    return null;
  }
}

const STATE_LABEL: Record<string, { text: string; color: string }> = {
  Thinking: { text: "思考中", color: "blue" },
  AwaitingApproval: { text: "等待审批", color: "orange" },
  Running: { text: "执行中", color: "green" },
};

export default function AIPanel({
  messages,
  streamingText,
  pendingCommand,
  sessionState,
  lastError,
  hostName,
  collapsed,
  onToggleCollapse,
  onSendMessage,
  onApprove,
  onReject,
  onCancel,
  onNewConversation,
}: AIPanelProps): React.JSX.Element {
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const msgRef = useRef<HTMLDivElement>(null);

  const busy = sessionState === "Thinking" || sessionState === "Running";
  const inputDisabled = busy || sessionState === "AwaitingApproval";

  useEffect(() => {
    if (msgRef.current) {
      msgRef.current.scrollTop = msgRef.current.scrollHeight;
    }
  }, [messages, streamingText, pendingCommand]);

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

  const renderMessage = (msg: Message): React.JSX.Element => {
    if (msg.role === "user") {
      return (
        <div key={msg.id} style={{ display: "flex", justifyContent: "flex-end", marginBottom: 6 }}>
          <div
            style={{
              maxWidth: "85%",
              padding: "5px 10px",
              borderRadius: 8,
              fontSize: 12,
              lineHeight: 1.5,
              background: "var(--antd-color-primary)",
              color: "#fff",
              whiteSpace: "pre-wrap",
              wordBreak: "break-all",
            }}
          >
            {msg.content}
          </div>
        </div>
      );
    }

    if (msg.role === "assistant") {
      const suggested = parseToolCallCommand(msg);
      if (suggested) {
        // 历史命令提议（回放模式，无操作按钮）
        return (
          <CommandCard key={msg.id} command={suggested} history />
        );
      }
      return (
        <div key={msg.id} style={{ display: "flex", justifyContent: "flex-start", marginBottom: 6 }}>
          <div
            style={{
              maxWidth: "85%",
              padding: "5px 10px",
              borderRadius: 8,
              fontSize: 12,
              lineHeight: 1.5,
              background: "var(--antd-color-fill-secondary)",
              color: "var(--antd-color-text)",
              whiteSpace: "pre-wrap",
              wordBreak: "break-all",
            }}
          >
            {msg.content}
          </div>
        </div>
      );
    }

    // tool 消息：终端风格输出块
    return (
      <div key={msg.id} style={{ marginBottom: 6 }}>
        <div
          style={{
            fontFamily: '"Cascadia Code", "Fira Code", "Consolas", monospace',
            fontSize: 11,
            background: "rgba(0,0,0,0.25)",
            border: "1px solid var(--antd-color-border-secondary)",
            borderRadius: 4,
            padding: "6px 8px",
            whiteSpace: "pre-wrap",
            wordBreak: "break-all",
            color: "var(--antd-color-text)",
            maxHeight: 200,
            overflow: "auto",
          }}
        >
          {msg.content}
        </div>
      </div>
    );
  };

  const stateMeta = sessionState ? STATE_LABEL[sessionState] : null;

  return (
    <div
      style={{
        position: "absolute",
        bottom: 0,
        left: 0,
        right: 0,
        height: 360,
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
      {/* 标题栏 */}
      <div
        style={{
          padding: "6px 10px",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          borderBottom: "1px solid var(--antd-color-border-secondary)",
          flexShrink: 0,
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <MessageOutlined style={{ color: "var(--antd-color-primary)" }} />
          <span style={{ fontSize: 12, fontWeight: 600 }}>AI 助手</span>
          <span style={{ fontSize: 11, color: "var(--antd-color-text-secondary)" }}>
            · {hostName}
          </span>
          {stateMeta && <Tag color={stateMeta.color}>{stateMeta.text}</Tag>}
        </div>
        <div style={{ display: "flex", gap: 4 }}>
          {sessionState === "Running" && (
            <Tooltip title="取消执行">
              <Button
                type="text"
                size="small"
                danger
                icon={<StopOutlined />}
                onClick={() => void onCancel()}
              />
            </Tooltip>
          )}
          <Tooltip title="新建对话">
            <Button
              type="text"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => void onNewConversation()}
            />
          </Tooltip>
          <Tooltip title="收起">
            <Button
              type="text"
              size="small"
              icon={<CompressOutlined />}
              onClick={onToggleCollapse}
            />
          </Tooltip>
        </div>
      </div>

      {/* 消息区 */}
      <div
        ref={msgRef}
        style={{ flex: 1, overflow: "auto", padding: "8px 12px" }}
      >
        {messages.length === 0 && !streamingText && !pendingCommand ? (
          <div
            style={{
              color: "var(--antd-color-text-secondary)",
              fontSize: 12,
              padding: 16,
              textAlign: "center",
            }}
          >
            发送消息开始对话...
          </div>
        ) : (
          <>
            {messages.map(renderMessage)}

            {/* 流式气泡 */}
            {streamingText && (
              <div style={{ display: "flex", justifyContent: "flex-start", marginBottom: 6 }}>
                <div
                  style={{
                    maxWidth: "85%",
                    padding: "5px 10px",
                    borderRadius: 8,
                    fontSize: 12,
                    lineHeight: 1.5,
                    background: "var(--antd-color-fill-secondary)",
                    color: "var(--antd-color-text)",
                    whiteSpace: "pre-wrap",
                    wordBreak: "break-all",
                  }}
                >
                  {streamingText}
                  <span style={{ opacity: 0.6 }}>▌</span>
                </div>
              </div>
            )}

            {/* 审批卡 */}
            {pendingCommand && (
              <CommandCard
                command={pendingCommand}
                busy={false}
                onApprove={(cmd) => void onApprove(cmd)}
                onReject={() => void onReject()}
              />
            )}

            {/* 错误提示 */}
            {lastError && (
              <div
                style={{
                  fontSize: 12,
                  color: "var(--antd-color-error)",
                  padding: "4px 8px",
                }}
              >
                {lastError}
              </div>
            )}

            {busy && !streamingText && (
              <div style={{ display: "flex", justifyContent: "flex-start" }}>
                <Spin size="small" />
              </div>
            )}
          </>
        )}
      </div>

      {/* 输入区 */}
      <div
        style={{
          padding: "6px 8px",
          borderTop: "1px solid var(--antd-color-border-secondary)",
          display: "flex",
          gap: 6,
          alignItems: "flex-end",
          flexShrink: 0,
        }}
      >
        <Input.TextArea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={
            inputDisabled ? "等待本轮对话结束..." : "输入问题..."
          }
          autoSize={{ minRows: 1, maxRows: 3 }}
          style={{ fontSize: 12 }}
          disabled={inputDisabled}
        />
        <Button
          type="primary"
          size="small"
          icon={<SendOutlined />}
          onClick={() => void handleSend()}
          disabled={!input.trim() || sending || inputDisabled}
          loading={sending}
        />
      </div>
    </div>
  );
}
