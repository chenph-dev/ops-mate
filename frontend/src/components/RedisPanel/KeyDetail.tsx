import { useCallback, useEffect, useState } from 'react';
import {
  App,
  Button,
  Input,
  Modal,
  Popconfirm,
  Spin,
  Tag,
  Tooltip,
} from 'antd';
import { ClockCircleOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { dbexec } from '@wailsjs/go/models';
import ResultGrid from '@/components/DbPanel/ResultGrid';

/** Redis 键名/值转义：双引号包裹，内部转义反斜杠与引号（parseCommand 支持 \"）。 */
function quote(s: string): string {
  return `"${s.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
}

interface KeyDetailProps {
  name: string;
  run: (cmd: string) => Promise<dbexec.Result>;
  onDeleted: () => void;
}

/** 键详情：TYPE/TTL + 按类型取值 + 编辑(string)/删除/设置过期。 */
export default function KeyDetail({
  name,
  run,
  onDeleted,
}: KeyDetailProps): React.JSX.Element {
  const { t } = useTranslation('redis');
  const { message } = App.useApp();
  const [type, setType] = useState('');
  const [ttl, setTtl] = useState<number | null>(null);
  const [result, setResult] = useState<dbexec.Result | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [editOpen, setEditOpen] = useState(false);
  const [editValue, setEditValue] = useState('');
  const [ttlOpen, setTtlOpen] = useState(false);
  const [ttlInput, setTtlInput] = useState('');
  const [deleting, setDeleting] = useState(false);

  const load = useCallback(async (): Promise<void> => {
    setLoading(true);
    setError('');
    try {
      const typeRes = await run(`TYPE ${quote(name)}`);
      const t = String(typeRes.rows?.[0]?.[0] ?? 'string');
      setType(t);
      const ttlRes = await run(`TTL ${quote(name)}`);
      setTtl(Number(String(ttlRes.rows?.[0]?.[0] ?? -1)));
      const cmd =
        t === 'hash'
          ? `HGETALL ${quote(name)}`
          : t === 'list'
            ? `LRANGE ${quote(name)} 0 -1`
            : t === 'set'
              ? `SMEMBERS ${quote(name)}`
              : t === 'zset'
                ? `ZRANGE ${quote(name)} 0 -1 WITHSCORES`
                : `GET ${quote(name)}`;
      const res = await run(cmd);
      setResult(res);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [run, name]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleDelete = async (): Promise<void> => {
    setDeleting(true);
    try {
      await run(`DEL ${quote(name)}`);
      onDeleted();
    } catch (e) {
      message.error(t('detail.deleteFailed', { err: String(e) }));
      setDeleting(false);
    }
  };

  const handleSaveValue = async (): Promise<void> => {
    try {
      await run(`SET ${quote(name)} ${quote(editValue)}`);
      setEditOpen(false);
      await load();
    } catch (e) {
      message.error(t('detail.saveFailed', { err: String(e) }));
    }
  };

  const handleSaveTtl = async (): Promise<void> => {
    const seconds = Number(ttlInput);
    try {
      await run(
        `EXPIRE ${quote(name)} ${Number.isFinite(seconds) ? seconds : 0}`,
      );
      setTtlOpen(false);
      await load();
    } catch (e) {
      message.error(t('detail.saveFailed', { err: String(e) }));
    }
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
        <span style={{ fontWeight: 600, wordBreak: 'break-all' }}>{name}</span>
        {type && (
          <Tag color="blue">
            {t('detail.type')}: {type}
          </Tag>
        )}
        <Tag color={ttl !== null && ttl >= 0 ? 'orange' : 'default'}>
          {t('detail.ttl')}: {ttl !== null && ttl >= 0 ? `${ttl}s` : t('detail.ttlNone')}
        </Tag>
        <div style={{ flex: 1 }} />
        {type === 'string' && (
          <Tooltip title={t('detail.editValue')}>
            <Button
              size="small"
              icon={<EditOutlined />}
              onClick={() => {
                setEditValue(String(result?.rows?.[0]?.[0] ?? ''));
                setEditOpen(true);
              }}
            />
          </Tooltip>
        )}
        <Tooltip title={t('detail.setTtl')}>
          <Button
            size="small"
            icon={<ClockCircleOutlined />}
            onClick={() => {
              setTtlInput(String(ttl ?? -1));
              setTtlOpen(true);
            }}
          />
        </Tooltip>
        <Popconfirm
          title={t('detail.deleteConfirm', { key: name })}
          onConfirm={() => void handleDelete()}
        >
          <Button size="small" danger icon={<DeleteOutlined />} loading={deleting} />
        </Popconfirm>
      </div>
      <div style={{ flex: 1, minHeight: 0 }}>
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
            {error}
          </div>
        ) : result ? (
          <ResultGrid columns={result.columns ?? []} rows={result.rows ?? []} />
        ) : null}
      </div>
      <Modal
        title={t('detail.editStringTitle')}
        open={editOpen}
        onOk={() => void handleSaveValue()}
        onCancel={() => setEditOpen(false)}
        width={480}
      >
        <Input.TextArea
          rows={4}
          value={editValue}
          onChange={(e) => setEditValue(e.target.value)}
        />
      </Modal>
      <Modal
        title={t('detail.ttlTitle')}
        open={ttlOpen}
        onOk={() => void handleSaveTtl()}
        onCancel={() => setTtlOpen(false)}
        width={360}
      >
        <Input
          type="number"
          value={ttlInput}
          onChange={(e) => setTtlInput(e.target.value)}
          placeholder={t('detail.ttlPlaceholder')}
        />
      </Modal>
    </div>
  );
}
