import { Button, Tag, theme } from 'antd';
import { useTranslation } from 'react-i18next';
import { ExperimentOutlined } from '@ant-design/icons';
import type { PlanInfo } from '@/hooks/useSessions';

// text 存 i18n key（ai 命名空间），渲染处用 t(text) 取当前语言文案
const STATUS_META: Record<string, { text: string; color: string }> = {
  pending: { text: 'plan.statusPending', color: 'processing' },
  approved: { text: 'plan.statusApproved', color: 'success' },
  rejected: { text: 'plan.statusRejected', color: 'error' },
};

interface PlanCardProps {
  plan: PlanInfo;
  /** 审批调用是否进行中 */
  busy?: boolean;
  /** 历史回放模式：只展示，无操作按钮 */
  history?: boolean;
  /** 计划审批状态 */
  status?: string;
  onApprove?: () => void;
  onReject?: () => void;
}

/** 执行计划审批卡：展示目标 + 步骤列表，批准后模型按计划逐步执行。 */
export default function PlanCard({
  plan,
  busy,
  history,
  status,
  onApprove,
  onReject,
}: PlanCardProps): React.JSX.Element {
  const { token } = theme.useToken();
  const { t } = useTranslation('ai');
  const meta = STATUS_META[status ?? ''] ?? STATUS_META.pending;
  const done = status === 'approved' || status === 'rejected';

  return (
    <div
      style={{
        border: `1px solid ${token.colorPrimaryBorder}`,
        borderRadius: 6,
        padding: '6px 8px',
        background: token.colorBgElevated,
        margin: '2px 0',
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          marginBottom: 4,
        }}
      >
        <ExperimentOutlined
          style={{ color: token.colorPrimary, fontSize: 12 }}
        />
        <span style={{ fontWeight: 600, fontSize: 12 }}>{t('plan.title')}</span>
        <Tag
          color={meta.color}
          style={{ margin: 0, fontSize: 11, lineHeight: '16px' }}
        >
          {t(meta.text)}
        </Tag>
      </div>

      <div
        style={{
          fontSize: 12,
          color: token.colorTextSecondary,
          marginBottom: 4,
        }}
      >
        {t('plan.goal', { goal: plan.goal })}
      </div>

      <ol
        style={{
          margin: 0,
          paddingLeft: 18,
          fontSize: 12,
          lineHeight: 1.7,
          color: token.colorText,
        }}
      >
        {plan.steps.map((s, i) => (
          <li key={i}>{s}</li>
        ))}
      </ol>

      {!history && !done && (
        <div
          style={{
            display: 'flex',
            justifyContent: 'flex-end',
            gap: 4,
            marginTop: 6,
          }}
        >
          <Button size="small" onClick={() => onReject?.()} disabled={busy}>
            {t('plan.reject')}
          </Button>
          <Button
            type="primary"
            size="small"
            loading={busy}
            onClick={() => onApprove?.()}
          >
            {t('plan.approve')}
          </Button>
        </div>
      )}
    </div>
  );
}
