import { Spin, theme, type InputRef } from "antd";
import { Terminal as XTerm } from "@xterm/xterm";
import { CanvasAddon } from "@xterm/addon-canvas";
import { FitAddon } from "@xterm/addon-fit";
import { LigaturesAddon } from "@xterm/addon-ligatures";
import { SearchAddon } from "@xterm/addon-search";
import { SerializeAddon } from "@xterm/addon-serialize";
import { Unicode11Addon } from "@xterm/addon-unicode11";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { WebglAddon } from "@xterm/addon-webgl";
import "@xterm/xterm/css/xterm.css";
import { ClipboardGetText, ClipboardSetText } from "@wailsjs/runtime/runtime";
import { ListHostCommands } from "@wailsjs/go/handler/TerminalHandler";
import { useCallback, useEffect, useRef, useState } from "react";
import { terminalTheme } from "@/theme";
import { useCommandCompletion } from "@/hooks/useCommandCompletion";
import CompletionPopup from "./CompletionPopup";
import TerminalHeader from "./TerminalHeader";
import SearchBar from "./SearchBar";
import StatusBar from "./StatusBar";
import TerminalContextMenu, { type ContextMenuItem } from "./TerminalContextMenu";

interface TerminalProps {
  isDark: boolean;
  connected: boolean;
  connecting: boolean;
  reconnecting?: boolean;
  reconnectCount?: number;
  hostID?: string;
  hostName: string;
  hostAddr: string;
  aiOpen: boolean;
  onToggleAI: () => void;
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

export default function Terminal({
  isDark,
  connected,
  connecting,
  reconnecting = false,
  reconnectCount = 0,
  hostID,
  hostName,
  hostAddr,
  aiOpen,
  onToggleAI,
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
  const canvasRef = useRef<CanvasAddon | null>(null);
  const ligaturesRef = useRef<LigaturesAddon | null>(null);
  const serializeRef = useRef<SerializeAddon | null>(null);
  const unicodeRef = useRef<Unicode11Addon | null>(null);
  const searchInputRef = useRef<InputRef>(null);
  const [dims, setDims] = useState({ cols: 80, rows: 24 });
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchText, setSearchText] = useState("");
  const [fontSize, setFontSize] = useState(DEFAULT_FONT_SIZE);
  const [contextMenu, setContextMenu] = useState<{
    x: number;
    y: number;
  } | null>(null);

  const {
    completion,
    handleInputData,
    handleKeyEvent,
    onCursorMove,
    closeCompletion,
    acceptMatch,
    loadRemoteCommands,
  } = useCommandCompletion(xtermRef, onData);

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
      fontFamily:
        '"Cascadia Code", "Fira Code", "Consolas", "Monaco", monospace',
      cursorBlink: true,
      scrollback: 10000,
      convertEol: true,
      allowProposedApi: true,
    });
    const fit = new FitAddon();
    xterm.loadAddon(fit);
    // 可点击链接（URL/文件路径）
    xterm.loadAddon(new WebLinksAddon());
    // Unicode 11 支持（更完整的 emoji/生僻字）
    try {
      const unicode = new Unicode11Addon();
      xterm.loadAddon(unicode);
      unicodeRef.current = unicode;
    } catch {
      // 忽略：部分环境或字体不支持
    }
    // 字体连字（Fira Code / Cascadia Code 支持）
    try {
      const ligatures = new LigaturesAddon();
      xterm.loadAddon(ligatures);
      ligaturesRef.current = ligatures;
    } catch {
      // 忽略：字体或环境不支持
    }
    // WebGL 加速渲染（失败则回退到 Canvas 渲染器）
    try {
      xterm.loadAddon(new WebglAddon());
    } catch {
      try {
        const canvas = new CanvasAddon();
        xterm.loadAddon(canvas);
        canvasRef.current = canvas;
      } catch {
        // 再失败则使用默认 DOM 渲染器
      }
    }
    const search = new SearchAddon();
    xterm.loadAddon(search);
    searchRef.current = search;
    // 序列化（导出终端内容）
    const serialize = new SerializeAddon();
    xterm.loadAddon(serialize);
    serializeRef.current = serialize;
    xterm.open(containerRef.current);
    fit.fit();
    xtermRef.current = xterm;
    fitRef.current = fit;

    // 键盘输入 → 后端（同时给命令补全 hook 一份）
    xterm.onData((d) => {
      handleInputData(d);
      onDataRef.current(d);
    });
    // 光标移动时更新补全浮层位置
    xterm.onCursorMove(onCursorMove);
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
      // 命令补全优先消费 Tab / ↑ / ↓ / Enter / Esc
      if (!handleKeyEvent(e)) {
        return false;
      }
      const isModifier = e.ctrlKey || e.metaKey;
      if (isModifier && e.key.toLowerCase() === "f") {
        e.preventDefault();
        setSearchOpen(true);
        setTimeout(() => searchInputRef.current?.focus(), 0);
        return false; // 阻止 xterm 处理
      }
      if (isModifier && (e.key === "+" || e.key === "=")) {
        e.preventDefault();
        handleZoomIn();
        return false;
      }
      if (isModifier && e.key === "-") {
        e.preventDefault();
        handleZoomOut();
        return false;
      }
      if (isModifier && e.key === "0") {
        e.preventDefault();
        handleZoomReset();
        return false;
      }
      if (e.key === "Escape") {
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
      canvasRef.current = null;
      ligaturesRef.current = null;
      serializeRef.current = null;
      unicodeRef.current = null;
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

  // 连接成功后异步抓取该主机命令列表，用于补全
  useEffect(() => {
    if (connected && hostID) {
      void loadRemoteCommands(hostID, ListHostCommands);
    }
  }, [connected, hostID, loadRemoteCommands]);

  // 断开或字体大小变化时关闭补全气泡
  useEffect(() => {
    if (!connected) {
      closeCompletion();
    }
  }, [connected, closeCompletion]);

  useEffect(() => {
    closeCompletion();
  }, [fontSize, closeCompletion]);

  // 断开时（从已连接 → 未连接）在终端末尾写一行提示（非重连场景）
  const prevConnectedRef = useRef(false);
  useEffect(() => {
    if (prevConnectedRef.current && !connected && !reconnecting) {
      xtermRef.current?.write(
        "\x1b[90m\r\n[会话已断开] 双击主机重新连接\x1b[0m\r\n",
      );
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
    container.addEventListener("contextmenu", handleContextMenu);
    return () => {
      container.removeEventListener("contextmenu", handleContextMenu);
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
    container.addEventListener("wheel", handleWheel, { passive: false });
    return () => {
      container.removeEventListener("wheel", handleWheel);
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

  const handleSearchChange = useCallback(
    (value: string): void => {
      setSearchText(value);
      if (value) {
        searchRef.current?.findNext(value);
      } else {
        searchRef.current?.clearDecorations();
      }
    },
    [],
  );

  const handleSearchNext = useCallback((): void => {
    searchRef.current?.findNext(searchText);
  }, [searchText]);

  const handleSearchPrev = useCallback((): void => {
    searchRef.current?.findPrevious(searchText);
  }, [searchText]);

  const handleSearchClose = useCallback((): void => {
    setSearchOpen(false);
    setSearchText("");
    searchRef.current?.clearDecorations();
  }, []);

  const contextMenuItems: ContextMenuItem[] = [
    { key: "copy", label: "复制", onClick: handleCopy },
    { key: "paste", label: "粘贴", onClick: handlePaste },
    { key: "select-all", label: "全选", onClick: handleSelectAll },
    { key: "clear", label: "清空", onClick: handleClear },
  ];

  const statusDot = connecting ? "⏳" : connected ? "🟢" : "⚪";
  const statusText = connecting
    ? "连接中"
    : reconnecting
      ? `重连中 (${reconnectCount}/${MAX_RECONNECT_RETRIES})`
      : connected
        ? "已连接"
        : "未连接";

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        height: "100%",
        position: "relative",
        borderRadius: 8,
        border: `1px solid ${token.colorBorderSecondary}`,
        overflow: "hidden",
        background: terminalTheme(isDark).background,
        marginLeft: 5,
      }}
    >
      <TerminalHeader
        hostName={hostName}
        hostAddr={hostAddr}
        statusDot={statusDot}
        statusText={statusText}
        connected={connected}
        fontSize={fontSize}
        maxFontSize={MAX_FONT_SIZE}
        minFontSize={MIN_FONT_SIZE}
        aiOpen={aiOpen}
        onToggleAI={onToggleAI}
        onZoomIn={handleZoomIn}
        onZoomOut={handleZoomOut}
        onSearch={(): void => {
          setSearchOpen(true);
          setTimeout(() => searchInputRef.current?.focus(), 0);
        }}
        onClear={handleClear}
        onCopy={handleCopy}
        onDisconnect={onDisconnect}
      />

      {/* 搜索栏 */}
      {searchOpen && (
        <SearchBar
          searchText={searchText}
          searchInputRef={searchInputRef}
          onSearchChange={handleSearchChange}
          onSearchNext={handleSearchNext}
          onSearchPrev={handleSearchPrev}
          onSearchClose={handleSearchClose}
        />
      )}

      {/* xterm 容器 + 连接遮罩 */}
      <div
        style={{
          flex: 1,
          position: "relative",
          minHeight: 0,
          overflow: "hidden",
        }}
      >
        <div
          ref={containerRef}
          style={{ width: "100%", height: "100%", padding: "4px" }}
        />
        {(connecting || reconnecting) && (
          <div
            style={{
              position: "absolute",
              inset: 0,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              background: "var(--antd-color-bg-elevated)",
              zIndex: 10,
            }}
          >
            <Spin description={reconnecting ? "连接断开，正在重连..." : "连接中..."} />
          </div>
        )}
      </div>

      {/* 底部状态栏 */}
      <StatusBar
        statusText={statusText}
        dims={dims}
        fontSize={fontSize}
        hostAddr={hostAddr}
      />

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

      {/* 命令补全气泡 */}
      {completion.open && completion.position && (
        <CompletionPopup
          x={completion.position.x}
          y={completion.position.y}
          matches={completion.matches}
          selectedIndex={completion.selectedIndex}
          prefix={completion.prefix}
          onSelect={acceptMatch}
        />
      )}
    </div>
  );
}
