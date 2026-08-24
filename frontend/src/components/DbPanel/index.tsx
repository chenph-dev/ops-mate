import { useCallback, useEffect, useRef, useState } from 'react';
import { Tabs } from 'antd';
import { useTranslation } from 'react-i18next';
import { ListSchema } from '@wailsjs/go/db/DbHandler';
import type { dbexec, hoststore } from '@wailsjs/go/models';
import { useDbTabs } from '@/hooks/useDbTabs';
import Toolbar from './Toolbar';
import ObjectTree from './ObjectTree';
import QueryTab from './QueryTab';
import DataTab from './DataTab';
import StatusBar from './StatusBar';
import styles from './index.module.css';

type TreeNode = hoststore.TreeNode;

interface DbPanelProps {
  host: TreeNode;
  aiCollapsed: boolean;
  onToggleAI: () => void;
}

/** Navicat 风格 DB 工作台：工具栏 + 对象树（表/视图）+ 多标签主区 + 状态栏。 */
export default function DbPanel({
  host,
  aiCollapsed,
  onToggleAI,
}: DbPanelProps): React.JSX.Element {
  const { t } = useTranslation('hosts');
  const [schema, setSchema] = useState<dbexec.Schema | null>(null);
  const [schemaError, setSchemaError] = useState('');
  const [schemaLoading, setSchemaLoading] = useState(false);
  const { tabs, activeKey, newQuery, openTable, closeTab, activate } =
    useDbTabs();

  // 各查询标签注册的 run：供标题栏「执行」按钮触发当前活动标签。
  const runsRef = useRef<Record<string, (() => void) | null>>({});
  const registerRun = useCallback(
    (key: string) => (run: (() => void) | null) => {
      runsRef.current[key] = run;
    },
    [],
  );
  const runActiveQuery = useCallback((): void => {
    const run = activeKey ? runsRef.current[activeKey] : null;
    run?.();
  }, [activeKey]);
  const activeIsQuery = tabs.some(
    (tab) => tab.key === activeKey && tab.type === 'query',
  );

  const loadSchema = useCallback(async (): Promise<void> => {
    setSchemaLoading(true);
    setSchemaError('');
    try {
      setSchema(await ListSchema(host.id));
    } catch (e) {
      setSchemaError(String(e));
    } finally {
      setSchemaLoading(false);
    }
  }, [host.id]);

  useEffect(() => {
    void loadSchema();
  }, [loadSchema]);

  const params = (host.params ?? {}) as Record<string, unknown>;
  const dbLabel =
    typeof params.filePath === 'string' && params.filePath
      ? `${host.protocol} · ${params.filePath}`
      : `${host.protocol} · ${host.user}@${host.addr}:${host.port}/${String(
          params.database ?? '',
        )}`;

  const items = tabs.map((tab) => ({
    key: tab.key,
    label: tab.title,
    closable: true,
    children:
      tab.type === 'query' ? (
        <QueryTab hostId={host.id} registerRun={registerRun(tab.key)} />
      ) : (
        <DataTab
          hostId={host.id}
          protocol={host.protocol ?? 'ssh'}
          tableName={tab.tableName ?? ''}
        />
      ),
  }));

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
        onNewQuery={newQuery}
        onRunQuery={runActiveQuery}
        runEnabled={activeIsQuery}
        onRefresh={() => void loadSchema()}
        aiCollapsed={aiCollapsed}
        onToggleAI={onToggleAI}
      />
      <div style={{ flex: 1, minHeight: 0, display: 'flex', gap: 8, padding: 8 }}>
        <ObjectTree
          schema={schema}
          loading={schemaLoading}
          error={schemaError}
          onOpenTable={openTable}
        />
        <div
          style={{
            flex: 1,
            minWidth: 0,
            display: 'flex',
            flexDirection: 'column',
            border: '1px solid var(--antd-color-border-secondary)',
            borderRadius: 4,
            overflow: 'hidden',
          }}
        >
          {tabs.length === 0 ? (
            <div
              style={{
                flex: 1,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: 'var(--antd-color-text-secondary)',
                fontSize: 12,
              }}
            >
              {t('db.noResult')}
            </div>
          ) : (
            <Tabs
              type="editable-card"
              size="small"
              className={styles.workbenchTabs}
              items={items}
              activeKey={activeKey ?? undefined}
              onChange={activate}
              onEdit={(targetKey, action) => {
                if (action === 'add') {
                  newQuery();
                } else if (action === 'remove' && typeof targetKey === 'string') {
                  closeTab(targetKey);
                }
              }}
              tabBarStyle={{ margin: 0, padding: '2px 4px 0' }}
              style={{ flex: 1, minHeight: 0 }}
            />
          )}
        </div>
      </div>
      <StatusBar label={dbLabel} />
    </div>
  );
}
