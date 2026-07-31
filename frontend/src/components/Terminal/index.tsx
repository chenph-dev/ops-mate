import { Button, Tooltip, theme } from 'antd';
import { ClearOutlined } from '@ant-design/icons';
import { useEffect, useRef, useCallback } from 'react';
import { Terminal as XTerm } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

export interface TerminalLine {
  stream: 'stdout' | 'stderr' | 'exit' | 'info';
  text: string;
}

interface TerminalProps {
  lines: TerminalLine[];
  isDark: boolean;
}

export default function Terminal({ lines, isDark }: TerminalProps) {
  const { token } = theme.useToken();
  const containerRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<XTerm | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const initialized = useRef(false);

  // 初始化 xterm（仅一次）
  useEffect(() => {
    if (!containerRef.current || initialized.current) return;
    initialized.current = true;

    const xterm = new XTerm({
      theme: {
        background: isDark ? '#1e1e1e' : '#ffffff',
        foreground: isDark ? '#d4d4d4' : '#333333',
        cursor: isDark ? '#d4d4d4' : '#333333',
        selectionBackground: isDark ? '#264f78' : '#add6ff',
      },
      fontSize: 12,
      fontFamily: '"Cascadia Code", "Fira Code", "Consolas", "Monaco", monospace',
      cursorBlink: false,
      scrollback: 10000,
      convertEol: true,
    });

    const fit = new FitAddon();
    xterm.loadAddon(fit);
    xterm.open(containerRef.current);
    fit.fit();
    xtermRef.current = xterm;
    fitRef.current = fit;

    // 自适应容器尺寸变化
    const observer = new ResizeObserver(() => fit.fit());
    if (containerRef.current) {
      observer.observe(containerRef.current);
    }

    return () => {
      observer.disconnect();
      xterm.dispose();
      xtermRef.current = null;
      initialized.current = false;
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // 主题切换时更新 xterm 配色
  useEffect(() => {
    const xterm = xtermRef.current;
    if (!xterm) return;
    xterm.options.theme = {
      background: isDark ? '#1e1e1e' : '#ffffff',
      foreground: isDark ? '#d4d4d4' : '#333333',
      cursor: isDark ? '#d4d4d4' : '#333333',
      selectionBackground: isDark ? '#264f78' : '#add6ff',
    };
  }, [isDark]);

  // 写入输出行
  useEffect(() => {
    const xterm = xtermRef.current;
    if (!xterm) return;
    for (const line of lines) {
      if (line.stream === 'stderr') {
        xterm.writeln(`\x1b[31m${line.text}\x1b[0m`);
      } else if (line.stream === 'exit') {
        xterm.writeln(`\x1b[33m[${line.text}]\x1b[0m`);
      } else if (line.stream === 'info') {
        xterm.writeln(`\x1b[90m${line.text}\x1b[0m`);
      } else {
        xterm.writeln(line.text);
      }
    }
  }, [lines]);

  const handleClear = useCallback(() => {
    xtermRef.current?.clear();
  }, []);

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
          flexShrink: 0,
        }}
      >
        <span style={{ fontSize: 12, fontWeight: 600, color: token.colorTextSecondary }}>
          终端输出
        </span>
        <Tooltip title="清空">
          <Button type="text" size="small" icon={<ClearOutlined />} onClick={handleClear} />
        </Tooltip>
      </div>

      {/* xterm 容器 */}
      <div
        ref={containerRef}
        style={{
          flex: 1,
          overflow: 'hidden',
          backgroundColor: isDark ? '#1e1e1e' : '#ffffff',
        }}
      />
    </div>
  );
}
