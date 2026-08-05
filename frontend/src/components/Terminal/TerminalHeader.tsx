import { Button, Tooltip, theme } from "antd";
import {
  ClearOutlined,
  CopyOutlined,
  DisconnectOutlined,
  MessageOutlined,
  SearchOutlined,
  ZoomInOutlined,
  ZoomOutOutlined,
} from "@ant-design/icons";

interface TerminalHeaderProps {
  hostName: string;
  hostAddr: string;
  statusDot: string;
  statusText: string;
  connected: boolean;
  fontSize: number;
  maxFontSize: number;
  minFontSize: number;
  aiOpen: boolean;
  onToggleAI: () => void;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onSearch: () => void;
  onClear: () => void;
  onCopy: () => Promise<void>;
  onDisconnect: () => void;
}

/** 终端标题栏：主机信息 + AI 面板开关/缩放/搜索/清空/复制/断开操作。 */
export default function TerminalHeader({
  hostName,
  hostAddr,
  statusDot,
  statusText,
  connected,
  fontSize,
  maxFontSize,
  minFontSize,
  aiOpen,
  onToggleAI,
  onZoomIn,
  onZoomOut,
  onSearch,
  onClear,
  onCopy,
  onDisconnect,
}: TerminalHeaderProps): React.JSX.Element {
  const { token } = theme.useToken();
  return (
    <div
      style={{
        padding: "4px 10px",
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        borderBottom: `1px solid ${token.colorBorderSecondary}`,
        flexShrink: 0,
        background: token.colorBgElevated,
      }}
    >
      <div
        style={{ display: "flex", alignItems: "center", gap: 8, minWidth: 0 }}
      >
        <span
          style={{
            fontSize: 11,
            color: token.colorTextSecondary,
            whiteSpace: "nowrap",
          }}
        >
          终端
        </span>
        <span style={{ fontSize: 12, fontWeight: 600, whiteSpace: "nowrap" }}>
          {hostName || "未选择主机"}
        </span>
        <span
          style={{
            fontSize: 11,
            color: connected ? token.colorSuccess : token.colorTextSecondary,
            whiteSpace: "nowrap",
          }}
        >
          {statusDot} {statusText}
        </span>
        {hostAddr && (
          <Tooltip title={hostAddr}>
            <span
              style={{
                fontSize: 11,
                color: token.colorTextTertiary,
                whiteSpace: "nowrap",
                overflow: "hidden",
                textOverflow: "ellipsis",
              }}
            >
              {hostAddr}
            </span>
          </Tooltip>
        )}
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: 2 }}>
        <Tooltip title={aiOpen ? "收起 AI 面板" : "打开 AI 面板"}>
          <Button
            type={aiOpen ? "primary" : "text"}
            size="small"
            icon={<MessageOutlined />}
            onClick={onToggleAI}
          />
        </Tooltip>
        <Tooltip title="放大 (Ctrl++)">
          <Button
            type="text"
            size="small"
            icon={<ZoomInOutlined />}
            onClick={onZoomIn}
            disabled={fontSize >= maxFontSize}
          />
        </Tooltip>
        <Tooltip title="缩小 (Ctrl+-)">
          <Button
            type="text"
            size="small"
            icon={<ZoomOutOutlined />}
            onClick={onZoomOut}
            disabled={fontSize <= minFontSize}
          />
        </Tooltip>
        <Tooltip title="搜索 (Ctrl+F)">
          <Button type="text" size="small" icon={<SearchOutlined />} onClick={onSearch} />
        </Tooltip>
        <Tooltip title="清空">
          <Button type="text" size="small" icon={<ClearOutlined />} onClick={onClear} />
        </Tooltip>
        <Tooltip title="复制选中内容">
          <Button type="text" size="small" icon={<CopyOutlined />} onClick={() => void onCopy()} />
        </Tooltip>
        {connected && (
          <Tooltip title="断开连接">
            <Button
              type="text"
              size="small"
              danger
              icon={<DisconnectOutlined />}
              onClick={onDisconnect}
            />
          </Tooltip>
        )}
      </div>
    </div>
  );
}
