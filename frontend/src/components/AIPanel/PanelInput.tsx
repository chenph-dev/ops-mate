import { Button, Input } from "antd";
import { SendOutlined } from "@ant-design/icons";

interface PanelInputProps {
  input: string;
  sending: boolean;
  inputDisabled: boolean;
  configured: boolean;
  onInputChange: (value: string) => void;
  onSend: () => Promise<void>;
  onKeyDown: (e: React.KeyboardEvent) => void;
}

/** 抽屉底部输入区。 */
export default function PanelInput({
  input,
  sending,
  inputDisabled,
  configured,
  onInputChange,
  onSend,
  onKeyDown,
}: PanelInputProps): React.JSX.Element {
  return (
    <div
      style={{
        padding: "6px 8px",
        borderTop: "1px solid var(--antd-color-border-secondary)",
        display: "flex",
        gap: 6,
        alignItems: "flex-end",
        flexShrink: 0,
      }}
    >
      <Input.TextArea
        value={input}
        onChange={(e) => onInputChange(e.target.value)}
        onKeyDown={onKeyDown}
        placeholder={
          !configured
            ? "请先配置 AI 后端"
            : inputDisabled
              ? "等待本轮对话结束..."
              : "输入问题..."
        }
        autoSize={{ minRows: 1, maxRows: 3 }}
        style={{ fontSize: 12 }}
        disabled={inputDisabled}
      />
      <Button
        type="primary"
        size="small"
        icon={<SendOutlined />}
        onClick={() => void onSend()}
        disabled={!input.trim() || sending || inputDisabled}
        loading={sending}
      />
    </div>
  );
}
