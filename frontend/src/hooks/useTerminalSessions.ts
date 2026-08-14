import { useState, useCallback, useEffect, useRef } from 'react';
import { EventsOn } from '@wailsjs/runtime/runtime';
import {
  OpenTerminal,
  TerminalInput,
  TerminalResize,
  CloseTerminal,
} from '@wailsjs/go/terminal/TerminalHandler';

// 多标签终端会话：同一主机可开多个标签，每标签独立 SSH 会话、独立状态。
// 后端 TerminalHandler 已支持多会话（sessionId 路由），本 hook 负责前端多实例管理。

const MAX_TABS = 6; // 标签上限
const DEFAULT_COLS = 80;
const DEFAULT_ROWS = 24;
const MAX_RECONNECT_RETRIES = 5;
const RECONNECT_BASE_DELAY_MS = 1000;
const RECONNECT_MAX_DELAY_MS = 30000;

/** 展示给 Terminal 组件的标签状态。 */
interface TabInst {
  key: string;
  hostID: string;
  hostName: string;
  hostAddr: string;
  connected: boolean;
  connecting: boolean;
  reconnecting: boolean;
  reconnectCount: number;
}

/** 标签的连接细节（存 ref，避免闭包过期）。 */
interface TabRef {
  hostID: string;
  sessionId: string | null;
  output: (data: Uint8Array) => void;
  manualClose: boolean;
  reconnectCount: number;
  timer: ReturnType<typeof setTimeout> | null;
}

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

function reconnectDelay(attempt: number): number {
  const delay = RECONNECT_BASE_DELAY_MS * 2 ** attempt;
  return Math.min(delay, RECONNECT_MAX_DELAY_MS);
}

let tabSeq = 0;
function nextKey(): string {
  tabSeq += 1;
  return `t-${tabSeq}`;
}

export function useTerminalSessions(): {
  tabs: TabInst[];
  activeKey: string | null;
  open: (hostID: string, hostName: string, hostAddr: string) => void;
  activate: (key: string) => void;
  closeTab: (key: string) => void;
  refresh: (key: string) => void;
  sendData: (key: string, data: string) => void;
  resize: (key: string, cols: number, rows: number) => void;
  setOutputHandler: (key: string, cb: (data: Uint8Array) => void) => void;
  closeAll: () => void;
} {
  const [tabs, setTabs] = useState<TabInst[]>([]);
  const [activeKey, setActiveKey] = useState<string | null>(null);
  const refs = useRef<Record<string, TabRef>>({});
  const tabsRef = useRef<TabInst[]>([]);
  useEffect(() => {
    tabsRef.current = tabs;
  }, [tabs]);

  const patchTab = useCallback((key: string, patch: Partial<TabInst>) => {
    setTabs((prev) =>
      prev.map((t) => (t.key === key ? { ...t, ...patch } : t)),
    );
  }, []);

  // 建立/重连指定标签的 SSH 会话
  const connect = useCallback(
    (key: string) => {
      const r = refs.current[key];
      if (!r) return;
      patchTab(key, { connecting: true, reconnecting: false });
      OpenTerminal(r.hostID, DEFAULT_COLS, DEFAULT_ROWS)
        .then((sid) => {
          r.sessionId = sid;
          patchTab(key, { connected: true, connecting: false });
        })
        .catch(() => {
          patchTab(key, { connected: false, connecting: false });
        });
    },
    [patchTab],
  );

  const activate = useCallback((key: string) => setActiveKey(key), []);

  const open = useCallback(
    (hostID: string, hostName: string, hostAddr: string) => {
      if (tabsRef.current.length >= MAX_TABS) {
        return; // 已达上限，调用方负责提示
      }
      const key = nextKey();
      refs.current[key] = {
        hostID,
        sessionId: null,
        output: () => {},
        manualClose: false,
        reconnectCount: 0,
        timer: null,
      };
      setTabs((prev) => [
        ...prev,
        {
          key,
          hostID,
          hostName,
          hostAddr,
          connected: false,
          connecting: true,
          reconnecting: false,
          reconnectCount: 0,
        },
      ]);
      setActiveKey(key);
      void connect(key);
    },
    [connect],
  );

  const closeTab = useCallback((key: string) => {
    const r = refs.current[key];
    if (r) {
      r.manualClose = true;
      if (r.timer) clearTimeout(r.timer);
      if (r.sessionId) CloseTerminal(r.sessionId).catch(() => {});
      delete refs.current[key];
    }
    setTabs((prev) => {
      const idx = prev.findIndex((t) => t.key === key);
      const next = prev.filter((t) => t.key !== key);
      // 若关闭的是激活标签，激活相邻标签
      setActiveKey((cur) => {
        if (cur !== key) return cur;
        return next[Math.min(idx, next.length - 1)]?.key ?? null;
      });
      return next;
    });
  }, []);

  const refresh = useCallback(
    (key: string) => {
      const r = refs.current[key];
      if (!r) return;
      r.manualClose = false;
      if (r.timer) clearTimeout(r.timer);
      if (r.sessionId) {
        const sid = r.sessionId;
        r.sessionId = null;
        CloseTerminal(sid).catch(() => {});
      }
      r.reconnectCount = 0;
      patchTab(key, { connected: false });
      void connect(key);
    },
    [connect, patchTab],
  );

  const sendData = useCallback((key: string, data: string) => {
    const sid = refs.current[key]?.sessionId;
    if (!sid) return;
    const bytes = new TextEncoder().encode(data);
    TerminalInput(sid, bytesToBase64(bytes)).catch(() => {});
  }, []);

  const resize = useCallback((key: string, cols: number, rows: number) => {
    const sid = refs.current[key]?.sessionId;
    if (!sid) return;
    TerminalResize(sid, cols, rows).catch(() => {});
  }, []);

  const setOutputHandler = useCallback(
    (key: string, cb: (data: Uint8Array) => void) => {
      if (refs.current[key]) {
        refs.current[key].output = cb;
      }
    },
    [],
  );

  // 关闭全部标签（页面卸载时调用）
  const closeAll = useCallback(() => {
    for (const key in refs.current) {
      const r = refs.current[key];
      r.manualClose = true;
      if (r.timer) clearTimeout(r.timer);
      if (r.sessionId) CloseTerminal(r.sessionId).catch(() => {});
      delete refs.current[key];
    }
    setTabs([]);
    setActiveKey(null);
  }, []);

  // 全局订阅后端输出/关闭事件，按 sessionId 路由到对应标签
  useEffect(() => {
    const offOut = EventsOn('terminal:output', (e) => {
      for (const key in refs.current) {
        if (refs.current[key].sessionId === e.sessionId) {
          refs.current[key].output(base64ToBytes(e.data));
          break;
        }
      }
    });
    const offClosed = EventsOn('terminal:closed', (e) => {
      for (const key in refs.current) {
        const r = refs.current[key];
        if (r.sessionId !== e.sessionId) continue;
        r.sessionId = null;
        patchTab(key, { connected: false });

        // 用户主动关闭不重连
        if (r.manualClose) {
          patchTab(key, { reconnecting: false });
          return;
        }
        if (r.reconnectCount >= MAX_RECONNECT_RETRIES) {
          patchTab(key, { reconnecting: false });
          return;
        }
        const attempt = r.reconnectCount;
        r.reconnectCount += 1;
        patchTab(key, { reconnecting: true, reconnectCount: attempt });
        r.timer = setTimeout(() => connect(key), reconnectDelay(attempt));
        return;
      }
    });
    return () => {
      offOut();
      offClosed();
    };
  }, [connect, patchTab]);

  return {
    tabs,
    activeKey,
    open,
    activate,
    closeTab,
    refresh,
    sendData,
    resize,
    setOutputHandler,
    closeAll,
  };
}
