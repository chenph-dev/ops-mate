import { Button, Tooltip, theme } from 'antd';
import { ClearOutlined } from '@ant-design/icons';
import { useEffect, useRef, useCallback } from 'react';
import { Terminal as XTerm } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import type { TerminalEntry } from '@/hooks/useTerminal';

interface TerminalProps {
  entries: TerminalEntry[];
  isDark: boolean;
  interactive: boolean;
  hostConnected: boolean;
  onCommand?: (command: string) => void;
}

export default function Terminal({ entries, isDark, interactive, hostConnected, onCommand }: TerminalProps) {
  const { token } = theme.useToken();
  const containerRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<XTerm | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const initialized = useRef(false);
  const promptRef = useRef('');

  // 初始化 xterm
  useEffect(() => {
    if (!containerRef.current || initialized.current) return;
    initialized.current = true;

    const xterm = new XTerm({
      theme: {
        background: isDark ? '#1e1e1e' : '#ffffff',
        foreground: isDark ? '#d4d4d4' : '#333333',
        cursor: isDark ? '#d4d4d4' : '#333333',
        selectionBackground: isDark ? '#264f78' : '#add6ff',
        black: '#000000',
        red: '#cd3131',
        green: '#0dbc79',
        yellow: '#e5e510',
        blue: '#2472c8',
        magenta: '#bc3fbc',
        cyan: '#11a8cd',
        white: '#e5e5e5',
      },
      fontSize: 13,
      fontFamily: '"Cascadia Code", "Fira Code", "Consolas", "Monaco", monospace',
      cursorBlink: interactive,
      scrollback: 10000,
      convertEol: true,
      allowProposedApi: true,
    });

    const fit = new FitAddon();
    xterm.loadAddon(fit);
    xterm.open(containerRef.current);
    fit.fit();
    xtermRef.current = xterm;
    fitRef.current = fit;

    // 交互模式：捕获键盘输入
    if (interactive) {
      let currentLine = '';
      xterm.onData((data) => {
        const code = data.charCodeAt(0);
        if (code === 13) { // Enter
          xterm.writeln('');
          if (currentLine.trim() && onCommand) {
            onCommand(currentLine);
          }
          currentLine = '';
          xterm.write('$ ');
        } else if (code === 127) { // Backspace
          if (currentLine.length > 0) {
            currentLine = currentLine.slice(0, -1);
            xterm.write('\b \b');
          }
        } else if (code >= 32 && code < 127) { // 可打印字符
          currentLine += data;
          xterm.write(data);
        }
      });
    }

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
      black: '#000000',
      red: '#cd3131',
      green: '#0dbc79',
      yellow: '#e5e510',
      blue: '#2472c8',
      magenta: '#bc3fbc',
      cyan: '#11a8cd',
      white: '#e5e5e5',
    };
  }, [isDark]);

  // 写入输出行
  useEffect(() => {
    const xterm = xtermRef.current;
    if (!xterm) return;
    for (const entry of entries) {
      switch (entry.type) {
        case 'input':
          xterm.writeln(`\x1b[36m${entry.text}\x1b[0m`); // 青色
          break;
        case 'output':
          xterm.writeln(entry.text);
          break;
        case 'error':
          xterm.writeln(`\x1b[31m${entry.text}\x1b[0m`); // 红色
          break;
        case 'system':
          xterm.writeln(`\x1b[90m${entry.text}\x1b[0m`); // 灰色
          break;
      }
    }
  }, [entries]);

  // 连接主机后显示提示符
  useEffect(() => {
    const xterm = xtermRef.current;
    if (!xterm || !interactive) return;
    if (hostConnected) {
      xterm.writeln('\x1b[32m[已连接到主机]\x1b[0m');
      xterm.write('$ ');
    } else {
      xterm.writeln('\x1b[90m[请选择主机]\x1b[0m');
    }
  }, [hostConnected, interactive]);

  const handleClear = useCallback(() => {
    xtermRef.current?.clear();
  }, []);

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        position: 'relative',
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
          终端 {hostConnected ? '🟢' : '⚪'}
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
