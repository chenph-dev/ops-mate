import { useCallback, useEffect, useState } from 'react';
import { Descriptions, Spin } from 'antd';
import { useTranslation } from 'react-i18next';
import type { dbexec } from '@wailsjs/go/models';

/** INFO 里展示的关键指标（按此顺序过滤）。 */
const INFO_KEYS = [
  'redis_version',
  'redis_mode',
  'os',
  'uptime_in_seconds',
  'connected_clients',
  'used_memory',
  'used_memory_human',
  'used_memory_peak_human',
  'total_commands_processed',
  'keyspace_hits',
  'keyspace_misses',
  'expired_keys',
  'evicted_keys',
  'total_net_input_bytes',
  'total_net_output_bytes',
];

interface ServerInfoProps {
  run: (cmd: string) => Promise<dbexec.Result>;
}

/** 服务器信息：执行 INFO 并解析关键指标展示。 */
export default function ServerInfo({ run }: ServerInfoProps): React.JSX.Element {
  const { t } = useTranslation('redis');
  const [metrics, setMetrics] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const load = useCallback(async (): Promise<void> => {
    setLoading(true);
    setError('');
    try {
      const res = await run('INFO');
      const text = String(res.rows?.[0]?.[0] ?? '');
      const map: Record<string, string> = {};
      for (const line of text.split('\n')) {
        const s = line.trim();
        if (s === '' || s.startsWith('#')) continue;
        const idx = s.indexOf(':');
        if (idx > 0) map[s.slice(0, idx)] = s.slice(idx + 1);
      }
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
