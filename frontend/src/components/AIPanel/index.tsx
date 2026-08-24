import { useCallback, useEffect, useRef, useState } from 'react';
import type { InputRef } from 'antd';
import { useTranslation } from 'react-i18next';
import { GetAIConfig } from '@wailsjs/go/aiconfig/AIConfigHandler';
import type { configstore, convstore } from '@wailsjs/go/models';
import type {
  ApprovalStatus,
  CommandSuggestion,
  Message,
  SessionState,
} from './types';
import type { PlanInfo } from '@/hooks/useSessions';
import { STATE_LABEL } from './types';
import PanelHeader from './PanelHeader';
import MessageList from './MessageList';
import PanelInput from './PanelInput';

export interface AIPanelProps {
  activeSession: string | null;
  messages: Message[];
  conversations: convstore.Conversation[];
  streamingText: string;
  pendingCommand: CommandSuggestion | null;
  commandStatus: ApprovalStatus | null;
  pendingPlan: PlanInfo | null;
  planStatus: ApprovalStatus | null;
  sessionState: SessionState;
  lastError: string | null;
  lastErrorCancelled: boolean;
  runningCommand: string | null;
  runElapsed: number;
  runOutput: string;
  sshConnected: boolean;
  hostName: string;
  /** 当前资产协议（ssh/winrm/mysql/postgres/sqlite/redis），决定空态建议命令。 */
  protocol: string;
  collapsed: boolean;
  onRefreshConversations: () => Promise<void>;
  onSwitchConversation: (sid: string) => Promise<void>;
  onDeleteConversation: (sid: string) => Promise<void>;
  onRenameConversation: (sid: string, title: string) => Promise<void>;
  onToggleCollapse: () => void;
  onSendMessage: (text: string) => Promise<void>;
  onClearMessages: () => Promise<void>;
  onRunInTerminal: (command: string) => void;
  onApprove: (command: string) => Promise<void>;
  onReject: () => Promise<void>;
  onApprovePlan: () => Promise<void>;
  onRejectPlan: () => Promise<void>;
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
  pendingPlan,
  planStatus,
  sessionState,
  lastError,
  lastErrorCancelled,
  runningCommand,
  runElapsed,
  runOutput,
  sshConnected,
  hostName,
  protocol,
  collapsed,
  onRefreshConversations,
  onSwitchConversation,
  onDeleteConversation,
  onRenameConversation,
  onToggleCollapse,
  onSendMessage,
  onClearMessages,
  onRunInTerminal,
  onApprove,
  onReject,
  onApprovePlan,
  onRejectPlan,
  onCancel,
  onNewConversation,
}: AIPanelProps): React.JSX.Element | null {
  const { t } = useTranslation('ai');
  const { t: tc } = useTranslation('common');
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [aiCfg, setAiCfg] = useState<configstore.AIConfig | null>(null);
  const [cfgLoading, setCfgLoading] = useState(true);
  const [resizeHover, setResizeHover] = useState(false);
  const [panelWidth, setPanelWidth] = useState<number>(() => {
    const saved = Number(localStorage.getItem('ai-panel-width'));
    return saved >= 280 && saved <= 600 ? saved : 420;
  });
  const resizeRef = useRef<{ startX: number; startW: number } | null>(null);
  const inputRef = useRef<InputRef>(null);

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
        localStorage.setItem('ai-panel-width', String(next));
      };
      const onUp = (): void => {
        resizeRef.current = null;
        window.removeEventListener('mousemove', onMove);
        window.removeEventListener('mouseup', onUp);
      };
      window.addEventListener('mousemove', onMove);
      window.addEventListener('mouseup', onUp);
    },
    [panelWidth],
  );

  const configured = !!aiCfg && !!aiCfg.provider && !!aiCfg.model;
  const busy = sessionState === 'Thinking' || sessionState === 'Running';
  const inputDisabled =
    busy || sessionState === 'AwaitingApproval' || !configured;

  // 展开抽屉时拉取 LLM 模型配置：已配置在标题显示模型；未配置给出提醒与跳转。
  // 从「LLM模型配置」页保存后返回本页，AIPanel 重新挂载，collapsed 回到初始态，
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
    setInput('');
    // 斜杠快捷命令：不发给模型
    if (text === '/clear') {
      await onClearMessages();
      return;
    }
    if (text === '/new') {
      await onNewConversation();
      return;
    }
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
      void handleSend();
    }
  };

  // 斜杠命令菜单项点击：清空输入并直接执行对应快捷命令。
  const handleSlashCommand = (cmd: string): void => {
    setInput('');
    if (cmd === '/clear') void onClearMessages();
    else if (cmd === '/new') void onNewConversation();
  };

  // 折叠态：入口已移到终端右上角工具栏（TerminalHeader 的 AI 开关按钮），
  // 本组件不占位，仅展开时渲染并排面板。
  if (collapsed) return null;

  const stateMeta = sessionState ? STATE_LABEL[sessionState] : null;

  return (
    <div
      style={{
        width: panelWidth,
        height: '100%',
        flexShrink: 0,
        display: 'flex',
        flexDirection: 'column',
        position: 'relative',
        background: 'var(--antd-color-bg-elevated)',
        // 与终端风格一致：同边框、同圆角，overflow hidden 裁切内容到圆角内
        border: '1px solid var(--antd-color-border-secondary)',
        borderRadius: 8,
        overflow: 'hidden',
        marginLeft: 5,
      }}
    >
      {/* 左边缘拖拽条：调整面板宽度（与终端并排，终端自动让出空间） */}
      <div
        onMouseDown={onResizeStart}
        onMouseEnter={() => setResizeHover(true)}
        onMouseLeave={() => setResizeHover(false)}
        title={tc('dragResize')}
        style={{
          position: 'absolute',
          left: 0,
          top: 0,
          bottom: 0,
          width: 5,
          cursor: 'ew-resize',
          background: resizeHover ? 'var(--antd-color-primary)' : 'transparent',
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
        sshConnected={sshConnected}
        conversations={conversations}
        activeSession={activeSession}
        onSwitchConversation={onSwitchConversation}
        onDeleteConversation={onDeleteConversation}
        onRenameConversation={onRenameConversation}
        onRefreshConversations={onRefreshConversations}
        onCancel={onCancel}
        onNewConversation={onNewConversation}
        onToggleCollapse={onToggleCollapse}
      />
      {/* SSH 断开警告：AI 命令依赖资产连接 */}
      {!sshConnected && (
        <div
          style={{
            padding: '4px 10px',
            fontSize: 11,
            lineHeight: 1.5,
            background: 'var(--antd-color-warning-bg)',
            borderBottom: '1px solid var(--antd-color-warning-border)',
            color: 'var(--antd-color-warning-text)',
          }}
        >
          {t('ssh.disconnected')}
        </div>
      )}
      <MessageList
        messages={messages}
        streamingText={streamingText}
        pendingCommand={pendingCommand}
        commandStatus={commandStatus}
        pendingPlan={pendingPlan}
        planStatus={planStatus}
        lastError={lastError}
        lastErrorCancelled={lastErrorCancelled}
        busy={busy}
        configured={configured}
        cfgLoading={cfgLoading}
        protocol={protocol}
        runningCommand={runningCommand}
        runElapsed={runElapsed}
        runOutput={runOutput}
        onApprove={onApprove}
        onReject={onReject}
        onApprovePlan={onApprovePlan}
        onRejectPlan={onRejectPlan}
        onSelectSuggestion={(text) => {
          setInput(text);
          requestAnimationFrame(() => inputRef.current?.focus());
        }}
        onRunInTerminal={onRunInTerminal}
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
        onSlashCommand={handleSlashCommand}
      />
    </div>
  );
}
