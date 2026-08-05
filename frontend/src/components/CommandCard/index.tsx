import { useState } from "react";
import { Button, Tag, Divider, Input, Tooltip, theme } from "antd";
import { WarningOutlined, EditOutlined } from "@ant-design/icons";
import type { CommandSuggestion } from "@/hooks/useSessions";

interface CommandCardProps {
  command: CommandSuggestion;
  /** 审批调用是否进行中 */
  busy?: boolean;
  /** 历史回放模式：只展示，无操作按钮 */
  history?: boolean;
  onApprove?: (command: string) => void;
  onReject?: () => void;
}

export default function CommandCard({
  command,
  busy,
  history,
  onApprove,
  onReject,
}: CommandCardProps): React.JSX.Element {
  const { token } = theme.useToken();
  const isHighRisk =
    command.assessedRisk === "high" || command.risk === "high";
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

      {!history && (
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
