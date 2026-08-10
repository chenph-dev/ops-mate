import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

/** 后端落库的 tool 执行元数据（Message.toolResult，见 internal/einoagent/tools/toolMeta）。 */
interface ToolMeta {
  command?: string;
  exitCode?: number;
  durationMs?: number;
  status?: string;
  cancelled?: boolean;
}

/** 命令执行输出块：默认折叠成一行标题（命令 + 退出码 + 耗时），点击展开查看完整输出。 */
export default function ToolOutputBlock({
  content,
  toolResult,
}: {
  content: string;
  toolResult?: string;
}): React.JSX.Element {
  const { t } = useTranslation('ai');
  const [open, setOpen] = useState(false);
  const lineCount = content.split('\n').filter((l) => l.trim() !== '').length;
  const meta = useMemo(() => {
    if (!toolResult) return null;
    try {
      return JSON.parse(toolResult) as ToolMeta;
    } catch {
      return null;
    }
  }, [toolResult]);

  const cmdText = meta?.command ? `$ ${meta.command}` : t('output.title');
  const durText =
    meta?.durationMs != null ? `${(meta.durationMs / 1000).toFixed(1)}s` : '';
  const statusEl =
    meta?.status === 'rejected' ? (
      <span style={{ color: 'var(--antd-color-error)' }}>{t('output.rejected')}</span>
    ) : meta?.cancelled ? (
      <span style={{ color: 'var(--antd-color-warning)' }}>{t('output.cancelled')}</span>
    ) : meta?.exitCode !== undefined ? (
      <span
        style={{
          color:
            meta.exitCode === 0
              ? 'var(--antd-color-text-secondary)'
              : 'var(--antd-color-warning)',
        }}
      >
        exit {meta.exitCode}
      </span>
    ) : null;

  return (
    <div style={{ marginBottom: 6 }}>
      <div
        onClick={() => setOpen(!open)}
        title={open ? t('output.collapse') : t('output.expand')}
        style={{
          fontFamily: '"Cascadia Code", "Fira Code", "Consolas", monospace',
          fontSize: 11,
          background: 'rgba(0,0,0,0.25)',
          border: '1px solid var(--antd-color-border-secondary)',
          borderRadius: 4,
          padding: '2px 8px',
          cursor: 'pointer',
          color: 'var(--antd-color-text-secondary)',
          userSelect: 'none',
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          display: 'flex',
          alignItems: 'center',
          gap: 4,
        }}
      >
        <span style={{ flexShrink: 0 }}>{open ? '▾' : '▸'}</span>
        <span
          style={{
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            minWidth: 0,
          }}
        >
          {cmdText}
        </span>
        {statusEl && (
          <span
            style={{ flexShrink: 0, color: 'var(--antd-color-text-secondary)' }}
          >
            · {statusEl}
          </span>
        )}
        {durText && (
          <span
            style={{ flexShrink: 0, color: 'var(--antd-color-text-secondary)' }}
          >
            · {durText}
          </span>
        )}
        <span
          style={{ flexShrink: 0, color: 'var(--antd-color-text-secondary)' }}
        >
          {t('output.lines', { count: lineCount })}
        </span>
      </div>
      {open && (
        <div
          style={{
            fontFamily: '"Cascadia Code", "Fira Code", "Consolas", monospace',
            fontSize: 11,
            background: 'rgba(0,0,0,0.25)',
            border: '1px solid var(--antd-color-border-secondary)',
            borderTop: 'none',
            borderRadius: '0 0 4px 4px',
            padding: '6px 8px',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
            color: 'var(--antd-color-text)',
            maxHeight: 240,
            overflow: 'auto',
          }}
        >
          {content}
        </div>
      )}
    </div>
  );
}
