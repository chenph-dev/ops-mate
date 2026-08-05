export interface ContextMenuItem {
  key: string;
  label: string;
  onClick: () => void;
}

interface TerminalContextMenuProps {
  x: number;
  y: number;
  items: ContextMenuItem[];
  onClose: () => void;
  colorBgElevated: string;
  borderRadiusLG: number;
  boxShadowSecondary: string;
  colorText: string;
}

/** 终端右键上下文菜单。 */
export default function TerminalContextMenu({
  x,
  y,
  items,
  onClose,
  colorBgElevated,
  borderRadiusLG,
  boxShadowSecondary,
  colorText,
}: TerminalContextMenuProps): React.JSX.Element {
  return (
    <div
      style={{
        position: "fixed",
        left: x,
        top: y,
        zIndex: 9999,
        background: colorBgElevated,
        borderRadius: borderRadiusLG,
        boxShadow: boxShadowSecondary,
        padding: "4px 0",
        minWidth: 140,
      }}
      onClick={onClose}
    >
      {items.map((item) => (
        <div
          key={item.key}
          style={{
            padding: "6px 16px",
            cursor: "pointer",
            fontSize: 13,
            color: colorText,
          }}
          onClick={item.onClick}
        >
          {item.label}
        </div>
      ))}
    </div>
  );
}
