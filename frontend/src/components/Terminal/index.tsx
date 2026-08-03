import { Button, Tooltip, theme, Spin, Input, type InputRef } from 'antd';
import {
  ClearOutlined,
  CopyOutlined,
  DisconnectOutlined,
  SearchOutlined,
  UpOutlined,
  DownOutlined,
  CloseOutlined,
  ZoomInOutlined,
  ZoomOutOutlined,
} from '@ant-design/icons';
import { useCallback, useEffect, useRef, useState } from 'react';
import { Terminal as XTerm } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { SearchAddon } from '@xterm/addon-search';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { WebglAddon } from '@xterm/addon-webgl';
import '@xterm/xterm/css/xterm.css';
import { ClipboardGetText, ClipboardSetText } from '@wailsjs/runtime/runtime';
import { terminalTheme } from '@/theme';

interface TerminalProps {
  isDark: boolean;
  connected: boolean;
  connecting: boolean;
  reconnecting?: boolean;
  reconnectCount?: number;
  hostName: string;
  hostAddr: string;
  onData: (data: string) => void;
  onResize: (cols: number, rows: number) => void;
  setOutputHandler: (cb: (data: Uint8Array) => void) => void;
  onDisconnect: () => void;
}

const DEFAULT_FONT_SIZE = 13;
const MIN_FONT_SIZE = 9;
const MAX_FONT_SIZE = 24;
// 与 useTerminal.ts 保持一致，用于状态栏显示重连计数上限
const MAX_RECONNECT_RETRIES = 5;

interface ContextMenuItem {
  key: string;
  label: string;
  onClick: () => void;
}

interface ContextMenuProps {
  x: number;
  y: number;
  items: ContextMenuItem[];
  onClose: () => void;
  colorBgElevated: string;
  borderRadiusLG: number;
  boxShadowSecondary: string;
  colorText: string;
}

function TerminalContextMenu({
  x,
  y,
  items,
  onClose,
  colorBgElevated,
  borderRadiusLG,
  boxShadowSecondary,
  colorText,
}: ContextMenuProps): React.JSX.Element {
  return (
    <div
      style={{
        position: 'fixed',
        left: x,
        top: y,
        zIndex: 9999,
        background: colorBgElevated,
        borderRadius: borderRadiusLG,
        boxShadow: boxShadowSecondary,
        padding: '4px 0',
        minWidth: 140,
      }}
      onClick={onClose}
    >
      {items.map((item) => (
        <div
          key={item.key}
          style={{
            padding: '6px 16px',
            cursor: 'pointer',
            fontSize: 13,
            color: colorText,
          }}
          onClick={item.onClick}
        >
          {item.label}
        </div>
      ))}
    </div>
  );
}

export default function Terminal({
  isDark,
  connected,
  connecting,
  reconnecting = false,
  reconnectCount = 0,
  hostName,
  hostAddr,
  onData,
  onResize,
  setOutputHandler,
  onDisconnect,
}: TerminalProps): React.JSX.Element {
  const { token } = theme.useToken();
  const containerRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<XTerm | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const searchRef = useRef<SearchAddon | null>(null);
  const searchInputRef = useRef<InputRef>(null);
  const [dims, setDims] = useState({ cols: 80, rows: 24 });
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchText, setSearchText] = useState('');
  const [fontSize, setFontSize] = useState(DEFAULT_FONT_SIZE);
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number } | null>(null);

  // 用 ref 保存最新回调，避免 effect 只初始化一次导致闭包过期
  const onDataRef = useRef(onData);
  const onResizeRef = useRef(onResize);
  useEffect(() => {
    onDataRef.current = onData;
    onResizeRef.current = onResize;
  });

  const handleZoomIn = useCallback((): void => {
    setFontSize((prev) => Math.min(prev + 1, MAX_FONT_SIZE));
  }, []);

  const handleZoomOut = useCallback((): void => {
    setFontSize((prev) => Math.max(prev - 1, MIN_FONT_SIZE));
  }, []);

  const handleZoomReset = useCallback((): void => {
    setFontSize(DEFAULT_FONT_SIZE);
  }, []);

  // 初始化 xterm
  useEffect(() => {
    if (!containerRef.current) return;
    const xterm = new XTerm({
      theme: terminalTheme(isDark),
      fontSize,
      fontFamily: '"Cascadia Code", "Fira Code", "Consolas", "Monaco", monospace',
      cursorBlink: true,
      scrollback: 10000,
      convertEol: true,
      allowProposedApi: true,
    });
    const fit = new FitAddon();
    xterm.loadAddon(fit);
    // 可点击链接（URL/文件路径）
    xterm.loadAddon(new WebLinksAddon());
    // WebGL 加速渲染（失败则回退到默认 canvas 渲染器）
    try {
      xterm.loadAddon(new WebglAddon());
    } catch {
      // 忽略：部分环境不支持 WebGL
    }
    const search = new SearchAddon();
    xterm.loadAddon(search);
    searchRef.current = search;
    xterm.open(containerRef.current);
    fit.fit();
    xtermRef.current = xterm;
    fitRef.current = fit;

    // 键盘输入 → 后端
    xterm.onData((d) => onDataRef.current(d));
    // 后端输出 → xterm
    setOutputHandler((data) => xterm.write(data));

    // 容器尺寸变化 → 同步 PTY 尺寸
    let timer: ReturnType<typeof setTimeout> | undefined;
    const observer = new ResizeObserver(() => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        fit.fit();
        const d = fit.proposeDimensions();
        if (d) {
          setDims({ cols: d.cols, rows: d.rows });
          onResizeRef.current(d.cols, d.rows);
        }
      }, 100);
    });
    observer.observe(containerRef.current);

    // Ctrl+F 打开搜索、Esc 关闭；Ctrl++/-/0 缩放字体；用 attachCustomKeyEventHandler 拦截，避免发给远程 shell。
    xterm.attachCustomKeyEventHandler((e) => {
      const isModifier = e.ctrlKey || e.metaKey;
      if (isModifier && e.key.toLowerCase() === 'f') {
        e.preventDefault();
        setSearchOpen(true);
        setTimeout(() => searchInputRef.current?.focus(), 0);
        return false; // 阻止 xterm 处理
      }
      if (isModifier && (e.key === '+' || e.key === '=')) {
        e.preventDefault();
        handleZoomIn();
        return false;
      }
      if (isModifier && e.key === '-') {
        e.preventDefault();
        handleZoomOut();
        return false;
      }
      if (isModifier && e.key === '0') {
        e.preventDefault();
        handleZoomReset();
        return false;
      }
      if (e.key === 'Escape') {
        setSearchOpen(false);
        setContextMenu(null);
        return false;
      }
      return true;
    });

    return () => {
      clearTimeout(timer);
      observer.disconnect();
      xterm.dispose();
      xtermRef.current = null;
      fitRef.current = null;
      searchRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 主题切换时更新 xterm 配色
  useEffect(() => {
    if (xtermRef.current) {
      xtermRef.current.options.theme = terminalTheme(isDark);
    }
  }, [isDark]);

  // 字体大小变化时同步到 xterm 并重新 fit
  useEffect(() => {
    if (xtermRef.current) {
      xtermRef.current.options.fontSize = fontSize;
      fitRef.current?.fit();
      const d = fitRef.current?.proposeDimensions();
      if (d) {
        setDims({ cols: d.cols, rows: d.rows });
        onResizeRef.current(d.cols, d.rows);
      }
    }
  }, [fontSize]);

  // 连接建立后，用实际容器尺寸同步一次 PTY
  useEffect(() => {
    if (connected && fitRef.current) {
      fitRef.current.fit();
      const d = fitRef.current.proposeDimensions();
      if (d) {
        setDims({ cols: d.cols, rows: d.rows });
        onResizeRef.current(d.cols, d.rows);
      }
    }
  }, [connected]);

  // 断开时（从已连接 → 未连接）在终端末尾写一行提示（非重连场景）
  const prevConnectedRef = useRef(false);
  useEffect(() => {
    if (prevConnectedRef.current && !connected && !reconnecting) {
      xtermRef.current?.write('\x1b[90m\r\n[会话已断开] 双击主机重新连接\x1b[0m\r\n');
    }
    prevConnectedRef.current = connected;
  }, [connected, reconnecting]);

  // 右键菜单事件监听
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const handleContextMenu = (e: MouseEvent): void => {
      e.preventDefault();
      setContextMenu({ x: e.clientX, y: e.clientY });
    };
    container.addEventListener('contextmenu', handleContextMenu);
    return () => {
      container.removeEventListener('contextmenu', handleContextMenu);
    };
  }, []);

  // Ctrl + 滚轮缩放
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const handleWheel = (e: WheelEvent): void => {
      if (!e.ctrlKey && !e.metaKey) return;
      e.preventDefault();
      if (e.deltaY < 0) {
        handleZoomIn();
      } else {
        handleZoomOut();
      }
    };
    container.addEventListener('wheel', handleWheel, { passive: false });
    return () => {
      container.removeEventListener('wheel', handleWheel);
    };
  }, [handleZoomIn, handleZoomOut]);

  const handleClear = useCallback((): void => {
    xtermRef.current?.clear();
  }, []);

  const handleCopy = useCallback(async (): Promise<void> => {
    const sel = xtermRef.current?.getSelection();
    if (sel) await ClipboardSetText(sel);
    setContextMenu(null);
  }, []);

  const handlePaste = useCallback(async (): Promise<void> => {
    try {
      const text = await ClipboardGetText();
      if (text && xtermRef.current) {
        xtermRef.current.paste(text);
      }
    } catch {
      // 忽略剪贴板读取错误
    }
    setContextMenu(null);
  }, []);

  const handleSelectAll = useCallback((): void => {
    xtermRef.current?.selectAll();
    setContextMenu(null);
  }, []);

  const handleSearchClose = useCallback((): void => {
    setSearchOpen(false);
    setSearchText('');
    searchRef.current?.clearDecorations();
  }, []);

  const contextMenuItems: ContextMenuItem[] = [
    { key: 'copy', label: '复制', onClick: handleCopy },
    { key: 'paste', label: '粘贴', onClick: handlePaste },
    { key: 'select-all', label: '全选', onClick: handleSelectAll },
    { key: 'clear', label: '清空', onClick: handleClear },
  ];

  const statusDot = connecting ? '⏳' : connected ? '🟢' : '⚪';
  const statusText = connecting
    ? '连接中'
    : reconnecting
      ? `重连中 (${reconnectCount}/${MAX_RECONNECT_RETRIES})`
      : connected
        ? '已连接'
        : '未连接';

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        position: 'relative',
        borderRadius: 8,
        border: `1px solid ${token.colorBorderSecondary}`,
        overflow: 'hidden',
        background: terminalTheme(isDark).background,
      }}
    >
      {/* 标题栏 */}
      <div
        style={{
          padding: '4px 10px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: `1px solid ${token.colorBorderSecondary}`,
          flexShrink: 0,
          background: token.colorBgElevated,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
          <span style={{ fontSize: 11, color: token.colorTextSecondary, whiteSpace: 'nowrap' }}>
            终端
          </span>
          <span style={{ fontSize: 12, fontWeight: 600, whiteSpace: 'nowrap' }}>
            {hostName || '未选择主机'}
          </span>
          <span
            style={{
              fontSize: 11,
              color: connected ? token.colorSuccess : token.colorTextSecondary,
              whiteSpace: 'nowrap',
            }}
          >
            {statusDot} {statusText}
          </span>
          {hostAddr && (
            <Tooltip title={hostAddr}>
              <span
                style={{
                  fontSize: 11,
                  color: token.colorTextTertiary,
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                }}
              >
                {hostAddr}
              </span>
            </Tooltip>
          )}
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <Tooltip title="放大 (Ctrl++)">
            <Button
              type="text"
              size="small"
              icon={<ZoomInOutlined />}
              onClick={handleZoomIn}
              disabled={fontSize >= MAX_FONT_SIZE}
            />
          </Tooltip>
          <Tooltip title="缩小 (Ctrl+-)">
            <Button
              type="text"
              size="small"
              icon={<ZoomOutOutlined />}
              onClick={handleZoomOut}
              disabled={fontSize <= MIN_FONT_SIZE}
            />
          </Tooltip>
          <Tooltip title="搜索 (Ctrl+F)">
            <Button
              type="text"
              size="small"
              icon={<SearchOutlined />}
              onClick={(): void => {
                setSearchOpen(true);
                setTimeout(() => searchInputRef.current?.focus(), 0);
              }}
            />
          </Tooltip>
          <Tooltip title="清空">
            <Button type="text" size="small" icon={<ClearOutlined />} onClick={handleClear} />
          </Tooltip>
          <Tooltip title="复制选中内容">
            <Button type="text" size="small" icon={<CopyOutlined />} onClick={handleCopy} />
          </Tooltip>
          {connected && (
            <Tooltip title="断开连接">
              <Button
                type="text"
                size="small"
                danger
                icon={<DisconnectOutlined />}
                onClick={onDisconnect}
              />
            </Tooltip>
          )}
        </div>
      </div>

      {/* 搜索栏 */}
      {searchOpen && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            padding: '4px 10px',
            borderBottom: `1px solid ${token.colorBorderSecondary}`,
            background: token.colorBgElevated,
            flexShrink: 0,
          }}
        >
          <Input
            ref={searchInputRef}
            size="small"
            placeholder="搜索终端内容..."
            value={searchText}
            onChange={(e): void => {
              const val = e.target.value;
              setSearchText(val);
              if (val) {
                searchRef.current?.findNext(val);
              } else {
                searchRef.current?.clearDecorations();
              }
            }}
            onKeyDown={(e): void => {
              if (e.key === 'Enter') {
                e.preventDefault();
                searchRef.current?.findNext(searchText);
              }
            }}
            style={{ width: 200 }}
            allowClear
          />
          <Button
            type="text"
            size="small"
            icon={<DownOutlined />}
            onClick={(): void => {
              searchRef.current?.findNext(searchText);
            }}
          />
          <Button
            type="text"
            size="small"
            icon={<UpOutlined />}
            onClick={(): void => {
              searchRef.current?.findPrevious(searchText);
            }}
          />
          <Button
            type="text"
            size="small"
            icon={<CloseOutlined />}
            onClick={handleSearchClose}
          />
        </div>
      )}

      {/* xterm 容器 + 连接遮罩 */}
      <div style={{ flex: 1, position: 'relative', minHeight: 0, overflow: 'hidden' }}>
        <div
          ref={containerRef}
          style={{ width: '100%', height: '100%', padding: '4px 0' }}
        />
        {(connecting || reconnecting) && (
          <div
            style={{
              position: 'absolute',
              inset: 0,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              background: 'var(--antd-color-bg-elevated)',
              zIndex: 10,
            }}
          >
            <Spin tip={reconnecting ? '连接断开，正在重连...' : '连接中...'} />
          </div>
        )}
      </div>

      {/* 底部状态栏 */}
      <div
        style={{
          padding: '4px 10px',
          display: 'flex',
          alignItems: 'center',
          gap: 16,
          borderTop: `1px solid ${token.colorBorderSecondary}`,
          flexShrink: 0,
          fontSize: 11,
          color: token.colorTextTertiary,
          background: token.colorBgElevated,
        }}
      >
        <span>{statusText}</span>
        <span>{dims.cols}×{dims.rows}</span>
        <span>字号 {fontSize}</span>
        <span style={{ marginLeft: 'auto' }}>{hostAddr || '未连接'}</span>
      </div>

      {/* 右键上下文菜单 */}
      {contextMenu && (
        <TerminalContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          items={contextMenuItems}
          onClose={(): void => setContextMenu(null)}
          colorBgElevated={token.colorBgElevated}
          borderRadiusLG={token.borderRadiusLG}
          boxShadowSecondary={token.boxShadowSecondary}
          colorText={token.colorText}
        />
      )}
    </div>
  );
}
