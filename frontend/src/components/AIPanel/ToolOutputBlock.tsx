import { useState } from "react";

/** 命令执行输出块：默认折叠成一行标题，点击展开查看完整输出。 */
export default function ToolOutputBlock({
  content,
}: {
  content: string;
}): React.JSX.Element {
  const [open, setOpen] = useState(false);
  const lineCount = content.split("\n").filter((l) => l.trim() !== "").length;
  return (
    <div style={{ marginBottom: 6 }}>
      <div
        onClick={() => setOpen(!open)}
        title={open ? "收起输出" : "展开输出"}
        style={{
          fontFamily: '"Cascadia Code", "Fira Code", "Consolas", monospace',
          fontSize: 11,
          background: "rgba(0,0,0,0.25)",
          border: "1px solid var(--antd-color-border-secondary)",
          borderRadius: 4,
          padding: "2px 8px",
          cursor: "pointer",
          color: "var(--antd-color-text-secondary)",
          userSelect: "none",
          whiteSpace: "nowrap",
          overflow: "hidden",
          textOverflow: "ellipsis",
        }}
      >
        {open ? "▾" : "▸"} 命令输出（{lineCount} 行）
      </div>
      {open && (
        <div
          style={{
            fontFamily: '"Cascadia Code", "Fira Code", "Consolas", monospace',
            fontSize: 11,
            background: "rgba(0,0,0,0.25)",
            border: "1px solid var(--antd-color-border-secondary)",
            borderTop: "none",
            borderRadius: "0 0 4px 4px",
            padding: "6px 8px",
            whiteSpace: "pre-wrap",
            wordBreak: "break-all",
            color: "var(--antd-color-text)",
            maxHeight: 240,
            overflow: "auto",
          }}
        >
          {content}
        </div>
      )}
    </div>
  );
}
