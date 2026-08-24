import { useCallback, useEffect, useState } from 'react';
import { Button, Descriptions, Spin, Tooltip } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { dbexec } from '@wailsjs/go/models';

/** 集群健康关键指标（按此顺序过滤 _cluster/health 结果）。 */
const INFO_KEYS = [
  'cluster_name',
  'status',
  'timed_out',
  'number_of_nodes',
  'number_of_data_nodes',
  'active_primary_shards',
  'active_shards',
  'relocating_shards',
  'initializing_shards',
  'unassigned_shards',
  'pending_tasks',
];

interface ServerInfoProps {
  run: (cmd: string) => Promise<dbexec.Result>;
}

/** 集群信息：执行 _cluster/health 并解析关键指标展示。 */
export default function ServerInfo({ run }: ServerInfoProps): React.JSX.Element {
  const { t } = useTranslation('es');
  const [metrics, setMetrics] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const load = useCallback(async (): Promise<void> => {
    setLoading(true);
    setError('');
    try {
      const res = await run('_cluster/health');
      const cols = res.columns ?? [];
      const row = res.rows?.[0] ?? [];
      const map: Record<string, string> = {};
      cols.forEach((c, i) => {
        if (row[i] !== undefined && row[i] !== null) map[c] = String(row[i]);
      });
      setMetrics(map);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [run]);

  useEffect(() => {
    void load();
  }, [load]);

  const items = INFO_KEYS.filter((k) => metrics[k] !== undefined).map((k) => ({
    key: k,
    label: k,
    children: metrics[k],
  }));

  return (
    <div style={{ flex: 1, minHeight: 0, overflow: 'auto', padding: 8 }}>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 4 }}>
        <Tooltip title={t('toolbar.refresh')}>
          <Button
            size="small"
            icon={<ReloadOutlined />}
            onClick={() => void load()}
          />
        </Tooltip>
      </div>
      {loading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: 30 }}>
          <Spin size="small" />
        </div>
      ) : error ? (
        <div
          style={{
            color: 'var(--antd-color-error)',
            fontSize: 12,
            whiteSpace: 'pre-wrap',
          }}
        >
          {t('info.loadFailed', { err: error })}
        </div>
      ) : items.length ? (
        <Descriptions size="small" column={2} bordered items={items} />
      ) : (
        <div style={{ color: 'var(--antd-color-text-secondary)', fontSize: 12 }}>
          {t('info.loadFailed', { err: '' })}
        </div>
      )}
    </div>
  );
}
