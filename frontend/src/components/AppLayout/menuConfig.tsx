import React, { lazy, type ComponentType } from 'react';
import type { ItemType, MenuItemType } from 'antd/es/menu/interface';
import {
  DesktopOutlined,
  CloudServerOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';

/** 路由项定义 — 同时驱动菜单和路由 */
export interface RouteItem {
  path: string;
  label: string;
  icon?: React.ReactNode;
  component: ComponentType;
  hideInMenu?: boolean;
}

const lazyPage = (loader: () => Promise<{ default: ComponentType }>): ComponentType =>
  lazy(loader);

export const routes: RouteItem[] = [
  {
    path: '/hosts',
    label: '主机',
    icon: <DesktopOutlined />,
    component: lazyPage(() => import('@/pages/Hosts')),
  },
  {
    path: '/config',
    label: 'AI 配置',
    icon: <CloudServerOutlined />,
    component: lazyPage(() => import('@/pages/Config')),
  },
  {
    path: '/about',
    label: '关于',
    icon: <InfoCircleOutlined />,
    component: lazyPage(() => import('@/pages/About')),
  },
];

/** RouteItem[] → antd Menu items */
export function toMenuItems(items: RouteItem[]): ItemType<MenuItemType>[] {
  return items
    .filter((r) => !r.hideInMenu)
    .map((r) => ({
      key: r.path,
      icon: r.icon,
      label: r.label,
    }));
}
