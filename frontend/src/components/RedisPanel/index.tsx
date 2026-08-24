import { useCallback, useEffect, useState } from 'react';
import { Empty, Tabs } from 'antd';
import { useTranslation } from 'react-i18next';
import { ExecuteSQL } from '@wailsjs/go/db/DbHandler';
import type { dbexec, hoststore } from '@wailsjs/go/models';
import Toolbar from './Toolbar';
import KeyBrowser from './KeyBrowser';
import KeyDetail from './KeyDetail';
import CommandTerminal from './CommandTerminal';
import ServerInfo from './ServerInfo';
import StatusBar from '@/components/DbPanel/StatusBar';

type TreeNode = hoststore.TreeNode;

/** Redis 参数转义：双引号包裹，内部转义反斜杠与引号（parseCommand 支持 \"）。 */
function quote(s: string): string {
  return `"${s.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
}

interface RedisPanelProps {
  host: TreeNode;
  aiCollapsed: boolean;
  onToggleAI: () => void;
}

/**
 * Redis 管理面板（参考 RedisInsight）：键空间浏览 + 键详情/编辑 + 命令终端 + 服务器信息。
 * 全部命令经现有 ExecuteSQL(hostID, cmd) 执行（redis 走注册表 QueryRunner）。
 */
export default function RedisPanel({
  host,
  aiCollapsed,
  onToggleAI,
}: RedisPanelProps): React.JSX.Element {
  const { t } = useTranslation('redis');
  const [keys, setKeys] = useState<string[]>([]);
  const [cursor, setCursor] = useState('0');
  const [pattern, setPattern] = useState('');
  const [loading, setLoading] = useState(false);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [mainTab, setMainTab] = useState('detail');

  const run = useCallback(
    async (cmd: string): Promise<dbexec.Result> => {
      const res = await ExecuteSQL(host.id, cmd);
      return res ?? { columns: [], rows: [] };
    },
    [host.id],
  );

  // SCAN 分页：结果 [[1,cursor],[2,[keys...]]]，解析游标与键数组
  const loadKeys = useCallback(
    async (start: string, pat: string): Promise<void> => {
      setLoading(true);
      try {
        const cmd = pat
          ? `SCAN ${start} MATCH ${quote(pat)} COUNT 50`
          : `SCAN ${start} COUNT 50`;
        const res = await run(cmd);
        const rows = res.rows ?? [];
        const next = String(rows[0]?.[1] ?? '0');
        const list = ((rows[1]?.[1] as unknown[]) ?? []).map((k) => String(k));
        setKeys(list);
        setCursor(next);
      } catch {
        setKeys([]);
        setCursor('0');
      } finally {
        setLoading(false);
      }
    },
    [run],
  );

  useEffect(() => {
    void loadKeys('0', '');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [host.id]);

  const refresh = useCallback((): void => {
    void loadKeys('0', pattern);
  }, [loadKeys, pattern]);

  const handleKeyDeleted = useCallback((): void => {
    setSelectedKey(null);
    void loadKeys('0', pattern);
  }, [loadKeys, pattern]);

  const params = (host.params ?? {}) as Record<string, unknown>;
  const dbLabel = `${host.protocol} · ${host.user}@${host.addr}:${host.port}/db${String(
    params['db'] ?? 0,
  )}`;

  return (
    <div
      style={{
        flex: 1,
        minWidth: 0,
        display: 'flex',
        flexDirection: 'column',
        background: 'var(--antd-color-bg-container)',
      }}
    >
      <Toolbar
        host={host}
        onRefresh={refresh}
        aiCollapsed={aiCollapsed}
        onToggleAI={onToggleAI}
      />
      <div style={{ flex: 1, minHeight: 0, display: 'flex', gap: 8, padding: 8 }}>
        <KeyBrowser
          keys={keys}
          loading={loading}
          cursor={cursor}
          pattern={pattern}
          selectedKey={selectedKey}
          onPatternChange={setPattern}
          onSearch={() => void loadKeys('0', pattern)}
          onNext={() => void loadKeys(cursor, pattern)}
          onSelectKey={setSelectedKey}
        />
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div
            style={{
              flex: 1,
              minHeight: 0,
              border: '1px solid var(--antd-color-border-secondary)',
              borderRadius: 4,
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
            }}
          >
            <Tabs
              size="small"
              activeKey={mainTab}
              onChange={setMainTab}
              items={[
                {
                  key: 'detail',
                  label: t('detail.title'),
                  children: selectedKey ? (
                    <KeyDetail
                      name={selectedKey}
                      run={run}
                      onDeleted={handleKeyDeleted}
                    />
                  ) : (
                    <Empty
                      image={Empty.PRESENTED_IMAGE_SIMPLE}
                      description={
                        <span style={{ fontSize: 12 }}>{t('detail.selectHint')}</span>
                      }
                      style={{ marginTop: 40 }}
                    />
                  ),
                },
                {
                  key: 'info',
                  label: t('info.title'),
                  children: <ServerInfo run={run} />,
                },
              ]}
              tabBarStyle={{ margin: 0, padding: '2px 4px 0' }}
            />
          </div>
          <CommandTerminal run={run} />
        </div>
      </div>
      <StatusBar label={dbLabel} />
    </div>
  );
}
