import { useState } from 'react';
import { Button, Tag, Input, Tooltip, theme } from 'antd';
import { useTranslation } from 'react-i18next';
import { CodeOutlined, EditOutlined } from '@ant-design/icons';
import type { ApprovalStatus, CommandSuggestion } from '@/hooks/useSessions';
import { isHighRiskCommand } from './risk';

interface CommandCardProps {
  command: CommandSuggestion;
  /** 审批调用是否进行中 */
  busy?: boolean;
  /** 历史回放模式：只展示，无操作按钮 */
  history?: boolean;
  /** 用户审批状态：待审批 / 已批准 / 已拒绝 */
  status?: ApprovalStatus;
  onApprove?: (command: string) => void;
  onReject?: () => void;
  /** 一键把命令发到右侧终端执行 */
  onRunInTerminal?: (command: string) => void;
}

// text 存 i18n key（ai 命名空间），渲染处用 t(text) 取当前语言文案
const STATUS_META: Record<ApprovalStatus, { text: string; color: string }> = {
  pending: { text: 'cmd.statusPending', color: 'processing' },
  approved: { text: 'cmd.statusApproved', color: 'success' },
  rejected: { text: 'cmd.statusRejected', color: 'error' },
  auto: { text: 'cmd.statusAuto', color: 'cyan' },
};

/** 命令审批卡：状态+原因一行、命令、操作按钮（紧凑布局）。 */
export default function CommandCard({
  command,
  busy,
  history,
  status,
  onApprove,
  onReject,
  onRunInTerminal,
}: CommandCardProps): React.JSX.Element {
  const { token } = theme.useToken();
  const { t } = useTranslation('ai');
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(command.command);
  const [confirming, setConfirming] = useState(false);
  // 编辑时用前端镜像对当前草稿实时重估风险（即时反馈）；
  // 未编辑时以后端守卫判定为准（历史/展示模式忠实于当时判定）。
  const isHighRisk = editing
    ? isHighRiskCommand(draft)
    : command.assessedRisk === 'high' || command.risk === 'high';
  // 已批准/已拒绝/已自动执行后不再可操作（执行中或结果已定）
  const done =
    status === 'approved' || status === 'rejected' || status === 'auto';

  const handleApprove = (): void => {
    // 高风险二次确认：首次点击只进入确认态，二次点击才批准
    if (isHighRisk && !confirming) {
      setConfirming(true);
      return;
    }
    onApprove?.(draft);
  };

  return (
    <div
      style={{
        border: `1px solid ${isHighRisk ? token.colorError : token.colorBorderSecondary}`,
        borderRadius: 6,
        padding: '6px 8px',
        background: token.colorBgElevated,
        margin: '2px 0',
      }}
    >
      {/* 头部一行：审批状态 + 原因 + 风险 */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          marginBottom: 4,
        }}
      >
        {status && (
          <Tag
            color={STATUS_META[status].color}
            style={{ margin: 0, fontSize: 11, lineHeight: '16px' }}
          >
            {t(STATUS_META[status].text)}
          </Tag>
        )}
        <span
          title={command.why}
          style={{
            flex: 1,
            minWidth: 0,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            fontSize: 11,
            color: token.colorTextSecondary,
          }}
        >
          {t('cmd.reason', { reason: command.why || '—' })}
        </span>
        {isHighRisk && (
          <Tag
            color="error"
            style={{ margin: 0, fontSize: 11, lineHeight: '16px' }}
          >
            {t('cmd.highRisk')}
          </Tag>
        )}
      </div>

      {/* 命令（编辑态为可编辑输入框） */}
      {editing && !history ? (
        <Input.TextArea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          autoSize={{ minRows: 1, maxRows: 3 }}
          style={{ fontFamily: 'monospace', fontSize: 11 }}
        />
      ) : (
        <div
          style={{
            fontFamily: '"Cascadia Code", "Fira Code", "Consolas", monospace',
            fontSize: 11,
            background: token.colorFillSecondary,
            padding: '3px 6px',
            borderRadius: 4,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
            color: token.colorText,
          }}
        >
          $ {draft}
        </div>
      )}

      {/* 操作按钮 */}
      {!history && !done && (
        <div
          style={{
            display: 'flex',
            justifyContent: 'flex-end',
            gap: 4,
            marginTop: 6,
          }}
        >
          {onRunInTerminal && (
            <Tooltip title={t('cmd.runInTerminal')}>
              <Button
                size="small"
                icon={<CodeOutlined />}
                onClick={() => onRunInTerminal(draft)}
                disabled={busy}
              />
            </Tooltip>
          )}
          <Tooltip title={editing ? t('cmd.editDone') : t('cmd.editModify')}>
            <Button
              size="small"
              icon={<EditOutlined />}
              onClick={() => {
                setEditing(!editing);
                // 命令变更后旧的二次确认态作废，重新点击才能再次确认
                setConfirming(false);
              }}
              disabled={busy}
            >
              {editing ? t('cmd.done') : t('cmd.edit')}
            </Button>
          </Tooltip>
          <Button size="small" onClick={() => onReject?.()} disabled={busy}>
            {t('cmd.reject')}
          </Button>
          <Button
            type="primary"
            size="small"
            danger={isHighRisk}
            loading={busy}
            onClick={handleApprove}
          >
            {isHighRisk && confirming ? t('cmd.confirmAgain') : t('cmd.approve')}
          </Button>
        </div>
      )}
    </div>
  );
}
