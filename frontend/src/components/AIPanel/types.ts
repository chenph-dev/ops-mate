import type { convstore } from '@wailsjs/go/models';
import type {
  ApprovalStatus,
  CommandSuggestion,
  SessionState,
} from '@/hooks/useSessions';

/** 对话消息（convstore.Message 别名）。 */
export type Message = convstore.Message;

export type { ApprovalStatus, CommandSuggestion, SessionState };

/** 会话状态角标（session:state 事件值 → 展示 key 与颜色）。text 为 ai 命名空间下的 i18n key。 */
export const STATE_LABEL: Record<string, { text: string; color: string }> = {
  Thinking: { text: 'state.thinking', color: 'blue' },
  AwaitingApproval: { text: 'state.awaitingApproval', color: 'orange' },
  Running: { text: 'state.running', color: 'green' },
};
