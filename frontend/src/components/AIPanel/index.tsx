import { Button, Input, Spin, Tooltip } from 'antd';
import { MessageOutlined, CompressOutlined, SendOutlined } from '@ant-design/icons';
import { useEffect, useRef, useState } from 'react';
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
}: AIPanelProps): React.JSX.Element {
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const msgRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (msgRef.current) {
      msgRef.current.scrollTop = msgRef.current.scrollHeight;
    }
  }, [messages, pendingCommand]);

  const handleSend = async (): Promise<void> => {
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

  const handleKeyDown = (e: React.KeyboardEvent): void => {
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
          background: 'var(--antd-color-bg-elevated)',
          border: '1px solid var(--antd-color-border)',
          borderRadius: 20,
          padding: '8px 14px',
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          boxShadow: 'var(--antd-box-shadow-secondary)',
          zIndex: 100,
          fontSize: 13,
          color: 'var(--antd-color-text)',
        }}
      >
        <MessageOutlined style={{ color: 'var(--antd-color-primary)' }} />
        <span>AI</span>
        {messages.length > 0 && (
          <span
            style={{
              background: 'var(--antd-color-primary)',
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
        background: 'var(--antd-color-bg-elevated)',
        borderTop: '1px solid var(--antd-color-border)',
        boxShadow: 'var(--antd-box-shadow)',
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
          borderBottom: '1px solid var(--antd-color-border-secondary)',
          flexShrink: 0,
          background: 'var(--antd-color-bg-elevated)',
          color: 'var(--antd-color-text)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <MessageOutlined style={{ color: 'var(--antd-color-primary)' }} />
          <span style={{ fontSize: 12, fontWeight: 600 }}>AI 助手</span>
          <span style={{ fontSize: 11, color: 'var(--antd-color-text-secondary)' }}>· {hostName}</span>
          {sessionState && (
            <span style={{ fontSize: 11, color: 'var(--antd-color-warning)' }}>[{sessionState}]</span>
          )}
        </div>
        <Tooltip title="收起">
          <Button type="text" size="small" icon={<CompressOutlined />} onClick={onToggleCollapse} />
        </Tooltip>
      </div>

      {/* 消息区 */}
      <div ref={msgRef} style={{ flex: 1, overflow: 'auto', padding: '8px 12px', background: 'var(--antd-color-bg-elevated)' }}>
        {messages.length === 0 && !pendingCommand ? (
          <div style={{ color: 'var(--antd-color-text-secondary)', fontSize: 12, padding: 16, textAlign: 'center' }}>
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
                    background: msg.role === 'user' ? 'var(--antd-color-primary)' : 'var(--antd-color-fill-secondary)',
                    color: msg.role === 'user' ? '#fff' : 'var(--antd-color-text)',
                  }}
                >
                  <div style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>{msg.content}</div>
                  {msg.toolResult && (
                    <div
                      style={{
                        marginTop: 4,
                        padding: '3px 6px',
                        background: 'rgba(0,0,0,0.15)',
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
          borderTop: '1px solid var(--antd-color-border-secondary)',
          display: 'flex',
          gap: 6,
          alignItems: 'flex-end',
          flexShrink: 0,
          background: 'var(--antd-color-bg-elevated)',
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
