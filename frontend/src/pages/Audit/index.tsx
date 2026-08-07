import { Card, Col, Empty, Row, Statistic, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
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

const columns: ColumnsType<CallLog> = [
  {
    title: '时间',
    dataIndex: 'ts',
    width: 150,
    render: (ts: number) => fmtTime(ts),
  },
  {
    title: '类型',
    dataIndex: 'component',
    width: 90,
    render: (c: string) =>
      c === 'model' ? <Tag color="blue">模型</Tag> : <Tag>工具</Tag>,
  },
  { title: '节点', dataIndex: 'name', width: 140 },
  { title: '实现', dataIndex: 'provider', width: 100 },
  {
    title: 'Token (in/out/total)',
    key: 'tokens',
    width: 180,
    render: (_, r: CallLog) =>
      r.component === 'model'
        ? `${r.tokensIn} / ${r.tokensOut} / ${r.tokensTotal}`
        : '—',
  },
  {
    title: '耗时',
    dataIndex: 'durationMs',
    width: 100,
    render: (ms: number) => `${ms}ms`,
  },
  {
    title: '状态',
    dataIndex: 'ok',
    width: 80,
    render: (ok: boolean) =>
      ok ? <Tag color="green">成功</Tag> : <Tag color="red">失败</Tag>,
  },
  {
    title: '错误',
    dataIndex: 'error',
    ellipsis: true,
    render: (e: string) =>
      e ? <span style={{ color: '#ff4d4f' }}>{e}</span> : '',
  },
];

export default function AuditPage(): React.JSX.Element {
  const { logs, summary, loading } = useLogs();

  return (
    <div style={{ maxWidth: 900, margin: '0 auto' }}>
      <Title level={4}>审计日志</Title>
      <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
        <Col span={4}>
          <Card size="small">
            <Statistic title="总调用" value={summary?.totalCalls ?? 0} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title="模型调用" value={summary?.modelCalls ?? 0} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title="工具调用" value={summary?.toolCalls ?? 0} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title="Token 入口" value={summary?.tokensIn ?? 0} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title="Token 出口" value={summary?.tokensOut ?? 0} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title="Token 总计" value={summary?.tokensTotal ?? 0} />
          </Card>
        </Col>
      </Row>

      <Card size="small">
        {logs.length === 0 && !loading ? (
          <Empty description="暂无审计记录（AI 调用后才会产生）" />
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
