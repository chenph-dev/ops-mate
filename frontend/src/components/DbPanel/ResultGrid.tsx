import { Button, Table, Tag, Tooltip } from 'antd';
import { DownloadOutlined } from '@ant-design/icons';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';

interface ResultGridProps {
  columns: string[];
  rows: unknown[][];
  rowsAffected?: number;
  durationMs?: number;
  /** 导出文件名（不含扩展名）；省略则不显示导出按钮。 */
  exportName?: string;
}

/** CSV 生成：字段值转义引号/逗号/换行，UTF-8 BOM 便于 Excel 识别。 */
function toCsv(columns: string[], rows: unknown[][]): string {
  const esc = (v: unknown): string => {
    if (v === null || v === undefined) return '';
    const s = String(v);
    return /[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
  };
  const lines = [columns.map(esc).join(',')];
  for (const row of rows) lines.push(row.map(esc).join(','));
  return lines.join('\r\n');
}

function downloadCsv(fileName: string, csv: string): void {
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = fileName.endsWith('.csv') ? fileName : `${fileName}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

/** 通用结果网格：行数/耗时/CSV 导出 + antd Table（前端分页，pageSize 可切）。 */
export default function ResultGrid({
  columns,
  rows,
  rowsAffected,
  durationMs,
  exportName,
}: ResultGridProps): React.JSX.Element {
  const { t } = useTranslation('hosts');

  const dataSource = useMemo(
    () =>
      rows.map((row, i) => {
        const rec: Record<string, unknown> = { key: i };
        columns.forEach((col, j) => {
          rec[col] = row[j];
        });
        return rec;
      }),
    [rows, columns],
  );

  const cols = useMemo(
    () =>
      columns.map((col) => ({
        title: col,
        dataIndex: col,
        key: col,
        ellipsis: true,
      })),
    [columns],
  );

  const hasResult = columns.length > 0;

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 6,
        flex: 1,
        minHeight: 0,
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          fontSize: 12,
          color: 'var(--antd-color-text-secondary)',
          flexShrink: 0,
        }}
      >
        {rowsAffected !== undefined && rowsAffected > 0 && (
          <Tag color="blue">{t('db.result.rowsAffected', { count: rowsAffected })}</Tag>
        )}
        {hasResult && (
          <>
            <span>{t('db.result.rows', { count: rows.length })}</span>
            {durationMs !== undefined && (
              <span>{t('db.result.duration', { ms: durationMs })}</span>
            )}
            <span style={{ flex: 1 }} />
            {exportName && (
              <Tooltip title={t('db.result.exportCsv')}>
                <Button
                  size="small"
                  type="text"
                  icon={<DownloadOutlined />}
                  onClick={() => downloadCsv(exportName, toCsv(columns, rows))}
                />
              </Tooltip>
            )}
          </>
        )}
      </div>
      <div style={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
        {hasResult ? (
          <Table
            size="small"
            bordered
            columns={cols}
            dataSource={dataSource}
            pagination={{
              pageSize: 100,
              showSizeChanger: true,
              pageSizeOptions: [50, 100, 200, 500],
            }}
            scroll={{ x: true }}
          />
        ) : (
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              height: '100%',
              color: 'var(--antd-color-text-secondary)',
              fontSize: 12,
            }}
          >
            {t('db.noResult')}
          </div>
        )}
      </div>
    </div>
  );
}
