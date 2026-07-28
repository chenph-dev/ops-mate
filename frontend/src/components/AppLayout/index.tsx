import { Layout, Menu, Button, theme } from 'antd';
import {
  Outlet,
  useLocation,
  useNavigate,
} from 'react-router-dom';
import {
  MinusOutlined,
  BorderOutlined,
  SwitcherOutlined,
  CloseOutlined,
} from '@ant-design/icons';
import {
  WindowMinimise,
  WindowToggleMaximise,
  WindowIsMaximised,
  Quit,
  EventsOn,
} from '@wailsjs/runtime/runtime';
import { routes, toMenuItems } from './menuConfig';
import logo from '@/assets/images/logo-universal.png';
import { useEffect, useState } from 'react';
import './index.module.css';

const { Header, Content } = Layout;

export default function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { token } = theme.useToken();
  const [isMaximised, setIsMaximised] = useState(false);

  // 同步窗口最大化状态（初始化 + 监听系统事件）
  useEffect(() => {
    WindowIsMaximised().then(setIsMaximised);
    const offMax = EventsOn('wails:window:maximized', () => setIsMaximised(true));
    const offUnmax = EventsOn('wails:window:unmaximized', () => setIsMaximised(false));
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
    // 切换后更新状态
    WindowIsMaximised().then(setIsMaximised);
  };

  return (
    <Layout style={{ height: '100vh' }}>
      <Header
        className="titlebar-drag-region"
        style={{
          background: token.colorBgContainer,
          padding: '0 8px',
          minHeight: 38,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: `1px solid ${token.colorBorderSecondary}`,
        }}
      >
        {/* 左侧：图标 + 软件名称 */}
        <div
          className="titlebar-no-drag"
          style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 120 }}
        >
          <img src={logo} alt="logo" style={{ width: 20, height: 20 }} />
          <span
            style={{
              color: token.colorText,
              fontWeight: 600,
              fontSize: 13,
              whiteSpace: 'nowrap',
            }}
          >
            ops-mate
          </span>
        </div>

        {/* 中间：水平菜单 */}
        <Menu
          className="titlebar-no-drag"
          mode="horizontal"
          selectedKeys={selectedKey ? [selectedKey] : []}
          items={toMenuItems(routes)}
          onClick={({ key }) => navigate(key)}
          style={{
            flex: 1,
            justifyContent: 'center',
            border: 'none',
            background: 'transparent',
            minWidth: 0,
          }}
        />

        {/* 右侧：窗口控制按钮 */}
        <div
          className="titlebar-no-drag"
          style={{ display: 'flex', minWidth: 132, justifyContent: 'flex-end' }}
        >
          {/* 最小化 */}
          <WindowButton
            icon={<MinusOutlined />}
            onClick={() => WindowMinimise()}
            title="最小化"
          />
          {/* 最大化 / 还原 */}
          <WindowButton
            icon={isMaximised ? <SwitcherOutlined /> : <BorderOutlined />}
            onClick={handleToggleMaximize}
            title={isMaximised ? '还原' : '最大化'}
          />
          {/* 关闭 */}
          <WindowButton
            icon={<CloseOutlined />}
            onClick={() => Quit()}
            isDanger
            title="关闭"
          />
        </div>
      </Header>

      <Content style={{ overflow: 'auto', padding: 16 }}>
        <Outlet />
      </Content>
    </Layout>
  );
}

/** 无边框窗口控制按钮 — 无背景、悬停高亮 */
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
      className={isDanger ? 'window-btn danger' : 'window-btn'}
      style={{
        width: 46,
        height: '100%',
        border: 'none',
        borderRadius: 0,
        color: 'inherit',
      }}
    />
  );
}
