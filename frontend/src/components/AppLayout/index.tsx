import { Button, Tooltip, Layout, theme } from "antd";
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

const { Header, Sider, Content } = Layout;

export default function AppLayout(): React.JSX.Element {
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
    <Layout style={{ height: "100vh" }}>
      {/* 顶部栏 */}
      <Header
        className="titlebar-drag-region"
        style={{
          padding: "0 8px",
          height: 38,
          lineHeight: "38px",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        {/* 左侧：Logo + 名称 */}
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <img src={logo} alt="logo" style={{ width: 22, height: 22 }} />
          <span style={{ fontSize: 13, fontWeight: 600, whiteSpace: "nowrap" }}>
            ops-mate
          </span>
        </div>

        {/* 右侧：主题切换 + 窗口控制 */}
        <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
          <WindowButton
            icon={isDark ? <SunOutlined /> : <MoonOutlined />}
            onClick={toggleTheme}
            title={isDark ? "切换浅色" : "切换深色"}
          />
          <WindowButton
            icon={<MinusOutlined />}
            onClick={() => WindowMinimise()}
            title="最小化"
          />
          <WindowButton
            icon={isMaximised ? <SwitcherOutlined /> : <BorderOutlined />}
            onClick={handleToggleMaximize}
            title={isMaximised ? "还原" : "最大化"}
          />
          <WindowButton
            icon={<CloseOutlined />}
            onClick={() => Quit()}
            isDanger
            title="关闭"
          />
        </div>
      </Header>

      {/* 下方：左侧菜单条 + 主内容 */}
      <Layout hasSider>
        {/* 左侧图标菜单条 */}
        <Sider width={44} theme={isDark ? "dark" : "light"}>
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              paddingTop: 4,
              height: "100%",
            }}
          >
            {routes.map((r) => (
              <Tooltip key={r.path} placement="right" title={r.label}>
                <Button
                  type="text"
                  size="small"
                  icon={<span style={{ fontSize: 18 }}>{r.icon}</span>}
                  onClick={() => navigate(r.path)}
                  style={{
                    width: 34,
                    height: 34,
                    border: "none",
                    borderRadius: 4,
                    borderLeft:
                      selectedKey === r.path
                        ? `3px solid ${token.colorPrimary}`
                        : "3px solid transparent",
                    marginBottom: 6,
                  }}
                />
              </Tooltip>
            ))}
          </div>
        </Sider>

        {/* 主内容区 */}
        <Content style={{ padding: 16, overflow: "auto" }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}

function WindowButton({
  icon,
  onClick,
  isDanger,
  title,
}: {
  icon: React.ReactNode;
  onClick: () => void;
  isDanger?: boolean;
  title?: string;
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
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
      }}
    />
  );
}
