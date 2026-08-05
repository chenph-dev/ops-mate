import { useCallback, useEffect, useRef, useState } from 'react';
import {
  EnsureSession,
  NewSession,
  SendMessage,
  ApproveCommand,
  RejectCommand,
  CancelRun,
  LoadMessages,
} from '@wailsjs/go/handler/SessionsHandler';
import { EventsOn } from '@wailsjs/runtime/runtime';
import type { convstore } from '@wailsjs/go/models';

type Message = convstore.Message;

export interface CommandSuggestion {
  command: string;
  why: string;
  risk: string;
  assessedRisk: string;
}

export type SessionState =
  | 'Thinking'
  | 'AwaitingApproval'
  | 'Running'
  | 'Idle'
  | null;

interface AgentEvent {
  sessionId: string;
  data: unknown;
}

/**
 * 每主机单会话模型：
 * - attach() 懒获取/创建会话并加载历史（DB 是历史唯一真相源）；
 * - 事件驱动流式态（ai:text 累加到 streamingText）；
 * - 关键节点（命令卡出现、执行完成、回到 Idle）后从 DB 重同步消息。
 */
export function useSessions(hostId: string | null): {
  activeSession: string | null;
  messages: Message[];
  streamingText: string;
  pendingCommand: CommandSuggestion | null;
  sessionState: SessionState;
  lastError: string | null;
  attach: () => Promise<void>;
  newConversation: () => Promise<void>;
  sendMessage: (text: string) => Promise<void>;
  approve: (command: string) => Promise<void>;
  reject: () => Promise<void>;
  cancel: () => Promise<void>;
} {
  const [activeSession, setActiveSession] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [streamingText, setStreamingText] = useState('');
  const [pendingCommand, setPendingCommand] = useState<CommandSuggestion | null>(null);
  const [sessionState, setSessionState] = useState<SessionState>(null);
  const [lastError, setLastError] = useState<string | null>(null);

  const sessionRef = useRef<string | null>(null);

  // 保持 ref 与当前会话同步（react-hooks/refs：禁止在 render 期间写 ref）
  useEffect(() => {
    sessionRef.current = activeSession;
  }, [activeSession]);

  const resync = useCallback(async (sid: string): Promise<void> => {
    try {
      const msgs = await LoadMessages(sid);
      setMessages(msgs ?? []);
      setStreamingText('');
    } catch {
      // DB 重同步失败不阻断 UI；下一事件会再次尝试
    }
  }, []);

  /** 主机选中时调用：懒获取/创建会话并加载历史。 */
  const attach = useCallback(async (): Promise<void> => {
    if (!hostId) return;
    const sid = await EnsureSession(hostId);
    setActiveSession(sid);
    setPendingCommand(null);
    setSessionState(null);
    setLastError(null);
    await resync(sid);
  }, [hostId, resync]);

  /** 新建对话：创建新 conversation 并切换（旧的留库）。 */
  const newConversation = useCallback(async (): Promise<void> => {
    if (!hostId) return;
    const sid = await NewSession(hostId, `对话 ${new Date().toLocaleString()}`);
    setActiveSession(sid);
    setMessages([]);
    setStreamingText('');
    setPendingCommand(null);
    setSessionState(null);
    setLastError(null);
  }, [hostId]);

  const sendMessage = useCallback(async (text: string): Promise<void> => {
    if (!activeSession) return;
    setLastError(null);
    await SendMessage(activeSession, text);
  }, [activeSession]);

  const approve = useCallback(async (command: string): Promise<void> => {
    if (!activeSession) return;
    setPendingCommand(null);
    await ApproveCommand(activeSession, command);
  }, [activeSession]);

  const reject = useCallback(async (): Promise<void> => {
    if (!activeSession) return;
    setPendingCommand(null);
    await RejectCommand(activeSession);
  }, [activeSession]);

  const cancel = useCallback(async (): Promise<void> => {
    if (!activeSession) return;
    await CancelRun(activeSession);
  }, [activeSession]);

  // Wails 事件订阅（sessionId 过滤）
  useEffect(() => {
    const isMine = (e: AgentEvent): boolean => e.sessionId === sessionRef.current;

    const offText = EventsOn('ai:text', (raw: AgentEvent) => {
      if (!isMine(raw)) return;
      const d = raw.data as { delta?: string };
      if (d?.delta) setStreamingText((prev) => prev + d.delta);
    });

    const offCommand = EventsOn('ai:command', (raw: AgentEvent) => {
      if (!isMine(raw)) return;
      setPendingCommand(raw.data as unknown as CommandSuggestion);
      setStreamingText('');
    });

    const offRunResult = EventsOn('run:result', (raw: AgentEvent) => {
      if (!isMine(raw) || !sessionRef.current) return;
      void resync(sessionRef.current);
    });

    const offError = EventsOn('ai:error', (raw: AgentEvent) => {
      if (!isMine(raw)) return;
      const d = raw.data as { message?: string };
      setLastError(d?.message ?? '未知错误');
      setStreamingText('');
    });

    const offState = EventsOn('session:state', (raw: AgentEvent) => {
      if (!isMine(raw)) return;
      const st = raw.data as Exclude<SessionState, null>;
      setSessionState(st);
      if (st === 'Idle' && sessionRef.current) {
        void resync(sessionRef.current);
      }
    });

    return () => {
      offText();
      offCommand();
      offRunResult();
      offError();
      offState();
    };
  }, [resync]);

  return {
    activeSession,
    messages,
    streamingText,
    pendingCommand,
    sessionState,
    lastError,
    attach,
    newConversation,
    sendMessage,
    approve,
    reject,
    cancel,
  };
}
