import { useCallback, useEffect, useState } from 'react';
import { Tabs } from 'antd';
import { useTranslation } from 'react-i18next';
import { ExecuteSQL } from '@wailsjs/go/db/DbHandler';
import type { dbexec, hoststore } from '@wailsjs/go/models';
import Toolbar from './Toolbar';
import IndexBrowser from './IndexBrowser';
import QueryTab from './QueryTab';
import CommandTerminal from './CommandTerminal';
import ServerInfo from './ServerInfo';
import StatusBar from '@/components/DbPanel/StatusBar';

type TreeNode = hoststore.TreeNode;

interface EsPanelProps {
  host: TreeNode;
  aiCollapsed: boolean;
  onToggleAI: () => void;
}

/**
 * Elasticsearch 管理面板：索引浏览 + DSL 查询 + 命令终端 + 集群信息。
 * 全部命令经现有 ExecuteSQL(hostID, cmd) 执行（es 走注册表 QueryRunner）。
 */
export default function EsPanel({
  host,
  aiCollapsed,
  onToggleAI,
}: EsPanelProps): React.JSX.Element {
  const { t } = useTranslation('es');
  const [indices, setIndices] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState('');
  const [mainTab, setMainTab] = useState('query');

  const run = useCallback(
    async (cmd: string): Promise<dbexec.Result> => {
      const res = await ExecuteSQL(host.id, cmd);
      return res ?? { columns: [], rows: [] };
    },
    [host.id],
  );

  // 索引列表：_cat/indices?format=json → 数组，取 index 列
  const loadIndices = useCallback(async (): Promise<void> => {
    setLoading(true);
    try {
      const res = await run('_cat/indices?format=json');
      const cols = res.columns ?? [];
      const idxCol = cols.indexOf('index');
      const list: string[] = [];
      for (const row of res.rows ?? []) {
        if (idxCol >= 0 && row[idxCol] !== null && row[idxCol] !== undefined) {
          list.push(String(row[idxCol]));
        }
      }
      setIndices(list);
    } catch {
      setIndices([]);
    } finally {
      setLoading(false);
    }
  }, [run]);

  useEffect(() => {
    void loadIndices();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [host.id]);

  const dbLabel = `${host.protocol} · ${host.user}@${host.addr}:${host.port}`;

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
        onRefresh={() => void loadIndices()}
        aiCollapsed={aiCollapsed}
        onToggleAI={onToggleAI}
      />
      <div style={{ flex: 1, minHeight: 0, display: 'flex', gap: 8, padding: 8 }}>
        <IndexBrowser
          indices={indices}
          loading={loading}
          selectedIndex={selectedIndex}
          onSelectIndex={setSelectedIndex}
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
                  key: 'query',
                  label: t('query.title'),
                  children: (
                    <QueryTab index={selectedIndex} run={run} />
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
