import { useCallback, useEffect, useRef, useState } from 'react';
import {
  EnsureSession,
  NewSession,
  SendMessage,
  ApproveCommand,
  RejectCommand,
  CancelRun,
  LoadMessages,
  ListConversations,
  DeleteConversation,
} from '@wailsjs/go/handler/SessionsHandler';
import { EventsOn } from '@wailsjs/runtime/runtime';
import type { convstore } from '@wailsjs/go/models';

type Message = convstore.Message;
type Conversation = convstore.Conversation;

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

/** 命令审批状态：待审批 / 已批准 / 已拒绝 */
export type ApprovalStatus = 'pending' | 'approved' | 'rejected';

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
  conversations: Conversation[];
  messages: Message[];
  streamingText: string;
  pendingCommand: CommandSuggestion | null;
  commandStatus: ApprovalStatus | null;
  sessionState: SessionState;
  lastError: string | null;
  attach: () => Promise<void>;
  refreshConversations: () => Promise<void>;
  switchConversation: (sid: string) => Promise<void>;
  deleteConversation: (sid: string) => Promise<void>;
  newConversation: () => Promise<void>;
  sendMessage: (text: string) => Promise<void>;
  approve: (command: string) => Promise<void>;
  reject: () => Promise<void>;
  cancel: () => Promise<void>;
} {
  const [activeSession, setActiveSession] = useState<string | null>(null);
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [messages, setMessages] = useState<Message[]>([]);
  const [streamingText, setStreamingText] = useState('');
  const [pendingCommand, setPendingCommand] = useState<CommandSuggestion | null>(null);
  const [commandStatus, setCommandStatus] = useState<ApprovalStatus | null>(null);
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
      // 历史重同步后由消息列表接管审批卡显示
      setPendingCommand(null);
      setCommandStatus(null);
    } catch {
      // DB 重同步失败不阻断 UI；下一事件会再次尝试
    }
  }, []);

  /** 刷新当前主机的历史会话列表。 */
  const refreshConversations = useCallback(async (): Promise<void> => {
    if (!hostId) return;
    try {
      const list = await ListConversations(hostId);
      setConversations(list ?? []);
    } catch {
      // 列表刷新失败不阻断；打开历史面板时会重试
    }
  }, [hostId]);

  /** 切换到指定历史会话并加载其消息（之后可继续对话）。 */
  const switchConversation = useCallback(async (sid: string): Promise<void> => {
    setActiveSession(sid);
    setPendingCommand(null);
    setCommandStatus(null);
    setSessionState(null);
    setLastError(null);
    setStreamingText('');
    await resync(sid);
  }, [resync]);

  /** 主机选中时调用：懒获取/创建会话并加载历史。 */
  const attach = useCallback(async (): Promise<void> => {
    if (!hostId) return;
    await refreshConversations();
    const sid = await EnsureSession(hostId);
    setActiveSession(sid);
    setPendingCommand(null);
    setCommandStatus(null);
    setSessionState(null);
    setLastError(null);
    await resync(sid);
  }, [hostId, resync, refreshConversations]);

  /** 删除历史会话；若删的是当前会话则切回最新会话。 */
  const deleteConversation = useCallback(
    async (sid: string): Promise<void> => {
      await DeleteConversation(sid);
      if (activeSession === sid) {
        await attach();
      } else {
        await refreshConversations();
      }
    },
    [activeSession, attach, refreshConversations],
  );

  /** 新建对话：创建新 conversation 并切换（旧的留库）。 */
  const newConversation = useCallback(async (): Promise<void> => {
    if (!hostId) return;
    const sid = await NewSession(hostId, `对话 ${new Date().toLocaleString()}`);
    setActiveSession(sid);
    setMessages([]);
    setStreamingText('');
    setPendingCommand(null);
    setCommandStatus(null);
    setSessionState(null);
    setLastError(null);
  }, [hostId]);

  const sendMessage = useCallback(async (text: string): Promise<void> => {
    if (!activeSession) return;
    setLastError(null);
    await SendMessage(activeSession, text);
  }, [activeSession]);

  // 防重复提交：双击/连点只提交一次，避免第二次后端报"无待审批"后把已批准状态打回 pending
  const approveBusyRef = useRef(false);
  const rejectBusyRef = useRef(false);

  const approve = useCallback(async (command: string): Promise<void> => {
    if (!activeSession || approveBusyRef.current) return;
    approveBusyRef.current = true;
    // 卡片保留，状态切为"已批准"，执行完成后由历史卡接管
    setCommandStatus('approved');
    try {
      await ApproveCommand(activeSession, command);
    } catch {
      setCommandStatus('pending');
    } finally {
      approveBusyRef.current = false;
    }
  }, [activeSession]);

  const reject = useCallback(async (): Promise<void> => {
    if (!activeSession || rejectBusyRef.current) return;
    rejectBusyRef.current = true;
    setCommandStatus('rejected');
    try {
      await RejectCommand(activeSession);
    } catch {
      setCommandStatus('pending');
    } finally {
      rejectBusyRef.current = false;
    }
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
      setCommandStatus('pending');
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
    conversations,
    messages,
    streamingText,
    pendingCommand,
    commandStatus,
    sessionState,
    lastError,
    attach,
    refreshConversations,
    switchConversation,
    deleteConversation,
    newConversation,
    sendMessage,
    approve,
    reject,
    cancel,
  };
}
