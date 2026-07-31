import { Button, Input, Spin, Tooltip, Dropdown, theme } from "antd";
import { PlusOutlined, DeleteOutlined, SendOutlined } from "@ant-design/icons";
import { useEffect, useRef, useState } from "react";
import type { convstore } from "@wailsjs/go/models";

type Message = convstore.Message;
type Conversation = convstore.Conversation;
import type { CommandSuggestion } from "@/hooks/useSessions";
import CommandCard from "./CommandCard";

interface AIChatProps {
  conversations: Conversation[];
  activeSession: string | null;
  messages: Message[];
  pendingCommand: CommandSuggestion | null;
  sessionState: string | null;
  hostName: string;
  onSelectSession: (id: string) => void;
  onCreateSession: (title: string) => Promise<string | void>;
  onDeleteSession: (id: string) => void;
  onSendMessage: (text: string) => Promise<void>;
  onApprove: (command: string) => Promise<void>;
  onReject: () => Promise<void>;
}

export default function AIChat({
  conversations,
  activeSession,
  messages,
  pendingCommand,
  sessionState,
  hostName,
  onSelectSession,
  onCreateSession,
  onDeleteSession,
  onSendMessage,
  onApprove,
  onReject,
}: AIChatProps): React.JSX.Element {
  const { token } = theme.useToken();
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const msgRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (msgRef.current) {
      msgRef.current.scrollTop = msgRef.current.scrollHeight;
    }
  }, [messages, pendingCommand]);

  const handleSend = async () => {
    const text = input.trim();
    if (!text || sending) return;
    setInput("");
    setSending(true);
    try {
      await onSendMessage(text);
    } finally {
      setSending(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      {/* 标题栏 */}
      <div
        style={{
          padding: "6px 10px",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          borderBottom: `1px solid ${token.colorBorderSecondary}`,
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span
            style={{
              fontSize: 12,
              fontWeight: 600,
              color: token.colorTextSecondary,
            }}
          >
            AI 对话
          </span>
          {activeSession && (
            <span style={{ fontSize: 11, color: token.colorTextSecondary }}>
              · {hostName}
            </span>
          )}
          {sessionState && (
            <span style={{ fontSize: 11, color: token.colorWarning }}>
              [{sessionState}]
            </span>
          )}
        </div>
        <Tooltip title="新建对话">
          <Button
            type="text"
            size="small"
            icon={<PlusOutlined />}
            onClick={() => {
              const title = `新对话 ${new Date().toLocaleTimeString()}`;
              onCreateSession(title);
            }}
          />
        </Tooltip>
      </div>

      {/* 会话切换栏 */}
      {conversations.length > 0 && (
        <div
          style={{
            padding: "4px 8px",
            borderBottom: `1px solid ${token.colorBorderSecondary}`,
            display: "flex",
            gap: 4,
            overflowX: "auto",
          }}
        >
          {conversations.map((c) => (
            <div
              key={c.id}
              onClick={() => onSelectSession(c.id)}
              style={{
                fontSize: 11,
                padding: "2px 8px",
                borderRadius: 4,
                cursor: "pointer",
                whiteSpace: "nowrap",
                background:
                  c.id === activeSession ? token.colorPrimaryBg : "transparent",
                color:
                  c.id === activeSession
                    ? token.colorPrimary
                    : token.colorTextSecondary,
                display: "flex",
                alignItems: "center",
                gap: 4,
              }}
            >
              <span
                style={{
                  maxWidth: 80,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                }}
              >
                {c.title}
              </span>
              <DeleteOutlined
                style={{ fontSize: 10 }}
                onClick={(e) => {
                  e.stopPropagation();
                  onDeleteSession(c.id);
                }}
              />
            </div>
          ))}
        </div>
      )}

      {/* 消息区 */}
      <div
        ref={msgRef}
        style={{ flex: 1, overflow: "auto", padding: "8px 12px" }}
      >
        {!activeSession ? (
          <div
            style={{
              color: token.colorTextSecondary,
              fontSize: 12,
              padding: 24,
              textAlign: "center",
            }}
          >
            选择一个会话或新建对话开始
          </div>
        ) : messages.length === 0 && !pendingCommand ? (
          <div
            style={{
              color: token.colorTextSecondary,
              fontSize: 12,
              padding: 24,
              textAlign: "center",
            }}
          >
            发送消息开始对话...
          </div>
        ) : (
          <>
            {messages.map((msg) => (
              <div
                key={msg.id}
                style={{
                  display: "flex",
                  justifyContent:
                    msg.role === "user" ? "flex-end" : "flex-start",
                  marginBottom: 8,
                }}
              >
                <div
                  style={{
                    maxWidth: "80%",
                    padding: "6px 10px",
                    borderRadius: 8,
                    fontSize: 13,
                    lineHeight: 1.5,
                    background:
                      msg.role === "user"
                        ? token.colorPrimary
                        : token.colorBgElevated,
                    color: msg.role === "user" ? "#fff" : token.colorText,
                  }}
                >
                  <div
                    style={{ whiteSpace: "pre-wrap", wordBreak: "break-all" }}
                  >
                    {msg.content}
                  </div>
                  {msg.toolResult && (
                    <div
                      style={{
                        marginTop: 6,
                        padding: "4px 8px",
                        background: "rgba(0,0,0,0.15)",
                        borderRadius: 4,
                        fontFamily: "monospace",
                        fontSize: 12,
                        whiteSpace: "pre-wrap",
                      }}
                    >
                      {msg.toolResult}
                    </div>
                  )}
                </div>
              </div>
            ))}

            {/* 命令审批卡片 */}
            {pendingCommand && (
              <CommandCard
                command={pendingCommand}
                onApprove={() => onApprove(pendingCommand.command)}
                onReject={onReject}
              />
            )}

            {sending && (
              <div
                style={{
                  display: "flex",
                  justifyContent: "flex-start",
                  marginBottom: 8,
                }}
              >
                <Spin size="small" />
              </div>
            )}
          </>
        )}
      </div>

      {/* 输入区 */}
      <div
        style={{
          padding: "8px 10px",
          borderTop: `1px solid ${token.colorBorderSecondary}`,
          display: "flex",
          gap: 8,
          alignItems: "flex-end",
        }}
      >
        <Input.TextArea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="输入问题... (Enter 发送, Shift+Enter 换行)"
          autoSize={{ minRows: 1, maxRows: 4 }}
          style={{ fontSize: 13 }}
          disabled={!activeSession || !!pendingCommand}
        />
        <Button
          type="primary"
          icon={<SendOutlined />}
          onClick={handleSend}
          disabled={!activeSession || !input.trim() || !!pendingCommand}
          loading={sending}
        />
      </div>
    </div>
  );
}
