import React, { lazy, type ComponentType } from 'react';
import type { ItemType, MenuItemType } from 'antd/es/menu/interface';
import {
  DesktopOutlined,
  SettingOutlined,
  InfoCircleOutlined,
  FileSearchOutlined,
} from '@ant-design/icons';

/** 路由项定义 — 同时驱动菜单和路由。
 * 叶子：需 path + component；分组（聚合菜单）：需 path + children（无 component）。
 */
export interface RouteItem {
  path: string;
  label: string;
  icon?: React.ReactNode;
  component?: ComponentType; // 叶子才有；分组为空
  hideInMenu?: boolean;
  children?: RouteItem[]; // 非空 = 分组，聚合若干子菜单
}

const lazyPage = (
  loader: () => Promise<{ default: ComponentType }>,
): ComponentType => lazy(loader);

export const routes: RouteItem[] = [
  {
    path: '/hosts',
    label: '主机',
    icon: <DesktopOutlined />,
    component: lazyPage(() => import('@/pages/Hosts')),
  },
  {
    path: '/system',
    label: '系统',
    icon: <SettingOutlined />,
    children: [
      {
        path: '/config',
        label: 'LLM模型配置',
        component: lazyPage(() => import('@/pages/Config')),
      },
      {
        path: '/audit',
        label: '审计日志',
        icon: <FileSearchOutlined />,
        component: lazyPage(() => import('@/pages/Audit')),
      },
    ],
  },
  {
    path: '/about',
    label: '关于',
    icon: <InfoCircleOutlined />,
    component: lazyPage(() => import('@/pages/About')),
  },
];

/** 叶子路由项：component 必填（与分组的可选 component 区分，供路由直接用）。 */
export type LeafRouteItem = Omit<RouteItem, 'component' | 'children'> & {
  component: ComponentType;
};

/** 递归取出所有叶子路由项（分组不参与路由，只承担菜单聚合）。 */
export function leafRoutes(items: RouteItem[]): LeafRouteItem[] {
  const out: LeafRouteItem[] = [];
  for (const r of items) {
    if (r.children?.length) {
      out.push(...leafRoutes(r.children));
    } else if (r.component) {
      out.push({
        path: r.path,
        label: r.label,
        icon: r.icon,
        component: r.component,
      });
    }
  }
  return out;
}

/** RouteItem[] → antd Menu items（含子菜单，供需要时使用）。 */
export function toMenuItems(items: RouteItem[]): ItemType<MenuItemType>[] {
  return items
    .filter((r) => !r.hideInMenu)
    .map((r) =>
      r.children?.length
        ? {
            key: r.path,
            icon: r.icon,
            label: r.label,
            children: toMenuItems(r.children),
          }
        : { key: r.path, icon: r.icon, label: r.label },
    );
}
