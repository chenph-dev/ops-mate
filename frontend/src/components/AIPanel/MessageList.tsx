import { useEffect, useRef } from 'react';
import { Button, Spin } from 'antd';
import { CopyOutlined, SettingOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ClipboardSetText } from '@wailsjs/runtime/runtime';
import CommandCard from '@/components/CommandCard';
import PlanCard from '@/components/PlanCard';
import MarkdownContent from '@/components/MarkdownContent';
import type { ApprovalStatus, CommandSuggestion, Message } from './types';
import type { PlanInfo } from '@/hooks/useSessions';
import ToolOutputBlock from './ToolOutputBlock';

/** 空态示例 prompt：点击填充输入框（暗示半自动模型：可提议命令需审批）。值为 ai 命名空间下的 i18n key。
 *  按资产协议分组：ssh → Linux 命令；winrm → PowerShell；mysql/postgres/sqlite → SQL；redis → Redis 命令。 */
const SUGGESTIONS_BY_PROTOCOL: Record<string, string[]> = {
  ssh: [
    'empty.suggestDisk',
    'empty.suggestLoad',
    'empty.suggestLogs',
    'empty.suggestDocker',
  ],
  winrm: [
    'empty.suggestWinrmService',
    'empty.suggestWinrmSysinfo',
    'empty.suggestWinrmEventlog',
    'empty.suggestWinrmDisk',
  ],
  mysql: ['empty.suggestDbTables', 'empty.suggestDbSchema', 'empty.suggestDbCount', 'empty.suggestDbInfo'],
  postgres: ['empty.suggestDbTables', 'empty.suggestDbSchema', 'empty.suggestDbCount', 'empty.suggestDbInfo'],
  sqlite: ['empty.suggestDbTables', 'empty.suggestDbSchema', 'empty.suggestDbCount', 'empty.suggestDbInfo'],
  redis: ['empty.suggestRedisInfo', 'empty.suggestRedisDbsize', 'empty.suggestRedisKeys', 'empty.suggestRedisMemory'],
  elasticsearch: [
    'empty.suggestEsHealth',
    'empty.suggestEsIndices',
    'empty.suggestEsDocs',
    'empty.suggestEsSearch',
  ],
};
const DEFAULT_SUGGESTIONS = SUGGESTIONS_BY_PROTOCOL.ssh;

interface MessageListProps {
  messages: Message[];
  streamingText: string;
  pendingCommand: CommandSuggestion | null;
  commandStatus: ApprovalStatus | null;
  pendingPlan: PlanInfo | null;
  planStatus: ApprovalStatus | null;
  lastError: string | null;
  lastErrorCancelled: boolean;
  busy: boolean;
  configured: boolean;
  cfgLoading: boolean;
  /** 当前资产协议（ssh/winrm/mysql/postgres/sqlite/redis），决定空态建议命令。 */
  protocol: string;
  runningCommand: string | null;
  runElapsed: number;
  runOutput: string;
  onApprove: (command: string) => Promise<void>;
  onReject: () => Promise<void>;
  onApprovePlan: () => Promise<void>;
  onRejectPlan: () => Promise<void>;
  onSelectSuggestion: (text: string) => void;
  onRunInTerminal: (command: string) => void;
}

/** 从 assistant 消息的 toolCalls JSON 解析出执行计划（create_plan），并据相邻 tool 消息推断审批状态。 */
function parseToolCallPlan(
  msg: Message,
  nextMsg?: Message,
): (PlanInfo & { status: ApprovalStatus }) | null {
  if (!msg.toolCalls) return null;
  try {
    const calls = JSON.parse(msg.toolCalls) as Array<{
      id?: string;
      name?: string;
      arguments: string;
    }>;
    if (calls.length === 0) return null;
    const first = calls[0];
    if (first.name !== 'create_plan') return null;
    const args = JSON.parse(first.arguments) as {
      goal?: string;
      steps?: string[];
    };
    if (!args.goal || !Array.isArray(args.steps)) return null;
    // 有相邻且同 ID 的 tool 消息 = 该计划已被处理过；审批状态直接读落库字段。
    let status: ApprovalStatus = 'pending';
    if (nextMsg?.role === 'tool' && nextMsg.toolCallId === first.id) {
      status = nextMsg.approvalStatus === 'rejected' ? 'rejected' : 'approved';
    }
    return { goal: args.goal, steps: args.steps, status };
  } catch {
    return null;
  }
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
    let status: ApprovalStatus = 'pending';
    if (nextMsg?.role === 'tool' && nextMsg.toolCallId === calls[0].id) {
      status =
        nextMsg.approvalStatus === 'rejected'
          ? 'rejected'
          : nextMsg.approvalStatus === 'auto'
            ? 'auto'
            : 'approved';
    }
    return {
      command: args.command,
      why: args.why ?? '',
      risk: '',
      assessedRisk: '',
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
  pendingPlan,
  planStatus,
  lastError,
  lastErrorCancelled,
  busy,
  configured,
  cfgLoading,
  protocol,
  runningCommand,
  runElapsed,
  runOutput,
  onApprove,
  onReject,
  onApprovePlan,
  onRejectPlan,
  onSelectSuggestion,
  onRunInTerminal,
}: MessageListProps): React.JSX.Element {
  const navigate = useNavigate();
  const { t } = useTranslation('ai');
  // 按资产协议选空态建议命令；未知协议回退 Linux。
  const suggestions = SUGGESTIONS_BY_PROTOCOL[protocol] ?? DEFAULT_SUGGESTIONS;
  const msgRef = useRef<HTMLDivElement>(null);
  const runOutputRef = useRef<HTMLDivElement>(null);

  const copyText = (text: string): void => {
    void ClipboardSetText(text);
  };

  // 实时输出增量时，输出块内部自动滚动到底部（不影响用户上翻历史）
  useEffect(() => {
    if (runOutputRef.current) {
      runOutputRef.current.scrollTop = runOutputRef.current.scrollHeight;
    }
  }, [runOutput]);

  useEffect(() => {
    if (msgRef.current) {
      msgRef.current.scrollTop = msgRef.current.scrollHeight;
    }
    // runElapsed 每秒变化不入依赖：命令运行中不打断用户向上翻看历史，
    // 仅命令开始（runningCommand 从 null 变为命令）时滚动到底部展示执行条。
  }, [messages, streamingText, pendingCommand, runningCommand]);

  const renderMessage = (msg: Message, index: number): React.JSX.Element => {
    if (msg.role === 'user') {
      return (
        <div
          key={msg.id}
          style={{
            display: 'flex',
            justifyContent: 'flex-end',
            marginBottom: 6,
            gap: 4,
            alignItems: 'center',
          }}
        >
          <Button
            type="text"
            size="small"
            icon={<CopyOutlined />}
            title={t('copyMsg')}
            onClick={() => copyText(msg.content)}
          />
          <div
            style={{
              maxWidth: '85%',
              padding: '5px 10px',
              borderRadius: 8,
              fontSize: 12,
              lineHeight: 1.5,
              background: 'var(--antd-color-primary)',
              color: '#fff',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
            }}
          >
            {msg.content}
          </div>
        </div>
      );
    }

    if (msg.role === 'assistant') {
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
      const plan = parseToolCallPlan(msg, messages[index + 1]);
      if (plan) {
        // 历史执行计划（回放模式，无操作按钮，显示审批状态）
        return (
          <PlanCard
            key={msg.id}
            plan={{ goal: plan.goal, steps: plan.steps }}
            history
            status={plan.status}
          />
        );
      }
      return (
        <div
          key={msg.id}
          style={{
            display: 'flex',
            justifyContent: 'flex-start',
            marginBottom: 6,
            gap: 4,
            alignItems: 'flex-start',
          }}
        >
          <div
            style={{
              maxWidth: '85%',
              padding: '5px 10px',
              borderRadius: 8,
              background: 'var(--antd-color-fill-secondary)',
            }}
          >
            {/* 已完成的智能体消息：Markdown 渲染（表格/代码块/列表） */}
            <MarkdownContent content={msg.content} />
          </div>
          <Button
            type="text"
            size="small"
            icon={<CopyOutlined />}
            title={t('copyMsg')}
            onClick={() => copyText(msg.content)}
          />
        </div>
      );
    }

    // tool 消息：命令执行输出，默认折叠（含结构化元数据头部）
    return (
      <ToolOutputBlock
        key={msg.id}
        content={msg.content}
        toolResult={msg.toolResult}
      />
    );
  };

  return (
    <div
      ref={msgRef}
      style={{ flex: 1, overflow: 'auto', padding: '8px 12px' }}
    >
      {!configured && !cfgLoading ? (
        // 未配置 AI：引导去配置页
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            gap: 12,
            padding: 24,
          }}
        >
          <div
            style={{
              fontSize: 12,
              color: 'var(--antd-color-text-secondary)',
              textAlign: 'center',
            }}
          >
            {t('unconfigured.hint')}
          </div>
          <Button
            type="primary"
            size="small"
            icon={<SettingOutlined />}
            onClick={() => navigate('/config')}
          >
            {t('unconfigured.go')}
          </Button>
        </div>
      ) : cfgLoading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: 24 }}>
          <Spin size="small" />
        </div>
      ) : messages.length === 0 &&
        !streamingText &&
        !pendingCommand &&
        !pendingPlan &&
        !runningCommand ? (
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: 8,
            padding: 16,
          }}
        >
          <div
            style={{
              color: 'var(--antd-color-text-secondary)',
              fontSize: 12,
            }}
          >
            {t('empty.hint')}
          </div>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '1fr 1fr',
              gap: 6,
              maxWidth: 360,
              width: '100%',
            }}
          >
            {suggestions.map((s) => (
              <div
                key={s}
                onClick={() => onSelectSuggestion(t(s))}
                title={t('empty.clickHint')}
                style={{
                  border: '1px solid var(--antd-color-border-secondary)',
                  borderRadius: 6,
                  padding: '6px 8px',
                  fontSize: 11,
                  lineHeight: 1.5,
                  color: 'var(--antd-color-text)',
                  cursor: 'pointer',
                  background: 'var(--antd-color-bg-elevated)',
                }}
              >
                {t(s)}
              </div>
            ))}
          </div>
        </div>
      ) : (
        <>
          {messages.map((msg, index) => renderMessage(msg, index))}

          {/* 流式气泡 */}
          {streamingText && (
            <div
              style={{
                display: 'flex',
                justifyContent: 'flex-start',
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
                  background: 'var(--antd-color-fill-secondary)',
                  color: 'var(--antd-color-text)',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
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
              busy={commandStatus === 'approved'}
              status={commandStatus ?? 'pending'}
              onApprove={(cmd) => void onApprove(cmd)}
              onReject={() => void onReject()}
              onRunInTerminal={onRunInTerminal}
            />
          )}

          {/* 计划审批卡（计划模式）：批准后模型按计划逐步执行。
              状态走独立 planStatus，避免被后续 ai:command 的命令审批状态污染。 */}
          {pendingPlan && (
            <PlanCard
              plan={pendingPlan}
              busy={planStatus === 'approved'}
              status={planStatus ?? 'pending'}
              onApprove={() => void onApprovePlan()}
              onReject={() => void onRejectPlan()}
            />
          )}

          {/* 错误提示；主动取消（后端 cancelled 标记）属正常反馈，用中性色而非红色 */}
          {lastError && (
            <div
              style={{
                fontSize: 12,
                color: lastErrorCancelled
                  ? 'var(--antd-color-text-secondary)'
                  : 'var(--antd-color-error)',
                padding: '4px 8px',
              }}
            >
              {lastError}
            </div>
          )}

          {busy && !streamingText && (
            <div style={{ display: 'flex', justifyContent: 'flex-start' }}>
              <Spin size="small" />
            </div>
          )}

          {/* 执行中：命令正在运行，展示在消息流末尾（含实时输出增量） */}
          {runningCommand && (
            <>
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 6,
                  marginTop: 6,
                }}
              >
                <Spin size="small" />
                <span
                  title={runningCommand}
                  style={{
                    fontFamily:
                      '"Cascadia Code", "Fira Code", "Consolas", monospace',
                    fontSize: 11,
                    background: 'rgba(0,0,0,0.25)',
                    border: '1px solid var(--antd-color-border-secondary)',
                    borderRadius: 4,
                    padding: '2px 8px',
                    color: 'var(--antd-color-text-secondary)',
                    maxWidth: '75%',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}
                >
                  $ {runningCommand}
                </span>
                <span
                  style={{
                    fontSize: 11,
                    color: 'var(--antd-color-text-secondary)',
                  }}
                >
                  {runElapsed}s
                </span>
              </div>
              {/* 实时输出增量（run:output 累积），执行完成由完整 tool 消息接管 */}
              {runOutput && (
                <div
                  ref={runOutputRef}
                  style={{
                    fontFamily:
                      '"Cascadia Code", "Fira Code", "Consolas", monospace',
                    fontSize: 11,
                    background: 'rgba(0,0,0,0.25)',
                    border: '1px solid var(--antd-color-border-secondary)',
                    borderRadius: 4,
                    padding: '4px 8px',
                    marginTop: 4,
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-all',
                    maxHeight: 160,
                    overflow: 'auto',
                    color: 'var(--antd-color-text)',
                  }}
                >
                  {runOutput}
                </div>
              )}
            </>
          )}
        </>
      )}
    </div>
  );
}
