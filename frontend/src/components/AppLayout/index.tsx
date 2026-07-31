import { Button, Tooltip, theme } from "antd";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import {
  MinusOutlined,
  BorderOutlined,
  SwitcherOutlined,
  CloseOutlined,
  SunOutlined,
  MoonOutlined,
} from "@ant-design/icons";
import { useThemeToggle } from "@/context/ThemeContext";
import {
  WindowMinimise,
  WindowToggleMaximise,
  WindowIsMaximised,
  Quit,
  EventsOn,
} from "@wailsjs/runtime/runtime";
import { routes } from "./menuConfig";
import logo from "@/assets/images/logo-universal.png";
import { useEffect, useState } from "react";

export default function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { token } = theme.useToken();
  const [isMaximised, setIsMaximised] = useState(false);
  const { isDark, toggleTheme } = useThemeToggle();

  useEffect(() => {
    WindowIsMaximised().then(setIsMaximised);
    const offMax = EventsOn("wails:window:maximized", () =>
      setIsMaximised(true),
    );
    const offUnmax = EventsOn("wails:window:unmaximized", () =>
      setIsMaximised(false),
    );
    return () => {
      offMax();
      offUnmax();
    };
  }, []);

  const selectedKey = routes.find((r) =>
    location.pathname.startsWith(r.path),
  )?.path;

  const handleToggleMaximize = () => {
    WindowToggleMaximise();
    WindowIsMaximised().then(setIsMaximised);
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100vh" }}>
      {/* 顶部栏：Logo + 主题切换 + 窗口控制 */}
      <div
        className="titlebar-drag-region"
        style={{
          background: token.colorBgContainer,
          padding: "0 8px",
          height: 38,
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          borderBottom: `1px solid ${token.colorBorderSecondary}`,
        }}
      >
        {/* 左侧：Logo + 名称（独立组件） */}
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <img src={logo} alt="logo" style={{ width: 22, height: 22 }} />
          <span
            style={{
              color: token.colorText,
              fontSize: 13,
              fontWeight: 600,
              whiteSpace: "nowrap",
            }}
          >
            ops-mate
          </span>
        </div>

        {/* 右侧：主题切换 + 窗口控制 */}
        <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
          <WindowButton
            icon={isDark ? <SunOutlined /> : <MoonOutlined />}
            onClick={toggleTheme}
            title={isDark ? "切换浅色" : "切换深色"}
            color={token.colorText}
          />
          <WindowButton
            icon={<MinusOutlined />}
            onClick={() => WindowMinimise()}
            title="最小化"
            color={token.colorText}
          />
          <WindowButton
            icon={isMaximised ? <SwitcherOutlined /> : <BorderOutlined />}
            onClick={handleToggleMaximize}
            title={isMaximised ? "还原" : "最大化"}
            color={token.colorText}
          />
          <WindowButton
            icon={<CloseOutlined />}
            onClick={() => Quit()}
            isDanger
            title="关闭"
            color={token.colorText}
          />
        </div>
      </div>

      {/* 下方：左侧菜单条 + 主内容 */}
      <div style={{ display: "flex", flex: 1, overflow: "hidden" }}>
        {/* 左侧图标菜单条 — 独立 28px */}
        <div
          style={{
            width: 44,
            background: token.colorBgContainer,
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            paddingTop: 4,
            borderRight: `1px solid ${token.colorBorderSecondary}`,
          }}
        >
          {routes.map((r) => (
            <Tooltip key={r.path} placement="right" title={r.label}>
              <Button
                type="text"
                size="small"
                icon={
                  <span style={{ fontSize: 18, color: token.colorText }}>
                    {r.icon}
                  </span>
                }
                onClick={() => navigate(r.path)}
                className={
                  selectedKey === r.path
                    ? "sidebar-menu-item active"
                    : "sidebar-menu-item"
                }
                style={{
                  width: 34,
                  height: 34,
                  border: "none",
                  borderRadius: 4,
                  borderLeft:
                    selectedKey === r.path
                      ? `3px solid ${token.colorPrimary}`
                      : "3px solid transparent",
                  color: token.colorText,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  marginBottom: 6,
                  paddingLeft: 0,
                  paddingRight: 0,
                }}
              />
            </Tooltip>
          ))}
        </div>

        {/* 主内容区 */}
        <div style={{ flex: 1, overflow: "auto", padding: 16 }}>
          <Outlet />
        </div>
      </div>
    </div>
  );
}

function WindowButton({
  icon,
  onClick,
  isDanger,
  title,
  color,
}: {
  icon: React.ReactNode;
  onClick: () => void;
  isDanger?: boolean;
  title?: string;
  color: string;
}) {
  return (
    <Button
      type="text"
      size="small"
      icon={icon}
      title={title}
      onClick={onClick}
      className={isDanger ? "window-btn danger" : "window-btn"}
      style={{
        width: 28,
        height: 28,
        border: "none",
        borderRadius: 0,
        color,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
      }}
    />
  );
}
