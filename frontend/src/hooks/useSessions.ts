import { useCallback, useEffect, useRef, useState } from 'react';
import {
  EnsureSession,
  NewSession,
  SendMessage,
  ApproveCommand,
  ApprovePlan,
  RejectCommand,
  RejectPlan,
  CancelRun,
  ClearMessages,
  LoadMessages,
  ListConversations,
  DeleteConversation,
  RenameConversation,
} from '@wailsjs/go/sessions/SessionsHandler';
import { EventsOn } from '@wailsjs/runtime/runtime';
import i18n from '@/i18n';
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

/** 执行计划（create_plan 工具提交，计划模式）。 */
export interface PlanInfo {
  goal: string;
  steps: string[];
}

export type SessionState =
  'Thinking' | 'AwaitingApproval' | 'Running' | 'Idle' | null;

/** 命令审批状态：待审批 / 已批准 / 已拒绝 / 已自动执行 */
export type ApprovalStatus = 'pending' | 'approved' | 'rejected' | 'auto';

interface AgentEvent {
  sessionId: string;
  data: unknown;
}

/**
 * 每资产单会话模型：
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
  pendingPlan: PlanInfo | null;
  planStatus: ApprovalStatus | null;
  sessionState: SessionState;
  lastError: string | null;
  lastErrorCancelled: boolean;
  runningCommand: string | null;
  runElapsed: number;
  runOutput: string;
  attach: () => Promise<void>;
  refreshConversations: () => Promise<void>;
  switchConversation: (sid: string) => Promise<void>;
  deleteConversation: (sid: string) => Promise<void>;
  renameConversation: (sid: string, title: string) => Promise<void>;
  newConversation: () => Promise<void>;
  sendMessage: (text: string) => Promise<void>;
  clearMessages: () => Promise<void>;
  approve: (command: string) => Promise<void>;
  reject: () => Promise<void>;
  approvePlan: () => Promise<void>;
  rejectPlan: () => Promise<void>;
  cancel: () => Promise<void>;
} {
  const [activeSession, setActiveSession] = useState<string | null>(null);
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [messages, setMessages] = useState<Message[]>([]);
  const [streamingText, setStreamingText] = useState('');
  const [pendingCommand, setPendingCommand] =
    useState<CommandSuggestion | null>(null);
  const [commandStatus, setCommandStatus] = useState<ApprovalStatus | null>(
    null,
  );
  const [pendingPlan, setPendingPlan] = useState<PlanInfo | null>(null);
  // 计划审批独立状态：不复用 commandStatus——否则计划批准后模型执行第一条命令，
  // ai:command 会把 commandStatus 重置为 pending，已批准的计划卡会退回"待审批"。
  const [planStatus, setPlanStatus] = useState<ApprovalStatus | null>(null);
  const [sessionState, setSessionState] = useState<SessionState>(null);
  const [lastError, setLastError] = useState<string | null>(null);
  // 主动取消（后端 ai:error 事件带 cancelled 标记）属正常反馈，用中性色展示而非告警红。
  const [lastErrorCancelled, setLastErrorCancelled] = useState(false);
  // 执行中的命令与计时（run:start 置位，run:result / Idle 清除）
  const [runningCommand, setRunningCommand] = useState<string | null>(null);
  const [runStartAt, setRunStartAt] = useState<number | null>(null);
  const [runElapsed, setRunElapsed] = useState(0);
  // 执行中的实时输出增量（run:output 累积，run:result 后由完整 tool 消息接管）
  const [runOutput, setRunOutput] = useState('');

  const sessionRef = useRef<string | null>(null);
  // 命令轮次标记：每次 ai:command 递增。resync 完成时若 epoch 未变（期间没有新命令提议），
  // 才清空 pendingCommand——防止"上一条命令执行完触发的异步 resync"误清新一轮待审批命令。
  const commandEpoch = useRef(0);
  // 重同步串行化标记：每次 resync 递增。完成时若已有更新的 resync 发起则丢弃本次结果，
  // 保证多个异步 resync 并发时只有最新一次生效，旧结果不覆盖新状态。
  const resyncSeq = useRef(0);
  // 资产切换守卫：attach 发起时记录目标 hostId；attach/refreshConversations 的异步结果
  // 与守卫不符时丢弃——防止快速切换资产时旧资产的 IPC 晚到，把 activeSession /
  // conversations 覆盖成上一资产（表现为打开 mysql 面板却显示其他资产的会话）。
  const attachSeq = useRef(0);
  const expectedHostRef = useRef<string | null>(null);

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
      // 计划审批不涉及命令轮次，重同步后一律清空
      setPendingPlan(null);
      setPlanStatus(null);
      // 同步完成即无进行中的命令（切换会话 / 命令结束 / 回到 Idle）
      setRunningCommand(null);
      setRunStartAt(null);
      setRunOutput('');
    } catch {
      // DB 重同步失败不阻断 UI；下一事件会再次尝试
    }
  }, []);

  /** 执行开始：记录命令、起点与实时输出缓冲（人工批准 run:start 与自动放行 run:auto 共用）。 */
  const startRun = (command: string): void => {
    setRunningCommand(command);
    setRunStartAt(Date.now());
    setRunElapsed(0);
    setRunOutput('');
  };

  /** 刷新当前资产的历史会话列表。 */
  const refreshConversations = useCallback(async (): Promise<void> => {
    if (!hostId) return;
    try {
      const list = await ListConversations(hostId);
      // 资产切换守卫：期间已切到其他资产（attach 更新了期望 hostId），丢弃过期结果。
      if (expectedHostRef.current !== hostId) return;
      setConversations(list ?? []);
    } catch {
      // 列表刷新失败不阻断；打开历史面板时会重试
    }
  }, [hostId]);

  /** 切换到指定历史会话并加载其消息（之后可继续对话）。 */
  const switchConversation = useCallback(
    async (sid: string): Promise<void> => {
      setActiveSession(sid);
      setPendingCommand(null);
      setCommandStatus(null);
      setPlanStatus(null);
      setSessionState(null);
      setLastError(null);
      setLastErrorCancelled(false);
      setStreamingText('');
      await resync(sid);
    },
    [resync],
  );

  /** 资产选中时调用：懒获取/创建会话并加载历史。
   * 竞态防护：attach 含多次 IPC await，切换资产时旧资产的 attach 可能晚于新资产完成，
   * 覆盖 activeSession（显示成上一资产的会话）。用 attachSeq 串行化——只有最新发起的
   * attach 允许写入状态，期间已发起更新 attach 的旧调用直接丢弃。
   */
  const attach = useCallback(async (): Promise<void> => {
    if (!hostId) return;
    expectedHostRef.current = hostId;
    const seq = ++attachSeq.current;
    await refreshConversations();
    if (seq !== attachSeq.current) return;
    const sid = await EnsureSession(hostId);
    if (seq !== attachSeq.current) return;
    setActiveSession(sid);
    setPendingCommand(null);
    setCommandStatus(null);
    setPlanStatus(null);
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
    const sid = await NewSession(
      hostId,
      i18n.t('ai:newConversationTitle', {
        time: new Date().toLocaleString(),
      }),
    );
    setActiveSession(sid);
    setMessages([]);
    setStreamingText('');
    setPendingCommand(null);
    setCommandStatus(null);
    setPendingPlan(null);
    setPlanStatus(null);
    setSessionState(null);
    setLastError(null);
  }, [hostId]);

  /** 重命名会话（历史菜单手动改名）。 */
  const renameConversation = useCallback(
    async (sid: string, title: string): Promise<void> => {
      try {
        await RenameConversation(sid, title);
        await refreshConversations();
      } catch {
        // 重命名失败不影响主流程
      }
    },
    [refreshConversations],
  );

  /** 清空当前会话全部消息（快捷命令 /clear）。 */
  const clearMessages = useCallback(async (): Promise<void> => {
    if (!activeSession) return;
    try {
      await ClearMessages(activeSession);
      setMessages([]);
      setStreamingText('');
      setPendingCommand(null);
      setCommandStatus(null);
      setPendingPlan(null);
      setPlanStatus(null);
      setSessionState(null);
      setLastError(null);
      setLastErrorCancelled(false);
      setRunningCommand(null);
      setRunStartAt(null);
    } catch (e) {
      setLastError(typeof e === 'string' ? e : i18n.t('ai:clearFailed'));
      setLastErrorCancelled(false);
    }
  }, [activeSession]);

  const sendMessage = useCallback(
    async (text: string): Promise<void> => {
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
      setLastErrorCancelled(false);
      } catch (e) {
        // 发送失败：移除乐观消息，避免残留一条后端未落库的本地消息
        setMessages((prev) => prev.filter((m) => m.id !== localId));
        setLastError(typeof e === 'string' ? e : i18n.t('ai:sendFailed'));
        setLastErrorCancelled(false);
      }
    },
    [activeSession],
  );

  // 防重复提交：双击/连点只提交一次，避免第二次后端报"无待审批"后把已批准状态打回 pending
  const approveBusyRef = useRef(false);
  const rejectBusyRef = useRef(false);

  const approve = useCallback(
    async (command: string): Promise<void> => {
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
    },
    [activeSession],
  );

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

  // 计划审批同样防重复提交：双击/连点只提交一次，避免第二次后端报"无待审批计划"后把状态打回 pending
  const approvePlanBusyRef = useRef(false);
  const rejectPlanBusyRef = useRef(false);

  /** 批准执行计划（计划模式）。 */
  const approvePlan = useCallback(async (): Promise<void> => {
    if (!activeSession || approvePlanBusyRef.current) return;
    approvePlanBusyRef.current = true;
    setPlanStatus('approved');
    try {
      await ApprovePlan(activeSession);
    } catch {
      setPlanStatus('pending');
    } finally {
      approvePlanBusyRef.current = false;
    }
  }, [activeSession]);

  /** 拒绝执行计划（计划模式）。 */
  const rejectPlan = useCallback(async (): Promise<void> => {
    if (!activeSession || rejectPlanBusyRef.current) return;
    rejectPlanBusyRef.current = true;
    setPlanStatus('rejected');
    try {
      await RejectPlan(activeSession);
    } catch {
      setPlanStatus('pending');
    } finally {
      rejectPlanBusyRef.current = false;
    }
  }, [activeSession]);

  const cancel = useCallback(async (): Promise<void> => {
    if (!activeSession) return;
    await CancelRun(activeSession);
  }, [activeSession]);

  // Wails 事件订阅（sessionId 过滤）
  useEffect(() => {
    const isMine = (e: AgentEvent): boolean =>
      e.sessionId === sessionRef.current;

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

    const offPlan = EventsOn('ai:plan', (raw: AgentEvent) => {
      if (!isMine(raw)) return;
      setPendingPlan(raw.data as unknown as PlanInfo);
      setPlanStatus('pending');
      setStreamingText('');
    });

    const offRunStart = EventsOn('run:start', (raw: AgentEvent) => {
      if (!isMine(raw)) return;
      const d = raw.data as { command?: string };
      if (d?.command) startRun(d.command);
    });

    const offAutoRun = EventsOn('run:auto', (raw: AgentEvent) => {
      if (!isMine(raw)) return;
      const d = raw.data as { command?: string };
      if (d?.command) startRun(d.command);
    });

    const offRunOutput = EventsOn('run:output', (raw: AgentEvent) => {
      if (!isMine(raw)) return;
      const d = raw.data as { delta?: string };
      if (d?.delta) setRunOutput((prev) => prev + d.delta);
    });

    const offRunResult = EventsOn('run:result', (raw: AgentEvent) => {
      if (!isMine(raw) || !sessionRef.current) return;
      setRunningCommand(null);
      setRunStartAt(null);
      setRunOutput('');
      void resync(sessionRef.current);
    });

    const offError = EventsOn('ai:error', (raw: AgentEvent) => {
      if (!isMine(raw)) return;
      const d = raw.data as { message?: string; cancelled?: boolean };
      setLastError(d?.message ?? i18n.t('ai:unknownError'));
      setLastErrorCancelled(d?.cancelled === true);
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
      offPlan();
      offRunStart();
      offAutoRun();
      offRunOutput();
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
    pendingPlan,
    planStatus,
    sessionState,
    lastError,
    lastErrorCancelled,
    runningCommand,
    runElapsed,
    runOutput,
    attach,
    refreshConversations,
    switchConversation,
    deleteConversation,
    renameConversation,
    newConversation,
    sendMessage,
    clearMessages,
    approve,
    reject,
    approvePlan,
    rejectPlan,
    cancel,
  };
}
