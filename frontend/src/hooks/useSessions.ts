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

// 乐观消息临时 ID 生成器：发送时本地插入的用户消息，resync 后被真实落库消息替换。
let localMsgSeq = 0;
function nextLocalId(): string {
  localMsgSeq += 1;
  return `local-${Date.now()}-${localMsgSeq}`;
}

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
  runningCommand: string | null;
  runElapsed: number;
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
  // 执行中的命令与计时（run:start 置位，run:result / Idle 清除）
  const [runningCommand, setRunningCommand] = useState<string | null>(null);
  const [runStartAt, setRunStartAt] = useState<number | null>(null);
  const [runElapsed, setRunElapsed] = useState(0);

  const sessionRef = useRef<string | null>(null);
  // 命令轮次标记：每次 ai:command 递增。resync 完成时若 epoch 未变（期间没有新命令提议），
  // 才清空 pendingCommand——防止"上一条命令执行完触发的异步 resync"误清新一轮待审批命令。
  const commandEpoch = useRef(0);
  // 重同步串行化标记：每次 resync 递增。完成时若已有更新的 resync 发起则丢弃本次结果，
  // 保证多个异步 resync 并发时只有最新一次生效，旧结果不覆盖新状态。
  const resyncSeq = useRef(0);

  // 保持 ref 与当前会话同步（react-hooks/refs：禁止在 render 期间写 ref）
  useEffect(() => {
    sessionRef.current = activeSession;
  }, [activeSession]);

  // 执行中计时：每秒依据 runStartAt 刷新已用秒数。
  useEffect(() => {
    if (!runningCommand || runStartAt === null) return;
    const id = setInterval(() => {
      setRunElapsed(Math.max(0, Math.floor((Date.now() - runStartAt) / 1000)));
    }, 1000);
    return () => clearInterval(id);
  }, [runningCommand, runStartAt]);

  const resync = useCallback(async (sid: string): Promise<void> => {
    try {
      const seq = ++resyncSeq.current;
      const epochAtStart = commandEpoch.current;
      const msgs = await LoadMessages(sid);
      // 过期重同步丢弃：期间已有更新的 resync 发起，本次结果不落状态（串行化保证）。
      if (seq !== resyncSeq.current) return;
      setMessages(msgs ?? []);
      setStreamingText('');
      // 竞态防御：resync 期间若收到新的 ai:command（新一轮命令已提议，pendingCommand 已置位），
      // 保留实时审批卡；否则历史重同步后由消息列表接管审批卡显示。
      if (commandEpoch.current === epochAtStart) {
        setPendingCommand(null);
        setCommandStatus(null);
      }
      // 同步完成即无进行中的命令（切换会话 / 命令结束 / 回到 Idle）
      setRunningCommand(null);
      setRunStartAt(null);
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
    const localId = nextLocalId();
    // 乐观更新：用户消息立即显示在对话流中（不等模型输出完）；
    // 后端 resync（Idle/run:result）后由真实落库消息整体替换，内容一致无缝。
    setMessages((prev) => [
      ...prev,
      {
        id: localId,
        sessionId: activeSession,
        role: 'user',
        content: text,
        toolResult: '',
        toolCalls: '',
        toolCallId: '',
        toolName: '',
        approvalStatus: '',
        ts: Math.floor(Date.now() / 1000),
      },
    ]);
    setStreamingText('');
    // 后端 SendMessage 在「会话进行中」「AI 后端不可用」等场景直接返回 error 而不发 ai:error 事件，
    // 这里捕获并展示，避免静默失败（表现为"发送没反应"）。
    try {
      await SendMessage(activeSession, text);
      setLastError(null);
    } catch (e) {
      // 发送失败：移除乐观消息，避免残留一条后端未落库的本地消息
      setMessages((prev) => prev.filter((m) => m.id !== localId));
      setLastError(typeof e === 'string' ? e : '发送失败，请查看后端日志');
    }
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
      // 标记新一轮命令提议，供 resync 完成时判断是否误清
      commandEpoch.current += 1;
      setPendingCommand(raw.data as unknown as CommandSuggestion);
      setCommandStatus('pending');
      setStreamingText('');
    });

    const offRunStart = EventsOn('run:start', (raw: AgentEvent) => {
      if (!isMine(raw)) return;
      const d = raw.data as { command?: string };
      if (d?.command) {
        setRunningCommand(d.command);
        setRunStartAt(Date.now());
        setRunElapsed(0);
      }
    });

    const offRunResult = EventsOn('run:result', (raw: AgentEvent) => {
      if (!isMine(raw) || !sessionRef.current) return;
      setRunningCommand(null);
      setRunStartAt(null);
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
      offRunStart();
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
    runningCommand,
    runElapsed,
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
