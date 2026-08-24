import { Button, Tooltip, Typography } from 'antd';
import {
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
  RobotOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { hoststore } from '@wailsjs/go/models';

interface ToolbarProps {
  host: hoststore.TreeNode;
  onNewQuery: () => void;
  /** 执行当前活动查询标签（仅 query 标签可用）。 */
  onRunQuery: () => void;
  runEnabled: boolean;
  onRefresh: () => void;
  aiCollapsed: boolean;
  onToggleAI: () => void;
}

/** 顶部工具栏：执行当前查询 / 新建查询 / 刷新对象树 / 连接信息 / AI 按钮。 */
export default function Toolbar({
  host,
  onNewQuery,
  onRunQuery,
  runEnabled,
  onRefresh,
  aiCollapsed,
  onToggleAI,
}: ToolbarProps): React.JSX.Element {
  const { t } = useTranslation('hosts');
  const { t: tt } = useTranslation('terminal');
  const params = (host.params ?? {}) as Record<string, unknown>;
  const dbLabel =
    typeof params.filePath === 'string' && params.filePath
      ? `${host.protocol} · ${params.filePath}`
      : `${host.protocol} · ${host.user}@${host.addr}:${host.port}/${String(
          params.database ?? '',
        )}`;

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
      <Button
        size="small"
        type="primary"
        icon={<PlayCircleOutlined />}
        disabled={!runEnabled}
        onClick={onRunQuery}
      >
        {t('db.run')}
      </Button>
      <Button size="small" icon={<PlusOutlined />} onClick={onNewQuery}>
        {t('db.toolbar.newQuery')}
      </Button>
      <Tooltip title={t('db.refreshSchema')}>
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
