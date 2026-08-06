import { Button, Tag, Tooltip } from "antd";
import {
  RobotOutlined,
  CompressOutlined,
  PlusOutlined,
  StopOutlined,
} from "@ant-design/icons";
import type { configstore, convstore } from "@wailsjs/go/models";
import type { SessionState } from "./types";
import HistoryPopover from "./HistoryPopover";

interface PanelHeaderProps {
  hostName: string;
  aiCfg: configstore.AIConfig | null;
  configured: boolean;
  cfgLoading: boolean;
  stateMeta: { text: string; color: string } | null;
  sessionState: SessionState;
  sshConnected: boolean;
  conversations: convstore.Conversation[];
  activeSession: string | null;
  onSwitchConversation: (sid: string) => Promise<void>;
  onDeleteConversation: (sid: string) => Promise<void>;
  onRenameConversation: (sid: string, title: string) => Promise<void>;
  onRefreshConversations: () => Promise<void>;
  onCancel: () => Promise<void>;
  onNewConversation: () => Promise<void>;
  onToggleCollapse: () => void;
}

/** 抽屉标题栏：智能体信息 + 会话操作按钮。宽度由面板左边缘拖拽条调整。 */
export default function PanelHeader({
  hostName,
  aiCfg,
  configured,
  cfgLoading,
  stateMeta,
  sessionState,
  sshConnected,
  conversations,
  activeSession,
  onSwitchConversation,
  onDeleteConversation,
  onRenameConversation,
  onRefreshConversations,
  onCancel,
  onNewConversation,
  onToggleCollapse,
}: PanelHeaderProps): React.JSX.Element {
  return (
    <div
      style={{
        padding: "6px 10px",
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        borderBottom: "1px solid var(--antd-color-border-secondary)",
        flexShrink: 0,
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
        <RobotOutlined style={{ color: "var(--antd-color-primary)" }} />
        <span style={{ fontSize: 12, fontWeight: 600 }}>智能体</span>
        <span
          style={{ fontSize: 11, color: "var(--antd-color-text-secondary)" }}
        >
          · {hostName}
        </span>
        {configured && aiCfg?.model && (
          <span
            style={{
              fontSize: 11,
              color: "var(--antd-color-text-secondary)",
            }}
          >
            · {aiCfg.model}
          </span>
        )}
        {!sshConnected && <Tag color="warning">SSH 已断开</Tag>}
        {!configured && !cfgLoading && <Tag color="error">未配置</Tag>}
        {stateMeta && <Tag color={stateMeta.color}>{stateMeta.text}</Tag>}
      </div>
      <div
        style={{ display: "flex", gap: 4 }}
        onMouseDown={(e) => e.stopPropagation()}
      >
        {(sessionState === "Running" || sessionState === "Thinking") && (
          <Tooltip title="取消本次（执行中/思考中均可中止）">
            <Button
              type="text"
              size="small"
              danger
              icon={<StopOutlined />}
              onClick={() => void onCancel()}
            />
          </Tooltip>
        )}
        <HistoryPopover
          conversations={conversations}
          activeSession={activeSession}
          onSwitchConversation={onSwitchConversation}
          onDeleteConversation={onDeleteConversation}
          onRenameConversation={onRenameConversation}
          onRefreshConversations={onRefreshConversations}
        />
        <Tooltip title="新建对话">
          <Button
            type="text"
            size="small"
            icon={<PlusOutlined />}
            onClick={() => void onNewConversation()}
          />
        </Tooltip>
        <Tooltip title="收起">
          <Button
            type="text"
            size="small"
            icon={<CompressOutlined />}
            onClick={onToggleCollapse}
          />
        </Tooltip>
      </div>
    </div>
  );
}
