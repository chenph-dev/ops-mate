import { Card, Col, Empty, Row, Statistic, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { useLogs } from '@/hooks/useLogs';
import type { logsstore } from '@wailsjs/go/models';

type CallLog = logsstore.CallLog;

const { Title } = Typography;

function fmtTime(ts: number): string {
  // ts 为 Unix 秒
  const d = new Date(ts * 1000);
  const pad = (n: number): string => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

export default function AuditPage(): React.JSX.Element {
  const { logs, summary, loading } = useLogs();
  const { t } = useTranslation('audit');

  // 列定义移入组件内以随语言刷新（title/render 均为 i18n 文案）
  const columns: ColumnsType<CallLog> = [
    {
      title: t('col.time'),
      dataIndex: 'ts',
      width: 150,
      render: (ts: number) => fmtTime(ts),
    },
    {
      title: t('col.type'),
      dataIndex: 'component',
      width: 90,
      render: (c: string) =>
        c === 'model' ? (
          <Tag color="blue">{t('col.model')}</Tag>
        ) : (
          <Tag>{t('col.tool')}</Tag>
        ),
    },
    { title: t('col.node'), dataIndex: 'name', width: 140 },
    { title: t('col.provider'), dataIndex: 'provider', width: 100 },
    {
      title: t('col.tokens'),
      key: 'tokens',
      width: 180,
      render: (_, r: CallLog) =>
        r.component === 'model'
          ? `${r.tokensIn} / ${r.tokensOut} / ${r.tokensTotal}`
          : '—',
    },
    {
      title: t('col.duration'),
      dataIndex: 'durationMs',
      width: 100,
      render: (ms: number) => `${ms}ms`,
    },
    {
      title: t('col.status'),
      dataIndex: 'ok',
      width: 80,
      render: (ok: boolean) =>
        ok ? (
          <Tag color="green">{t('col.success')}</Tag>
        ) : (
          <Tag color="red">{t('col.fail')}</Tag>
        ),
    },
    {
      title: t('col.error'),
      dataIndex: 'error',
      ellipsis: true,
      render: (e: string) =>
        e ? <span style={{ color: '#ff4d4f' }}>{e}</span> : '',
    },
  ];

  return (
    <div style={{ maxWidth: 900, margin: '0 auto' }}>
      <Title level={4}>{t('title')}</Title>
      <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
        <Col span={4}>
          <Card size="small">
            <Statistic title={t('stat.totalCalls')} value={summary?.totalCalls ?? 0} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title={t('stat.modelCalls')} value={summary?.modelCalls ?? 0} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title={t('stat.toolCalls')} value={summary?.toolCalls ?? 0} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title={t('stat.tokensIn')} value={summary?.tokensIn ?? 0} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title={t('stat.tokensOut')} value={summary?.tokensOut ?? 0} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title={t('stat.tokensTotal')} value={summary?.tokensTotal ?? 0} />
          </Card>
        </Col>
      </Row>

      <Card size="small">
        {logs.length === 0 && !loading ? (
          <Empty description={t('empty')} />
        ) : (
          <Table<CallLog>
            rowKey="id"
            size="small"
            loading={loading}
            columns={columns}
            dataSource={logs}
            pagination={{ pageSize: 20, showSizeChanger: false }}
          />
        )}
      </Card>
    </div>
  );
}
