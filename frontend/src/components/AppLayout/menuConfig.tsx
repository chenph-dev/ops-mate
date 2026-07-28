import React, { lazy, type ComponentType } from 'react';
import type { ItemType, MenuItemType } from 'antd/es/menu/interface';
import {
  HomeOutlined,
  InfoCircleOutlined,
  SettingOutlined,
} from '@ant-design/icons';

/** 路由项定义 — 同时驱动菜单和路由 */
export interface RouteItem {
  path: string;
  label: string;
  icon?: React.ReactNode;
  component: ComponentType;
  hideInMenu?: boolean;
}

const lazyPage = (loader: () => Promise<{ default: ComponentType }>) =>
  lazy(loader);

export const routes: RouteItem[] = [
  {
    path: '/home',
    label: '首页',
    icon: <HomeOutlined />,
    component: lazyPage(() => import('@/pages/Home')),
  },
  {
    path: '/about',
    label: '关于',
    icon: <InfoCircleOutlined />,
    component: lazyPage(() => import('@/pages/About')),
  },
  {
    path: '/settings',
    label: '设置',
    icon: <SettingOutlined />,
    component: lazyPage(() => import('@/pages/Settings')),
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
