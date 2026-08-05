import { useState } from "react";
import { Button, Tag, Divider, Input, Tooltip, theme } from "antd";
import { WarningOutlined, EditOutlined } from "@ant-design/icons";
import type { ApprovalStatus, CommandSuggestion } from "@/hooks/useSessions";

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

export default function CommandCard({
  command,
  busy,
  history,
  status,
  onApprove,
  onReject,
}: CommandCardProps): React.JSX.Element {
  const { token } = theme.useToken();
  const isHighRisk =
    command.assessedRisk === "high" || command.risk === "high";
  // 已批准/已拒绝后不再可操作（执行中或结果已定）
  const done = status === "approved" || status === "rejected";
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(command.command);
  const [confirming, setConfirming] = useState(false);

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
        border: `1px solid ${isHighRisk ? token.colorError : token.colorWarning}`,
        borderRadius: 8,
        padding: 12,
        background: token.colorBgElevated,
        margin: "8px 0",
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 6,
          marginBottom: 8,
        }}
      >
        <WarningOutlined
          style={{ color: isHighRisk ? token.colorError : token.colorWarning }}
        />
        <span style={{ fontWeight: 600, fontSize: 13 }}>
          {isHighRisk ? "⚠️ AI 提议执行高风险命令" : "AI 提议执行命令"}
        </span>
        {status && (
          <Tag
            color={STATUS_META[status].color}
            style={{ marginLeft: "auto" }}
          >
            {STATUS_META[status].text}
          </Tag>
        )}
      </div>

      {editing && !history ? (
        <Input.TextArea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          autoSize={{ minRows: 1, maxRows: 4 }}
          style={{ fontFamily: "monospace", fontSize: 12, marginBottom: 8 }}
        />
      ) : (
        <div
          style={{
            fontFamily: '"Cascadia Code", "Fira Code", "Consolas", monospace',
            fontSize: 12,
            background: token.colorFillSecondary,
            padding: "8px 10px",
            borderRadius: 4,
            marginBottom: 8,
            whiteSpace: "pre-wrap",
            wordBreak: "break-all",
          }}
        >
          $ {draft}
        </div>
      )}

      <div style={{ fontSize: 12, color: token.colorTextSecondary }}>
        <div>原因: {command.why || "-"}</div>
        <div style={{ marginTop: 4 }}>
          风险:{" "}
          <Tag color={isHighRisk ? "red" : "orange"}>
            {isHighRisk ? "高" : command.risk || "低"}
          </Tag>
        </div>
      </div>

      {!history && !done && (
        <>
          <Divider style={{ margin: "8px 0" }} />
          <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
            <Tooltip title={editing ? "完成编辑" : "修改命令"}>
              <Button
                size="small"
                icon={<EditOutlined />}
                onClick={() => setEditing(!editing)}
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
              {isHighRisk && confirming ? "再次点击确认执行" : "批准执行"}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
