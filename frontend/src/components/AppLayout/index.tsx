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
  WindowMaximise,
  WindowUnmaximise,
  WindowIsMaximised,
  WindowGetSize,
  Quit,
  EventsOn,
} from "@wailsjs/runtime/runtime";
import { routes } from "./menuConfig";
import logo from "@/assets/images/logo-universal.png";
import { useEffect, useState, useCallback } from "react";

const { Header, Sider, Content } = Layout;

export default function AppLayout(): React.JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const { token } = theme.useToken();
  const [isMaximised, setIsMaximised] = useState(false);
  const { isDark, toggleTheme } = useThemeToggle();
  // WebView2 中 CSS 单位（vh / 百分比）和 window.innerHeight 在窗口还原时不更新，
  // 必须通过 Wails 原生 API WindowGetSize 读取真实窗口高度
  const [winHeight, setWinHeight] = useState(() => window.innerHeight);

  const refreshWindowHeight = useCallback((): void => {
    WindowGetSize().then((size) => setWinHeight(size.h));
  }, []);

  useEffect(() => {
    // 浏览器 resize 兜底：window.innerHeight 连续更新，作为 Wails 事件的补充
    const onResize = (): void => setWinHeight(window.innerHeight);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  useEffect(() => {
    WindowIsMaximised().then(setIsMaximised);
    refreshWindowHeight();
    // 所有窗口尺寸变化统一走 Wails 原生事件，直接读事件载荷里的新尺寸（w/h），
    // 避免在事件触发瞬间调用 WindowGetSize 拿到旧值
    const offResize = EventsOn("wails:window:resized", (data) => {
      const h = (data as { h?: number } | undefined)?.h;
      if (typeof h === "number") setWinHeight(h);
      else refreshWindowHeight();
    });
    const offMax = EventsOn("wails:window:maximized", () => {
      setIsMaximised(true);
      refreshWindowHeight();
    });
    const offUnmax = EventsOn("wails:window:unmaximized", () => {
      setIsMaximised(false);
      refreshWindowHeight();
    });
    return () => {
      offResize();
      offMax();
      offUnmax();
    };
  }, [refreshWindowHeight]);

  const selectedKey = routes.find((r) =>
    location.pathname.startsWith(r.path),
  )?.path;

  const handleToggleMaximize = (): void => {
    // 基于当前状态乐观更新，不用立即查询（toggle 异步，查询会拿到旧值）
    // 事件 wails:window:maximized/unmaximized 作为外部触发的补充
    if (isMaximised) {
      WindowUnmaximise();
      setIsMaximised(false);
    } else {
      WindowMaximise();
      setIsMaximised(true);
    }
  };

  return (
    <Layout style={{ height: winHeight, overflow: "hidden" }}>
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
      <Layout hasSider style={{ flex: 1, minHeight: 0 }}>
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
        <Content style={{ padding: 16, overflow: "auto", minHeight: 0 }}>
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
}): React.JSX.Element {
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
