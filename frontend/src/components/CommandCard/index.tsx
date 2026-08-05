import { useState } from "react";
import { Button, Tag, Input, Tooltip, theme } from "antd";
import { EditOutlined } from "@ant-design/icons";
import type { ApprovalStatus, CommandSuggestion } from "@/hooks/useSessions";
import { isHighRiskCommand } from "./risk";

interface CommandCardProps {
  command: CommandSuggestion;
  /** 审批调用是否进行中 */
  busy?: boolean;
  /** 历史回放模式：只展示，无操作按钮 */
  history?: boolean;
  /** 用户审批状态：待审批 / 已批准 / 已拒绝 */
  status?: ApprovalStatus;
  onApprove?: (command: string) => void;
  onReject?: () => void;
}

const STATUS_META: Record<ApprovalStatus, { text: string; color: string }> = {
  pending: { text: "待审批", color: "processing" },
  approved: { text: "已批准", color: "success" },
  rejected: { text: "已拒绝", color: "error" },
};

/** 命令审批卡：状态+原因一行、命令、操作按钮（紧凑布局）。 */
export default function CommandCard({
  command,
  busy,
  history,
  status,
  onApprove,
  onReject,
}: CommandCardProps): React.JSX.Element {
  const { token } = theme.useToken();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(command.command);
  const [confirming, setConfirming] = useState(false);
  // 编辑时用前端镜像对当前草稿实时重估风险（即时反馈）；
  // 未编辑时以后端守卫判定为准（历史/展示模式忠实于当时判定）。
  const isHighRisk = editing
    ? isHighRiskCommand(draft)
    : command.assessedRisk === "high" || command.risk === "high";
  // 已批准/已拒绝后不再可操作（执行中或结果已定）
  const done = status === "approved" || status === "rejected";

  const handleApprove = (): void => {
    // 高风险二次确认：首次点击只进入确认态，二次点击才批准
    if (isHighRisk && !confirming) {
      setConfirming(true);
      return;
    }
    onApprove?.(draft);
  };

  return (
    <div
      style={{
        border: `1px solid ${isHighRisk ? token.colorError : token.colorBorderSecondary}`,
        borderRadius: 6,
        padding: "6px 8px",
        background: token.colorBgElevated,
        margin: "2px 0",
      }}
    >
      {/* 头部一行：审批状态 + 原因 + 风险 */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 6,
          marginBottom: 4,
        }}
      >
        {status && (
          <Tag
            color={STATUS_META[status].color}
            style={{ margin: 0, fontSize: 11, lineHeight: "16px" }}
          >
            {STATUS_META[status].text}
          </Tag>
        )}
        <span
          title={command.why}
          style={{
            flex: 1,
            minWidth: 0,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
            fontSize: 11,
            color: token.colorTextSecondary,
          }}
        >
          原因: {command.why || "—"}
        </span>
        {isHighRisk && (
          <Tag color="error" style={{ margin: 0, fontSize: 11, lineHeight: "16px" }}>
            高风险
          </Tag>
        )}
      </div>

      {/* 命令（编辑态为可编辑输入框） */}
      {editing && !history ? (
        <Input.TextArea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          autoSize={{ minRows: 1, maxRows: 3 }}
          style={{ fontFamily: "monospace", fontSize: 11 }}
        />
      ) : (
        <div
          style={{
            fontFamily: '"Cascadia Code", "Fira Code", "Consolas", monospace',
            fontSize: 11,
            background: token.colorFillSecondary,
            padding: "3px 6px",
            borderRadius: 4,
            whiteSpace: "pre-wrap",
            wordBreak: "break-all",
            color: token.colorText,
          }}
        >
          $ {draft}
        </div>
      )}

      {/* 操作按钮 */}
      {!history && !done && (
        <div
          style={{
            display: "flex",
            justifyContent: "flex-end",
            gap: 4,
            marginTop: 6,
          }}
        >
          <Tooltip title={editing ? "完成编辑" : "修改命令"}>
            <Button
              size="small"
              icon={<EditOutlined />}
              onClick={() => {
                setEditing(!editing);
                // 命令变更后旧的二次确认态作废，重新点击才能再次确认
                setConfirming(false);
              }}
              disabled={busy}
            >
              {editing ? "完成" : "编辑"}
            </Button>
          </Tooltip>
          <Button size="small" onClick={() => onReject?.()} disabled={busy}>
            拒绝
          </Button>
          <Button
            type="primary"
            size="small"
            danger={isHighRisk}
            loading={busy}
            onClick={handleApprove}
          >
            {isHighRisk && confirming ? "再次确认" : "批准"}
          </Button>
        </div>
      )}
    </div>
  );
}
