import { useState } from 'react';
import { Button, Input, message, Spin, Tooltip, Typography } from 'antd';
import { RobotOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { ExecuteCommand } from '@wailsjs/go/handler/HostsHandler';
import { OpenRdp } from '@wailsjs/go/handler/RdpHandler';
import type { hoststore } from '@wailsjs/go/models';

type TreeNode = hoststore.TreeNode;

interface WinRmPanelProps {
  host: TreeNode;
  aiCollapsed: boolean;
  onToggleAI: () => void;
}

export default function WinRmPanel({
  host,
  aiCollapsed,
  onToggleAI,
}: WinRmPanelProps): React.JSX.Element {
  const { t } = useTranslation('hosts');
  const { t: tt } = useTranslation('terminal');
  const [command, setCommand] = useState('');
  const [output, setOutput] = useState('');
  const [running, setRunning] = useState(false);
  const [hasRun, setHasRun] = useState(false);

  const run = async (): Promise<void> => {
    if (running) return;
    const cmd = command.trim();
    if (!cmd) return;
    setRunning(true);
    setHasRun(true);
    setOutput('');
    try {
      const result = await ExecuteCommand(host.id, cmd);
      setOutput(result);
    } catch (e) {
      message.error(t('winrm.failed', { err: String(e) }));
      setOutput(String(e));
    } finally {
      setRunning(false);
    }
  };

  const openRdp = async (): Promise<void> => {
    try {
      await OpenRdp(host.id);
    } catch (e) {
      message.error(t('winrm.openRdpFailed', { err: String(e) }));
    }
  };

  return (
    <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', padding: 8, gap: 8 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <Typography.Text strong>{host.name}</Typography.Text>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {host.user}@{host.addr}:{host.port}
        </Typography.Text>
        <Button size="small" onClick={() => void openRdp()}>{t('winrm.openRdp')}</Button>
        <Tooltip title={aiCollapsed ? tt('header.aiClose') : tt('header.aiOpen')}>
          <Button
            type={aiCollapsed ? 'text' : 'primary'}
            size="small"
            icon={<RobotOutlined />}
            onClick={onToggleAI}
          />
        </Tooltip>
      </div>
      <div style={{ display: 'flex', gap: 8 }}>
        <Input
          placeholder={t('winrm.commandPlaceholder')}
          value={command}
          onChange={(e) => setCommand(e.target.value)}
          onPressEnter={() => void run()}
        />
        <Button type="primary" loading={running} onClick={() => void run()}>
          {t('winrm.run')}
        </Button>
      </div>
      <div style={{ flex: 1, minHeight: 0, overflow: 'auto', border: '1px solid var(--antd-color-border-secondary)', borderRadius: 4, padding: 8, background: 'var(--antd-color-bg-container)' }}>
        {running ? <Spin size="small" /> : (
          <pre style={{ margin: 0, fontSize: 12, whiteSpace: 'pre-wrap' }}>
            {hasRun ? output : t('winrm.noOutput')}
          </pre>
        )}
      </div>
    </div>
  );
}
