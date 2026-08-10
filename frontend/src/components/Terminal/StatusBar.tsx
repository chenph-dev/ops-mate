import { theme } from 'antd';
import { useTranslation } from 'react-i18next';

interface StatusBarProps {
  statusText: string;
  dims: { cols: number; rows: number };
  fontSize: number;
  hostAddr: string;
}

/** 终端底部状态栏。 */
export default function StatusBar({
  statusText,
  dims,
  fontSize,
  hostAddr,
}: StatusBarProps): React.JSX.Element {
  const { token } = theme.useToken();
  const { t } = useTranslation('terminal');
  return (
    <div
      style={{
        padding: '4px 10px',
        display: 'flex',
        alignItems: 'center',
        gap: 16,
        borderTop: `1px solid ${token.colorBorderSecondary}`,
        flexShrink: 0,
        fontSize: 11,
        color: token.colorTextTertiary,
        background: token.colorBgElevated,
      }}
    >
      <span>{statusText}</span>
      <span>
        {dims.cols}×{dims.rows}
      </span>
      <span>{t('statusbar.fontSize', { size: fontSize })}</span>
      <span style={{ marginLeft: 'auto' }}>{hostAddr || t('status.disconnected')}</span>
    </div>
  );
}
