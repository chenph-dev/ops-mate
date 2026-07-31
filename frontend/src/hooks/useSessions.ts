import { useCallback, useState } from 'react';
import {
  NewSession,
  SendMessage,
  ApproveCommand,
  RejectCommand,
  CancelRun,
  ListConversations,
  LoadMessages,
  DeleteConversation,
} from '@wailsjs/go/handler/SessionsHandler';
import type { convstore } from '@wailsjs/go/models';

type Message = convstore.Message;
type Conversation = convstore.Conversation;

export interface CommandSuggestion {
  command: string;
  why: string;
  risk: string;
  assessedRisk: string;
}

export interface WailsEvent {
  sessionId: string;
  data: unknown;
}

export function useSessions(hostId: string | null) {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeSession, setActiveSession] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [pendingCommand, setPendingCommand] = useState<CommandSuggestion | null>(null);
  const [sessionState, setSessionState] = useState<string | null>(null);

  const refreshConversations = useCallback(async () => {
    if (!hostId) return;
    const list = await ListConversations(hostId);
    setConversations(list);
  }, [hostId]);

  const loadMessages = useCallback(async (sessionId: string) => {
    const msgs = await LoadMessages(sessionId);
    setMessages(msgs);
  }, []);

  const selectSession = useCallback(async (sessionId: string) => {
    setActiveSession(sessionId);
    setPendingCommand(null);
    await loadMessages(sessionId);
  }, [loadMessages]);

  const createSession = useCallback(async (title: string) => {
    if (!hostId) return;
    const sid = await NewSession(hostId, title);
    setActiveSession(sid);
    setMessages([]);
    setPendingCommand(null);
    await refreshConversations();
    return sid;
  }, [hostId, refreshConversations]);

  const sendMessage = useCallback(async (text: string) => {
    if (!activeSession) return;
    // 乐观添加用户消息
    const userMsg: Message = {
      id: `local-${Date.now()}`,
      sessionId: activeSession,
      role: 'user',
      content: text,
      toolResult: '',
      ts: Date.now(),
    };
    setMessages((prev) => [...prev, userMsg]);
    await SendMessage(activeSession, text);
  }, [activeSession]);

  const approve = useCallback(async (command: string) => {
    if (!activeSession) return;
    setPendingCommand(null);
    await ApproveCommand(activeSession, command);
  }, [activeSession]);

  const reject = useCallback(async () => {
    if (!activeSession) return;
    setPendingCommand(null);
    await RejectCommand(activeSession);
  }, [activeSession]);

  const cancel = useCallback(async () => {
    if (!activeSession) return;
    await CancelRun(activeSession);
  }, [activeSession]);

  const removeConversation = useCallback(async (sessionId: string) => {
    await DeleteConversation(sessionId);
    if (activeSession === sessionId) {
      setActiveSession(null);
      setMessages([]);
    }
    await refreshConversations();
  }, [activeSession, refreshConversations]);

  const handleEvent = useCallback((event: WailsEvent) => {
    if (event.sessionId !== activeSession) return;
    if (event.data && typeof event.data === 'object') {
      const data = event.data as Record<string, unknown>;
      if ('command' in data) {
        setPendingCommand(data as unknown as CommandSuggestion);
      }
      if ('data' in data && typeof data.data === 'string') {
        setSessionState(data.data);
      }
    }
  }, [activeSession]);

  return {
    conversations,
    activeSession,
    messages,
    pendingCommand,
    sessionState,
    refreshConversations,
    loadMessages,
    selectSession,
    createSession,
    sendMessage,
    approve,
    reject,
    cancel,
    removeConversation,
    handleEvent,
    setPendingCommand,
  };
}
