import { useCallback, useRef, useState } from 'react';
import { Button, Input, Spin } from 'antd';
import type { InputRef } from 'antd';
import { PlayCircleOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { dbexec } from '@wailsjs/go/models';
import ResultGrid from '@/components/DbPanel/ResultGrid';

interface CommandTerminalProps {
  run: (cmd: string) => Promise<dbexec.Result>;
}

/** 命令终端：输入 ES REST 命令（路径，可选 body），Enter 执行，↑↓ 历史。 */
export default function CommandTerminal({
  run,
}: CommandTerminalProps): React.JSX.Element {
  const { t } = useTranslation('es');
  const [cmd, setCmd] = useState('');
  const [result, setResult] = useState<dbexec.Result | null>(null);
  const [error, setError] = useState('');
  const [running, setRunning] = useState(false);
  const [history, setHistory] = useState<string[]>([]);
  const [histIdx, setHistIdx] = useState(-1);
  const inputRef = useRef<InputRef>(null);

  const execute = useCallback(
    async (text: string): Promise<void> => {
      const trimmed = text.trim();
      if (!trimmed || running) return;
      setRunning(true);
      setError('');
      setResult(null);
      try {
        const res = await run(trimmed);
        setResult(res);
        setHistory((prev) => [trimmed, ...prev.slice(0, 49)]);
        setCmd('');
        setHistIdx(-1);
      } catch (e) {
        setError(String(e));
      } finally {
        setRunning(false);
        inputRef.current?.focus();
      }
    },
    [run, running],
  );

  const onKeyDown = (e: React.KeyboardEvent): void => {
    if (e.key === 'Enter') {
      e.preventDefault();
      void execute(cmd);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      const next = Math.min(histIdx + 1, history.length - 1);
      if (next >= 0) {
        setHistIdx(next);
        setCmd(history[next]);
      }
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      const next = histIdx - 1;
      if (next < 0) {
        setHistIdx(-1);
        setCmd('');
      } else {
        setHistIdx(next);
        setCmd(history[next]);
      }
    }
  };

  return (
    <div
      style={{
        height: 200,
        flexShrink: 0,
        border: '1px solid var(--antd-color-border-secondary)',
        borderRadius: 4,
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
      }}
    >
      <div
        style={{
          padding: '4px 8px',
          fontSize: 12,
          borderBottom: '1px solid var(--antd-color-border-secondary)',
          color: 'var(--antd-color-text-secondary)',
          flexShrink: 0,
        }}
      >
        {t('terminal.title')}
      </div>
      <div style={{ flex: 1, minHeight: 0, overflow: 'auto', padding: 8 }}>
        {running ? (
          <div style={{ display: 'flex', justifyContent: 'center', padding: 20 }}>
            <Spin size="small" />
          </div>
        ) : error ? (
          <div style={{ color: 'var(--antd-color-error)', fontSize: 12 }}>
            {error}
          </div>
        ) : result ? (
          <ResultGrid
            columns={result.columns ?? []}
            rows={result.rows ?? []}
            exportName="es-command"
          />
        ) : (
          <div style={{ color: 'var(--antd-color-text-secondary)', fontSize: 12 }}>
            {t('terminal.empty')}
          </div>
        )}
      </div>
      <div
        style={{
          padding: 6,
          borderTop: '1px solid var(--antd-color-border-secondary)',
        }}
      >
        <Input
          ref={inputRef}
          value={cmd}
          onChange={(e) => setCmd(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder={t('terminal.placeholder')}
          suffix={
            <Button
              type="primary"
              size="small"
              icon={<PlayCircleOutlined />}
              loading={running}
              disabled={!cmd.trim() || running}
              onClick={() => void execute(cmd)}
              style={{ marginRight: -8 }}
            />
          }
        />
      </div>
    </div>
  );
}
