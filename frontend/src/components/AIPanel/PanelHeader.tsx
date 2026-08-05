import { Button, Tag, Tooltip } from "antd";
import {
  HolderOutlined,
  MessageOutlined,
  CompressOutlined,
  PlusOutlined,
  StopOutlined,
  HistoryOutlined,
} from "@ant-design/icons";
import type { configstore } from "@wailsjs/go/models";
import type { SessionState } from "./types";

interface PanelHeaderProps {
  hostName: string;
  aiCfg: configstore.AIConfig | null;
  configured: boolean;
  cfgLoading: boolean;
  stateMeta: { text: string; color: string } | null;
  sessionState: SessionState;
  resizeHover: boolean;
  onResizeStart: (e: React.MouseEvent<HTMLDivElement>) => void;
  onResizeHoverChange: (hover: boolean) => void;
  onCancel: () => Promise<void>;
  onOpenHistory: () => void;
  onNewConversation: () => Promise<void>;
  onToggleCollapse: () => void;
}

/** 抽屉标题栏：整体作为拖拽调整高度的区域，grip 图标提示可拖动。 */
export default function PanelHeader({
  hostName,
  aiCfg,
  configured,
  cfgLoading,
  stateMeta,
  sessionState,
  resizeHover,
  onResizeStart,
  onResizeHoverChange,
  onCancel,
  onOpenHistory,
  onNewConversation,
  onToggleCollapse,
}: PanelHeaderProps): React.JSX.Element {
  return (
    <div
      onMouseDown={onResizeStart}
      onMouseEnter={() => onResizeHoverChange(true)}
      onMouseLeave={() => onResizeHoverChange(false)}
      title="拖动调整高度"
      style={{
        padding: "6px 10px",
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        borderBottom: "1px solid var(--antd-color-border-secondary)",
        flexShrink: 0,
        cursor: "ns-resize",
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
        <HolderOutlined
          style={{
            fontSize: 11,
            color: resizeHover
              ? "var(--antd-color-text)"
              : "var(--antd-color-text-quaternary)",
            transition: "color 0.2s",
          }}
        />
        <MessageOutlined style={{ color: "var(--antd-color-primary)" }} />
        <span style={{ fontSize: 12, fontWeight: 600 }}>AI 助手</span>
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
        {!configured && !cfgLoading && <Tag color="error">未配置</Tag>}
        {stateMeta && <Tag color={stateMeta.color}>{stateMeta.text}</Tag>}
      </div>
      <div
        style={{ display: "flex", gap: 4 }}
        onMouseDown={(e) => e.stopPropagation()}
      >
        {sessionState === "Running" && (
          <Tooltip title="取消执行">
            <Button
              type="text"
              size="small"
              danger
              icon={<StopOutlined />}
              onClick={() => void onCancel()}
            />
          </Tooltip>
        )}
        <Tooltip title="历史对话">
          <Button
            type="text"
            size="small"
            icon={<HistoryOutlined />}
            onClick={onOpenHistory}
          />
        </Tooltip>
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
