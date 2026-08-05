import type { convstore } from "@wailsjs/go/models";
import type {
  ApprovalStatus,
  CommandSuggestion,
  SessionState,
} from "@/hooks/useSessions";

/** 对话消息（convstore.Message 别名）。 */
export type Message = convstore.Message;

export type { ApprovalStatus, CommandSuggestion, SessionState };

/** 会话状态角标文案（session:state 事件值 → 展示文本与颜色）。 */
export const STATE_LABEL: Record<string, { text: string; color: string }> = {
  Thinking: { text: "思考中", color: "blue" },
  AwaitingApproval: { text: "等待审批", color: "orange" },
  Running: { text: "执行中", color: "green" },
};
