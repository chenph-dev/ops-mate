import { useCallback, useEffect, useState } from 'react';
import { Button, Space, Spin, Tag } from 'antd';
import { useTranslation } from 'react-i18next';
import { ExecuteSQL } from '@wailsjs/go/db/DbHandler';
import type { dbexec } from '@wailsjs/go/models';
import ResultGrid from './ResultGrid';

interface DataTabProps {
  hostId: string;
  protocol: string;
  tableName: string;
}

const PAGE_SIZE = 100;

/** 表名安全包裹：mysql 用反引号，其余用双引号。 */
function quoteIdent(protocol: string, name: string): string {
  if (protocol === 'mysql') {
    return '`' + name.replaceAll('`', '``') + '`';
  }
  return '"' + name.replaceAll('"', '""') + '"';
}

/** 表/视图数据浏览标签：SELECT * 分页加载（LIMIT/OFFSET）+ 总数统计。 */
export default function DataTab({
  hostId,
  protocol,
  tableName,
}: DataTabProps): React.JSX.Element {
  const { t } = useTranslation('hosts');
  const [page, setPage] = useState(0);
  const [result, setResult] = useState<dbexec.Result | null>(null);
  const [total, setTotal] = useState<number | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [durationMs, setDurationMs] = useState<number | undefined>(undefined);

  const quoted = quoteIdent(protocol, tableName);

  const load = useCallback(
    async (pageIdx: number): Promise<void> => {
      setLoading(true);
      setError('');
      const start = Date.now();
      try {
        const sql = `SELECT * FROM ${quoted} LIMIT ${PAGE_SIZE} OFFSET ${pageIdx * PAGE_SIZE}`;
        const res = await ExecuteSQL(hostId, sql);
        setResult(res ?? null);
        setDurationMs(Date.now() - start);
        // 总数：仅首次查询（避免每次翻页重复 COUNT）
        if (total === null) {
          try {
            const cnt = await ExecuteSQL(hostId, `SELECT COUNT(*) FROM ${quoted}`);
            const v = cnt?.rows?.[0]?.[0];
            setTotal(typeof v === 'number' ? v : Number(String(v ?? 0)));
          } catch {
            setTotal(null);
          }
        }
      } catch (e) {
        setError(String(e));
        setDurationMs(Date.now() - start);
      } finally {
        setLoading(false);
      }
    },
    [hostId, quoted, total],
  );

  // 表变化时重置并加载第一页
  useEffect(() => {
    setPage(0);
    setTotal(null);
    setResult(null);
    void load(0);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tableName]);

  const totalPages = total === null ? null : Math.ceil(total / PAGE_SIZE);
  const goPage = (p: number): void => {
    setPage(p);
    void load(p);
  };

  return (
    <div
      style={{
        flex: 1,
        minHeight: 0,
        display: 'flex',
        flexDirection: 'column',
        gap: 8,
        padding: 8,
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          fontSize: 12,
          flexShrink: 0,
        }}
      >
        <span style={{ fontWeight: 600 }}>{tableName}</span>
        {total !== null && (
          <Tag color="blue">{t('db.table.totalRows', { count: total })}</Tag>
        )}
        <div style={{ flex: 1 }} />
        <Space size={4}>
          <Button
            size="small"
            disabled={page <= 0 || loading}
            onClick={() => goPage(page - 1)}
          >
            {t('db.pager.prev')}
          </Button>
          <span style={{ fontSize: 12, color: 'var(--antd-color-text-secondary)' }}>
            {t('db.pager.page', { page: page + 1, total: totalPages ?? '?' })}
          </span>
          <Button
            size="small"
            disabled={
              (totalPages !== null && page + 1 >= totalPages) || loading
            }
            onClick={() => goPage(page + 1)}
          >
            {t('db.pager.next')}
          </Button>
        </Space>
      </div>
      <div style={{ flex: 1, minHeight: 0 }}>
        {loading ? (
          <div style={{ display: 'flex', justifyContent: 'center', padding: 40 }}>
            <Spin size="small" />
          </div>
        ) : error ? (
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
        ) : result ? (
          <ResultGrid
            columns={result.columns ?? []}
            rows={result.rows ?? []}
            rowsAffected={result.rowsAffected}
            durationMs={durationMs}
            exportName={tableName}
          />
        ) : null}
      </div>
    </div>
  );
}
