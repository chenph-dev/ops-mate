import { useEffect, useMemo, useRef, useState } from 'react';
import { Button, message, Spin, Table, Tag, Tooltip, Tree, Typography } from 'antd';
import type { TreeDataNode } from 'antd';
import { ReloadOutlined, RobotOutlined } from '@ant-design/icons';
import CodeMirror from '@uiw/react-codemirror';
import { sql as sqlLang } from '@codemirror/lang-sql';
import { keymap } from '@codemirror/view';
import { Prec } from '@codemirror/state';
import { useTranslation } from 'react-i18next';
import { ExecuteSQL, ListSchema } from '@wailsjs/go/db/DbHandler';
import type { dbexec, hoststore } from '@wailsjs/go/models';

type TreeNode = hoststore.TreeNode;
type DBResult = dbexec.Result;
type DBSchema = dbexec.Schema;

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

  // 左侧 schema 树
  const [schema, setSchema] = useState<DBSchema | null>(null);
  const [schemaError, setSchemaError] = useState('');
  const [schemaLoading, setSchemaLoading] = useState(false);

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

  const loadSchema = async (): Promise<void> => {
    setSchemaLoading(true);
    setSchemaError('');
    try {
      setSchema(await ListSchema(host.id));
    } catch (e) {
      setSchemaError(String(e));
    } finally {
      setSchemaLoading(false);
    }
  };

  useEffect(() => {
    void loadSchema();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [host.id]);

  // CodeMirror：Ctrl/Cmd+Enter 执行（runRef 规避闭包过期）
  const runRef = useRef(run);
  useEffect(() => {
    runRef.current = run;
  });
  const extensions = useMemo(
    () => [
      sqlLang(),
      Prec.highest(
        keymap.of([
          {
            key: 'Mod-Enter',
            run: () => {
              void runRef.current();
              return true;
            },
          },
        ]),
      ),
    ],
    [],
  );

  // 表 → 列 的树节点
  const treeData: TreeDataNode[] = useMemo(
    () =>
      (schema?.tables ?? []).map((tbl) => ({
        key: `table:${tbl.name}`,
        title: tbl.name,
        children: tbl.columns.map((col) => ({
          key: `col:${tbl.name}:${col.name}`,
          title: `${col.name} : ${col.dataType}`,
          isLeaf: true,
        })),
      })),
    [schema],
  );

  // 点击表节点 → 生成 SELECT 查询填入编辑器
  const onTreeSelect = (_: React.Key[], info: { node: TreeDataNode }): void => {
    const key = String(info.node.key ?? '');
    if (key.startsWith('table:')) {
      setSql(`SELECT * FROM ${key.slice('table:'.length)};`);
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
      {/* 标题行 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <Typography.Text strong>{host.name}</Typography.Text>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {host.driver} / {host.user}@{host.addr}:{host.port}/{host.database}
        </Typography.Text>
        <Tooltip title={t('db.refreshSchema')}>
          <Button size="small" icon={<ReloadOutlined />} onClick={() => void loadSchema()} />
        </Tooltip>
        <Tooltip title={aiCollapsed ? tt('header.aiClose') : tt('header.aiOpen')}>
          <Button
            type={aiCollapsed ? 'text' : 'primary'}
            size="small"
            icon={<RobotOutlined />}
            onClick={onToggleAI}
          />
        </Tooltip>
      </div>

      <div style={{ flex: 1, minHeight: 0, display: 'flex', gap: 8 }}>
        {/* 左：schema 树 */}
        <div
          style={{
            width: 200,
            flexShrink: 0,
            border: '1px solid var(--antd-color-border-secondary)',
            borderRadius: 4,
            overflow: 'auto',
            padding: 4,
          }}
        >
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {t('db.schema')}
          </Typography.Text>
          {schemaLoading ? (
            <div style={{ textAlign: 'center', paddingTop: 20 }}>
              <Spin size="small" />
            </div>
          ) : schemaError ? (
            <div
              style={{
                color: 'var(--antd-color-error)',
                fontSize: 12,
                whiteSpace: 'pre-wrap',
                padding: 4,
              }}
            >
              {schemaError}
            </div>
          ) : (
            <Tree
              treeData={treeData}
              onSelect={onTreeSelect}
              showLine
              defaultExpandAll={false}
              virtual
              height={400}
            />
          )}
          {!schemaLoading && !schemaError && schema && schema.tables.length > 0 && (
            <Typography.Text type="secondary" style={{ fontSize: 11, padding: '0 4px' }}>
              {t('db.clickTableHint')}
            </Typography.Text>
          )}
        </div>

        {/* 右：编辑器 + 结果 */}
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
            <div style={{ flex: 1, minWidth: 0, border: '1px solid var(--antd-color-border-secondary)', borderRadius: 4, overflow: 'hidden' }}>
              <CodeMirror
                value={sql}
                height="140px"
                theme="dark"
                extensions={extensions}
                onChange={(value) => setSql(value)}
                placeholder={t('db.placeholder')}
              />
            </div>
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
      </div>
    </div>
  );
}
