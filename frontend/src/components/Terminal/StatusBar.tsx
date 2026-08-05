import { theme } from "antd";

interface StatusBarProps {
  statusText: string;
  dims: { cols: number; rows: number };
  fontSize: number;
  hostAddr: string;
}

/** 终端底部状态栏。 */
export default function StatusBar({
  statusText,
  dims,
  fontSize,
  hostAddr,
}: StatusBarProps): React.JSX.Element {
  const { token } = theme.useToken();
  return (
    <div
      style={{
        padding: "4px 10px",
        display: "flex",
        alignItems: "center",
        gap: 16,
        borderTop: `1px solid ${token.colorBorderSecondary}`,
        flexShrink: 0,
        fontSize: 11,
        color: token.colorTextTertiary,
        background: token.colorBgElevated,
      }}
    >
      <span>{statusText}</span>
      <span>
        {dims.cols}×{dims.rows}
      </span>
      <span>字号 {fontSize}</span>
      <span style={{ marginLeft: "auto" }}>{hostAddr || "未连接"}</span>
    </div>
  );
}
