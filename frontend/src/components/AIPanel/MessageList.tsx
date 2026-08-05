import { useEffect, useRef } from "react";
import { Button, Spin } from "antd";
import { SettingOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import CommandCard from "@/components/CommandCard";
import type {
  ApprovalStatus,
  CommandSuggestion,
  Message,
} from "./types";
import ToolOutputBlock from "./ToolOutputBlock";

interface MessageListProps {
  messages: Message[];
  streamingText: string;
  pendingCommand: CommandSuggestion | null;
  commandStatus: ApprovalStatus | null;
  lastError: string | null;
  busy: boolean;
  configured: boolean;
  cfgLoading: boolean;
  onApprove: (command: string) => Promise<void>;
  onReject: () => Promise<void>;
}

/** 从 assistant 消息的 toolCalls JSON 解析出命令建议，并据相邻 tool 消息推断审批状态。 */
function parseToolCallCommand(
  msg: Message,
  nextMsg?: Message,
): (CommandSuggestion & { status: ApprovalStatus }) | null {
  if (!msg.toolCalls) return null;
  try {
    const calls = JSON.parse(msg.toolCalls) as Array<{
      id?: string;
      arguments: string;
    }>;
    if (calls.length === 0) return null;
    const args = JSON.parse(calls[0].arguments) as {
      command?: string;
      why?: string;
    };
    if (!args.command) return null;
    // 有相邻且同 ID 的 tool 消息 = 该命令已被处理过；审批状态直接读落库字段。
    let status: ApprovalStatus = "pending";
    if (nextMsg?.role === "tool" && nextMsg.toolCallId === calls[0].id) {
      status = nextMsg.approvalStatus === "rejected" ? "rejected" : "approved";
    }
    return {
      command: args.command,
      why: args.why ?? "",
      risk: "",
      assessedRisk: "",
      status,
    };
  } catch {
    return null;
  }
}

export default function MessageList({
  messages,
  streamingText,
  pendingCommand,
  commandStatus,
  lastError,
  busy,
  configured,
  cfgLoading,
  onApprove,
  onReject,
}: MessageListProps): React.JSX.Element {
  const navigate = useNavigate();
  const msgRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (msgRef.current) {
      msgRef.current.scrollTop = msgRef.current.scrollHeight;
    }
  }, [messages, streamingText, pendingCommand]);

  const renderMessage = (msg: Message, index: number): React.JSX.Element => {
    if (msg.role === "user") {
      return (
        <div
          key={msg.id}
          style={{
            display: "flex",
            justifyContent: "flex-end",
            marginBottom: 6,
          }}
        >
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
      const suggested = parseToolCallCommand(msg, messages[index + 1]);
      if (suggested) {
        // 历史命令提议（回放模式，无操作按钮，显示审批状态）
        return (
          <CommandCard
            key={msg.id}
            command={suggested}
            history
            status={suggested.status}
          />
        );
      }
      return (
        <div
          key={msg.id}
          style={{
            display: "flex",
            justifyContent: "flex-start",
            marginBottom: 6,
          }}
        >
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

    // tool 消息：命令执行输出，默认折叠
    return <ToolOutputBlock key={msg.id} content={msg.content} />;
  };

  return (
    <div ref={msgRef} style={{ flex: 1, overflow: "auto", padding: "8px 12px" }}>
      {!configured && !cfgLoading ? (
        // 未配置 AI：引导去配置页
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            height: "100%",
            gap: 12,
            padding: 24,
          }}
        >
          <div
            style={{
              fontSize: 12,
              color: "var(--antd-color-text-secondary)",
              textAlign: "center",
            }}
          >
            尚未配置 AI 后端。请在「AI 配置」页设置
            API 协议、Base URL、API Key 与模型。
          </div>
          <Button
            type="primary"
            size="small"
            icon={<SettingOutlined />}
            onClick={() => navigate("/config")}
          >
            前往配置
          </Button>
        </div>
      ) : cfgLoading ? (
        <div style={{ display: "flex", justifyContent: "center", padding: 24 }}>
          <Spin size="small" />
        </div>
      ) : messages.length === 0 && !streamingText && !pendingCommand ? (
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
          {messages.map((msg, index) => renderMessage(msg, index))}

          {/* 流式气泡 */}
          {streamingText && (
            <div
              style={{
                display: "flex",
                justifyContent: "flex-start",
                marginBottom: 6,
              }}
            >
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
              busy={commandStatus === "approved"}
              status={commandStatus ?? "pending"}
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
  );
}
