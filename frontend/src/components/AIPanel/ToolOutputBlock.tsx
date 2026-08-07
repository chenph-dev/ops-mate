import { useMemo, useState } from 'react';

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

  const cmdText = meta?.command ? `$ ${meta.command}` : '命令输出';
  const durText =
    meta?.durationMs != null ? `${(meta.durationMs / 1000).toFixed(1)}s` : '';
  const statusEl =
    meta?.status === 'rejected' ? (
      <span style={{ color: 'var(--antd-color-error)' }}>已拒绝</span>
    ) : meta?.cancelled ? (
      <span style={{ color: 'var(--antd-color-warning)' }}>已取消</span>
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
        title={open ? '收起输出' : '展开输出'}
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
          （{lineCount} 行）
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
