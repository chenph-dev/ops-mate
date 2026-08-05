import { Button, Input } from "antd";
import { SendOutlined } from "@ant-design/icons";
import type { Ref } from "react";
import type { InputRef } from "antd";

interface PanelInputProps {
  /** 输入框 ref（示例引导填充后聚焦用）。 */
  ref?: Ref<InputRef>;
  input: string;
  sending: boolean;
  inputDisabled: boolean;
  configured: boolean;
  onInputChange: (value: string) => void;
  onSend: () => Promise<void>;
  onKeyDown: (e: React.KeyboardEvent) => void;
}

/**
 * 抽屉底部输入区。
 * 发送按钮使用 antd Input 原生的 suffix 内置在输入框内部右侧，
 * 天然与输入框一体，不存在外部按钮的高度对齐问题。
 */
export default function PanelInput({
  ref,
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
        flexShrink: 0,
      }}
    >
      <Input
        ref={ref}
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
        disabled={inputDisabled}
        style={{ fontSize: 12 }}
        suffix={
          <Button
            type="primary"
            size="small"
            icon={<SendOutlined />}
            onClick={() => void onSend()}
            disabled={!input.trim() || sending || inputDisabled}
            loading={sending}
            style={{ marginRight: -8 }}
          />
        }
      />
    </div>
  );
}
