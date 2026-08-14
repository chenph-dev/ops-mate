import { useState } from 'react';
import { Button, Input, message, Spin, Table, Tag, Tooltip, Typography } from 'antd';
import { RobotOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { ExecuteSQL } from '@wailsjs/go/db/DbHandler';
import type { dbexec, hoststore } from '@wailsjs/go/models';

type TreeNode = hoststore.TreeNode;
type DBResult = dbexec.Result;

interface DbPanelProps {
  host: TreeNode;
  aiCollapsed: boolean;
  onToggleAI: () => void;
}

export default function DbPanel({
  host,
  aiCollapsed,
  onToggleAI,
}: DbPanelProps): React.JSX.Element {
  const { t } = useTranslation('hosts');
  const { t: tt } = useTranslation('terminal');
  const [sql, setSql] = useState('');
  const [result, setResult] = useState<DBResult | null>(null);
  const [error, setError] = useState('');
  const [running, setRunning] = useState(false);

  const run = async (): Promise<void> => {
    if (running) return;
    const cmd = sql.trim();
    if (!cmd) return;
    setRunning(true);
    setError('');
    setResult(null);
    try {
      setResult(await ExecuteSQL(host.id, cmd));
    } catch (e) {
      setError(String(e));
      message.error(t('db.failed', { err: String(e) }));
    } finally {
      setRunning(false);
    }
  };

  const columns = (result?.columns ?? []).map((col) => ({
    title: col,
    dataIndex: col,
    key: col,
    ellipsis: true,
  }));
  const dataSource = (result?.rows ?? []).map((row, i) => {
    const rec: Record<string, unknown> = { key: i };
    (result?.columns ?? []).forEach((col, j) => {
      rec[col] = row[j];
    });
    return rec;
  });

  return (
    <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', padding: 8, gap: 8 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <Typography.Text strong>{host.name}</Typography.Text>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {host.driver} / {host.user}@{host.addr}:{host.port}/{host.database}
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
      <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
        <Input.TextArea
          placeholder={t('db.placeholder')}
          value={sql}
          onChange={(e) => setSql(e.target.value)}
          autoSize={{ minRows: 1, maxRows: 4 }}
          onPressEnter={(e) => {
            if (!e.shiftKey) {
              e.preventDefault();
              void run();
            }
          }}
        />
        <Button type="primary" loading={running} onClick={() => void run()}>
          {t('db.run')}
        </Button>
      </div>
      <div style={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
        {running ? (
          <div style={{ textAlign: 'center', paddingTop: 60 }}>
            <Spin />
          </div>
        ) : (
          <>
            {error && (
              <div
                style={{
                  color: 'var(--antd-color-error)',
                  fontSize: 12,
                  padding: 8,
                  border: '1px solid var(--antd-color-error-border)',
                  borderRadius: 4,
                  whiteSpace: 'pre-wrap',
                }}
              >
                {error}
              </div>
            )}
            {result && !error && (
              <>
                {typeof result.rowsAffected === 'number' && result.rowsAffected > 0 && (
                  <Tag color="blue" style={{ marginBottom: 8 }}>
                    {t('db.rowsAffected', { count: result.rowsAffected })}
                  </Tag>
                )}
                {result.columns && result.columns.length > 0 && (
                  <Table
                    size="small"
                    bordered
                    columns={columns}
                    dataSource={dataSource}
                    pagination={{ pageSize: 100, showSizeChanger: false }}
                    scroll={{ x: true }}
                  />
                )}
              </>
            )}
            {!result && !error && (
              <div
                style={{
                  color: 'var(--antd-color-text-secondary)',
                  fontSize: 12,
                  textAlign: 'center',
                  paddingTop: 40,
                }}
              >
                {t('db.noResult')}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
