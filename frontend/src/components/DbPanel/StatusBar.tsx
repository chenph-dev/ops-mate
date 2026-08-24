import { useTranslation } from 'react-i18next';

interface StatusBarProps {
  /** 连接/库描述文本（左侧）。 */
  label: string;
  /** 最近结果统计文本（右侧，可空→就绪）。 */
  resultInfo?: string;
}

/** 底部状态栏：左侧连接信息，右侧结果统计/就绪态。 */
export default function StatusBar({
  label,
  resultInfo,
}: StatusBarProps): React.JSX.Element {
  const { t } = useTranslation('hosts');
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 12,
        padding: '3px 10px',
        borderTop: '1px solid var(--antd-color-border-secondary)',
        fontSize: 11,
        color: 'var(--antd-color-text-secondary)',
        flexShrink: 0,
        minHeight: 24,
      }}
    >
      <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {label}
      </span>
      <span style={{ marginLeft: 'auto' }}>{resultInfo ?? t('db.status.ready')}</span>
    </div>
  );
}
