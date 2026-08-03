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
 * 真实 SSH 交互终端 hook。
 * 管理会话生命周期，输出经 Wails 事件推送，由 setOutputHandler 注册的回调写入 xterm。
 */
export function useTerminal() {
  const [connected, setConnected] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const sessionIdRef = useRef<string | null>(null);
  const outputRef = useRef<(data: Uint8Array) => void>(() => {});

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
    });
    return () => {
      offOut();
      offClosed();
    };
  }, []);

  const close = useCallback(async () => {
    const sid = sessionIdRef.current;
    sessionIdRef.current = null;
    setConnected(false);
    if (sid) {
      try {
        await CloseTerminal(sid);
      } catch {
        // 忽略关闭错误
      }
    }
  }, []);

  const open = useCallback(
    async (hostID: string) => {
      await close();
      setConnecting(true);
      try {
        const sid = await OpenTerminal(hostID, DEFAULT_COLS, DEFAULT_ROWS);
        sessionIdRef.current = sid;
        setConnected(true);
      } finally {
        setConnecting(false);
      }
    },
    [close],
  );

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

  return { connected, connecting, open, close, sendData, resize, setOutputHandler };
}