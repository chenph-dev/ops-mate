import { useState, useCallback, useEffect, useRef } from 'react';
import { EventsOn } from '@wailsjs/runtime/runtime';
import {
  OpenTerminal,
  TerminalInput,
  TerminalResize,
  CloseTerminal,
} from '@wailsjs/go/handler/TerminalHandler';

// 默认 PTY 尺寸，连接后由前端按实际容器尺寸纠正。
const DEFAULT_COLS = 80;
const DEFAULT_ROWS = 24;

// 自动重连最大次数
const MAX_RECONNECT_RETRIES = 5;
// 重连初始延迟（毫秒），采用指数退避
const RECONNECT_BASE_DELAY_MS = 1000;
// 重连最大延迟（毫秒）
const RECONNECT_MAX_DELAY_MS = 30000;

function bytesToBase64(bytes: Uint8Array): string {
  let bin = '';
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin);
}

function base64ToBytes(b64: string): Uint8Array {
  const bin = atob(b64);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return bytes;
}

interface TerminalOutputEvent {
  sessionId: string;
  data: string;
}

interface TerminalClosedEvent {
  sessionId: string;
}

/**
 * 计算指数退避延迟。
 */
function reconnectDelay(attempt: number): number {
  const delay = RECONNECT_BASE_DELAY_MS * 2 ** attempt;
  return Math.min(delay, RECONNECT_MAX_DELAY_MS);
}

/**
 * 真实 SSH 交互终端 hook。
 * 管理会话生命周期，输出经 Wails 事件推送，由 setOutputHandler 注册的回调写入 xterm。
 * 支持异常断开后的自动重连。
 */
export function useTerminal(hostID: string | null): {
  connected: boolean;
  connecting: boolean;
  reconnecting: boolean;
  reconnectCount: number;
  open: (id: string) => Promise<void>;
  close: () => Promise<void>;
  sendData: (data: string) => void;
  resize: (cols: number, rows: number) => void;
  setOutputHandler: (cb: (data: Uint8Array) => void) => void;
} {
  const [connected, setConnected] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const [reconnecting, setReconnecting] = useState(false);
  const [reconnectCount, setReconnectCount] = useState(0);
  const sessionIdRef = useRef<string | null>(null);
  const outputRef = useRef<(data: Uint8Array) => void>(() => {});
  const hostIDRef = useRef<string | null>(hostID);
  const manualCloseRef = useRef(false);
  const reconnectCountRef = useRef(0);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    hostIDRef.current = hostID;
  });

  const close = useCallback(async () => {
    manualCloseRef.current = true;
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
    const sid = sessionIdRef.current;
    sessionIdRef.current = null;
    setConnected(false);
    setReconnecting(false);
    setReconnectCount(0);
    reconnectCountRef.current = 0;
    if (sid) {
      try {
        await CloseTerminal(sid);
      } catch {
        // 忽略关闭错误
      }
    }
  }, []);

  const open = useCallback(
    async (id: string) => {
      await close();
      manualCloseRef.current = false;
      hostIDRef.current = id;
      setConnecting(true);
      setReconnecting(false);
      try {
        const sid = await OpenTerminal(id, DEFAULT_COLS, DEFAULT_ROWS);
        sessionIdRef.current = sid;
        setConnected(true);
        setReconnectCount(0);
        reconnectCountRef.current = 0;
      } finally {
        setConnecting(false);
      }
    },
    [close],
  );

  // 订阅后端输出/关闭事件
  useEffect(() => {
    const offOut = EventsOn('terminal:output', (e: TerminalOutputEvent) => {
      if (e.sessionId !== sessionIdRef.current) return;
      outputRef.current(base64ToBytes(e.data));
    });
    const offClosed = EventsOn('terminal:closed', (e: TerminalClosedEvent) => {
      if (e.sessionId !== sessionIdRef.current) return;
      sessionIdRef.current = null;
      setConnected(false);

      // 用户主动关闭时不重连
      if (manualCloseRef.current) {
        setReconnecting(false);
        return;
      }

      // 没有 hostID 或已达最大重试次数时停止重连
      const currentHostID = hostIDRef.current;
      if (
        !currentHostID ||
        reconnectCountRef.current >= MAX_RECONNECT_RETRIES
      ) {
        setReconnecting(false);
        return;
      }

      setReconnecting(true);
      const attempt = reconnectCountRef.current;
      const delay = reconnectDelay(attempt);
      reconnectTimerRef.current = setTimeout(() => {
        reconnectCountRef.current += 1;
        setReconnectCount(reconnectCountRef.current);
        void open(currentHostID);
      }, delay);
    });
    return () => {
      offOut();
      offClosed();
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
      }
    };
  }, [open]);

  const sendData = useCallback((data: string) => {
    const sid = sessionIdRef.current;
    if (!sid) return;
    const bytes = new TextEncoder().encode(data);
    TerminalInput(sid, bytesToBase64(bytes)).catch(() => {});
  }, []);

  const resize = useCallback((cols: number, rows: number) => {
    const sid = sessionIdRef.current;
    if (!sid) return;
    TerminalResize(sid, cols, rows).catch(() => {});
  }, []);

  // 注册输出写入回调（xterm.write）
  const setOutputHandler = useCallback((cb: (data: Uint8Array) => void) => {
    outputRef.current = cb;
  }, []);

  return {
    connected,
    connecting,
    reconnecting,
    reconnectCount,
    open,
    close,
    sendData,
    resize,
    setOutputHandler,
  };
}
