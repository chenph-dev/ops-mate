import { useCallback, useState } from 'react';
import { Button, Input, Spin } from 'antd';
import { useTranslation } from 'react-i18next';
import type { dbexec } from '@wailsjs/go/models';
import ResultGrid from '@/components/DbPanel/ResultGrid';

interface QueryTabProps {
  index: string;
  run: (cmd: string) => Promise<dbexec.Result>;
}

const DEFAULT_DSL = `{
  "query": {
    "match_all": {}
  }
}`;

/** 查询标签：选择索引 + 输入 Query DSL → 执行并展示命中文档（ResultGrid）。 */
export default function QueryTab({
  index,
  run,
}: QueryTabProps): React.JSX.Element {
  const { t } = useTranslation('es');
  const [dsl, setDsl] = useState(DEFAULT_DSL);
  const [result, setResult] = useState<dbexec.Result | null>(null);
  const [error, setError] = useState('');
  const [running, setRunning] = useState(false);

  const execute = useCallback(async (): Promise<void> => {
    if (!index.trim() || running) return;
    setRunning(true);
    setError('');
    setResult(null);
    try {
      const cmd = `${index.trim()}/_search\n${dsl}`;
      const res = await run(cmd);
      setResult(res);
    } catch (e) {
      setError(String(e));
    } finally {
      setRunning(false);
    }
  }, [index, dsl, running, run]);

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
      <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
        <Input
          size="small"
          value={index}
          disabled
          placeholder={t('query.indexPlaceholder')}
          style={{ width: 240 }}
        />
        <Button
          size="small"
          type="primary"
          loading={running}
          disabled={!index.trim()}
          onClick={() => void execute()}
        >
          {t('query.run')}
        </Button>
      </div>
      <div
        style={{
          border: '1px solid var(--antd-color-border-secondary)',
          borderRadius: 4,
          overflow: 'hidden',
          flexShrink: 0,
        }}
      >
        <Input.TextArea
          rows={5}
          value={dsl}
          onChange={(e) => setDsl(e.target.value)}
          style={{ fontFamily: 'monospace', fontSize: 12 }}
        />
      </div>
      <div style={{ flex: 1, minHeight: 0 }}>
        {running ? (
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
        ) : (
          <div
            style={{
              color: 'var(--antd-color-text-secondary)',
              fontSize: 12,
              paddingTop: 20,
            }}
          >
            {t('query.empty')}
          </div>
        )}
      </div>
    </div>
  );
}
