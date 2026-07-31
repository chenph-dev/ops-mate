import { Button, Input, Spin, Tooltip, theme, Divider } from 'antd';
import { MessageOutlined, CloseOutlined, ExpandOutlined, CompressOutlined, SendOutlined } from '@ant-design/icons';
import { useEffect, useRef, useState, useCallback } from 'react';
import type { convstore } from '@wailsjs/go/models';
import type { CommandSuggestion } from '@/hooks/useSessions';
import CommandCard from '@/components/AIChat/CommandCard';

type Message = convstore.Message;

interface AIPanelProps {
  messages: Message[];
  pendingCommand: CommandSuggestion | null;
  sessionState: string | null;
  hostName: string;
  collapsed: boolean;
  onToggleCollapse: () => void;
  onSendMessage: (text: string) => Promise<void>;
  onApprove: (command: string) => Promise<void>;
  onReject: () => Promise<void>;
}

export default function AIPanel({
  messages,
  pendingCommand,
  sessionState,
  hostName,
  collapsed,
  onToggleCollapse,
  onSendMessage,
  onApprove,
  onReject,
}: AIPanelProps) {
  const { token } = theme.useToken();
  const [input, setInput] = useState('');
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
    setInput('');
    setSending(true);
    try {
      await onSendMessage(text);
    } finally {
      setSending(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  // 悬浮态：右下角小按钮
  if (collapsed) {
    return (
      <div
        onClick={onToggleCollapse}
        style={{
          position: 'absolute',
          bottom: 12,
          right: 12,
          background: token.colorBgElevated,
          border: `1px solid ${token.colorBorder}`,
          borderRadius: 20,
          padding: '8px 14px',
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
          zIndex: 100,
          fontSize: 13,
        }}
      >
        <MessageOutlined style={{ color: token.colorPrimary }} />
        <span>AI</span>
        {messages.length > 0 && (
          <span
            style={{
              background: token.colorPrimary,
              color: '#fff',
              borderRadius: 10,
              padding: '0 6px',
              fontSize: 11,
              lineHeight: '18px',
            }}
          >
            {messages.length}
          </span>
        )}
      </div>
    );
  }

  // 展开态：悬浮在终端上方的抽屉
  return (
    <div
      style={{
        position: 'absolute',
        bottom: 0,
        left: 0,
        right: 0,
        height: 360,
        display: 'flex',
        flexDirection: 'column',
        background: token.colorBgElevated,
        borderTop: `1px solid ${token.colorBorder}`,
        boxShadow: '0 -2px 12px rgba(0,0,0,0.15)',
        zIndex: 99,
        borderTopLeftRadius: 8,
        borderTopRightRadius: 8,
      }}
    >
      {/* 标题栏 */}
      <div
        style={{
          padding: '6px 10px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: `1px solid ${token.colorBorderSecondary}`,
          flexShrink: 0,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <MessageOutlined style={{ color: token.colorPrimary }} />
          <span style={{ fontSize: 12, fontWeight: 600 }}>AI 助手</span>
          <span style={{ fontSize: 11, color: token.colorTextSecondary }}>· {hostName}</span>
          {sessionState && (
            <span style={{ fontSize: 11, color: token.colorWarning }}>[{sessionState}]</span>
          )}
        </div>
        <Tooltip title="收起">
          <Button type="text" size="small" icon={<CompressOutlined />} onClick={onToggleCollapse} />
        </Tooltip>
      </div>

      {/* 消息区 */}
      <div ref={msgRef} style={{ flex: 1, overflow: 'auto', padding: '8px 12px' }}>
        {messages.length === 0 && !pendingCommand ? (
          <div style={{ color: token.colorTextSecondary, fontSize: 12, padding: 16, textAlign: 'center' }}>
            发送消息开始对话...
          </div>
        ) : (
          <>
            {messages.map((msg) => (
              <div
                key={msg.id}
                style={{
                  display: 'flex',
                  justifyContent: msg.role === 'user' ? 'flex-end' : 'flex-start',
                  marginBottom: 6,
                }}
              >
                <div
                  style={{
                    maxWidth: '85%',
                    padding: '5px 10px',
                    borderRadius: 8,
                    fontSize: 12,
                    lineHeight: 1.5,
                    background: msg.role === 'user' ? token.colorPrimary : token.colorFillSecondary,
                    color: msg.role === 'user' ? '#fff' : token.colorText,
                  }}
                >
                  <div style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>{msg.content}</div>
                  {msg.toolResult && (
                    <div
                      style={{
                        marginTop: 4,
                        padding: '3px 6px',
                        background: 'rgba(0,0,0,0.1)',
                        borderRadius: 4,
                        fontFamily: 'monospace',
                        fontSize: 11,
                        whiteSpace: 'pre-wrap',
                      }}
                    >
                      {msg.toolResult}
                    </div>
                  )}
                </div>
              </div>
            ))}
            {pendingCommand && (
              <CommandCard
                command={pendingCommand}
                onApprove={() => onApprove(pendingCommand.command)}
                onReject={onReject}
              />
            )}
            {sending && (
              <div style={{ display: 'flex', justifyContent: 'flex-start' }}>
                <Spin size="small" />
              </div>
            )}
          </>
        )}
      </div>

      {/* 输入区 */}
      <div
        style={{
          padding: '6px 8px',
          borderTop: `1px solid ${token.colorBorderSecondary}`,
          display: 'flex',
          gap: 6,
          alignItems: 'flex-end',
          flexShrink: 0,
        }}
      >
        <Input.TextArea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="输入问题..."
          autoSize={{ minRows: 1, maxRows: 3 }}
          style={{ fontSize: 12 }}
        />
        <Button
          type="primary"
          size="small"
          icon={<SendOutlined />}
          onClick={handleSend}
          disabled={!input.trim() || sending}
          loading={sending}
        />
      </div>
    </div>
  );
}
