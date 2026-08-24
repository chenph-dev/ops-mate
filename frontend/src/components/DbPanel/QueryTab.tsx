import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Spin } from 'antd';
import CodeMirror from '@uiw/react-codemirror';
import { sql as sqlLang } from '@codemirror/lang-sql';
import { keymap } from '@codemirror/view';
import { Prec } from '@codemirror/state';
import { useTranslation } from 'react-i18next';
import { ExecuteSQL } from '@wailsjs/go/db/DbHandler';
import type { dbexec } from '@wailsjs/go/models';
import ResultGrid from './ResultGrid';

interface QueryTabProps {
  hostId: string;
  /** 把本标签的执行方法注册给父级（供标题栏「执行」触发）；null 表示卸载。 */
  registerRun: (run: (() => void) | null) => void;
}

/**
 * 查询标签：SQL 编辑器 + 结果网格。执行由父级标题栏「执行」按钮触发——
 * 通过 registerRun 把本标签的 run 注册到父级（effect 中注册，避免 render 访问 ref）。
 */
export default function QueryTab({
  hostId,
  registerRun,
}: QueryTabProps): React.JSX.Element {
  const { t } = useTranslation('hosts');
  const [sql, setSql] = useState('');
  const [result, setResult] = useState<dbexec.Result | null>(null);
  const [error, setError] = useState('');
  const [running, setRunning] = useState(false);
  const [durationMs, setDurationMs] = useState<number | undefined>(undefined);

  const run = useCallback(async (): Promise<void> => {
    const cmd = sql.trim();
    if (!cmd || running) return;
    setRunning(true);
    setError('');
    setResult(null);
    const start = Date.now();
    try {
      const res = await ExecuteSQL(hostId, cmd);
      setResult(res ?? null);
      setDurationMs(Date.now() - start);
    } catch (e) {
      setError(String(e));
      setDurationMs(Date.now() - start);
    } finally {
      setRunning(false);
    }
  }, [sql, running, hostId]);

  // 最新 run 引用：供注册闭包读取，避免 stale closure（在 effect/事件中访问）
  const runRef = useRef(run);
  useEffect(() => {
    runRef.current = run;
  }, [run]);

  // 注册/注销 run 到父级（挂载注册、卸载注销）；registerRun 经 ref 取最新
  const registerRunRef = useRef(registerRun);
  useEffect(() => {
    registerRunRef.current = registerRun;
  });
  useEffect(() => {
    registerRunRef.current(() => void runRef.current());
    return () => registerRunRef.current(null);
  }, []);

  // CodeMirror 扩展：keymap 闭包直接捕获当前 run（依赖 run 重建，无 stale、无 ref 访问）
  const extensions = useMemo(
    () => [
      sqlLang(),
      Prec.highest(
        keymap.of([
          {
            key: 'Mod-Enter',
            run: () => {
              void run();
              return true;
            },
          },
        ]),
      ),
    ],
    [run],
  );

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
          flex: 1,
          minWidth: 0,
          border: '1px solid var(--antd-color-border-secondary)',
          borderRadius: 4,
          overflow: 'hidden',
        }}
      >
        <CodeMirror
          value={sql}
          height="140px"
          theme="dark"
          extensions={extensions}
          onChange={(value) => setSql(value)}
          placeholder={t('db.placeholder')}
        />
      </div>
      <div style={{ flex: 1, minHeight: 0 }}>
        {running ? (
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
            exportName="db-query"
          />
        ) : (
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
      </div>
    </div>
  );
}
