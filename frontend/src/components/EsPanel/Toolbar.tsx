import { Button, Tooltip, Typography } from 'antd';
import { ReloadOutlined, RobotOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { hoststore } from '@wailsjs/go/models';

interface ToolbarProps {
  host: hoststore.TreeNode;
  onRefresh: () => void;
  aiCollapsed: boolean;
  onToggleAI: () => void;
}

/** ES 面板顶部工具栏：刷新 / 连接信息 / AI 按钮。 */
export default function Toolbar({
  host,
  onRefresh,
  aiCollapsed,
  onToggleAI,
}: ToolbarProps): React.JSX.Element {
  const { t } = useTranslation('es');
  const { t: tt } = useTranslation('terminal');
  const dbLabel = `${host.protocol} · ${host.user}@${host.addr}:${host.port}`;

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        padding: '6px 10px',
        borderBottom: '1px solid var(--antd-color-border-secondary)',
        flexShrink: 0,
      }}
    >
      <Tooltip title={t('toolbar.refresh')}>
        <Button size="small" icon={<ReloadOutlined />} onClick={onRefresh} />
      </Tooltip>
      <div style={{ flex: 1 }} />
      <Typography.Text strong style={{ fontSize: 12 }}>
        {host.name}
      </Typography.Text>
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        {dbLabel}
      </Typography.Text>
      <Tooltip title={aiCollapsed ? tt('header.aiClose') : tt('header.aiOpen')}>
        <Button
          type={aiCollapsed ? 'text' : 'primary'}
          size="small"
          icon={<RobotOutlined />}
          onClick={onToggleAI}
        />
      </Tooltip>
    </div>
  );
}
