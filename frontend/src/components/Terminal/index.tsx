import { Button, Tooltip, theme } from 'antd';
import { ClearOutlined } from '@ant-design/icons';
import { useEffect, useRef } from 'react';

export interface TerminalLine {
  stream: 'stdout' | 'stderr' | 'exit' | 'info';
  text: string;
}

interface TerminalProps {
  lines: TerminalLine[];
  onClear: () => void;
}

export default function Terminal({ lines, onClear }: TerminalProps) {
  const { token } = theme.useToken();
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (ref.current) {
      ref.current.scrollTop = ref.current.scrollHeight;
    }
  }, [lines]);

  const colorFor = (stream: TerminalLine['stream']) => {
    switch (stream) {
      case 'stderr': return token.colorError;
      case 'exit': return token.colorWarning;
      case 'info': return token.colorTextSecondary;
      default: return token.colorText;
    }
  };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        borderRight: `1px solid ${token.colorBorderSecondary}`,
      }}
    >
      {/* 标题栏 */}
      <div
        style={{
          padding: '6px 10px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: `1px solid ${token.colorBorderSecondary}`,
        }}
      >
        <span style={{ fontSize: 12, fontWeight: 600, color: token.colorTextSecondary }}>
          终端输出
        </span>
        <Tooltip title="清空">
          <Button type="text" size="small" icon={<ClearOutlined />} onClick={onClear} />
        </Tooltip>
      </div>

      {/* 输出区 */}
      <div
        ref={ref}
        style={{
          flex: 1,
          overflow: 'auto',
          padding: '8px 10px',
          background: token.colorBgContainer,
          fontFamily: '"Cascadia Code", "Fira Code", "Consolas", monospace',
          fontSize: 12,
          lineHeight: 1.6,
        }}
      >
        {lines.length === 0 ? (
          <div style={{ color: token.colorTextSecondary, fontSize: 12 }}>
            等待命令执行...
          </div>
        ) : (
          lines.map((line, i) => (
            <div key={i} style={{ color: colorFor(line.stream), whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
              {line.stream === 'exit' ? `[${line.text}]` : line.text}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
