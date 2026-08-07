import { Suspense } from 'react';
import { createHashRouter, Navigate, type RouteObject } from 'react-router-dom';
import { Spin } from 'antd';
import AppLayout from '@/components/AppLayout';
import { routes, leafRoutes } from '@/components/AppLayout/menuConfig';

// 只从叶子路由项生成 RouteObject（分组仅为菜单聚合，不参与路由）。
const childRoutes: RouteObject[] = leafRoutes(routes).map((r) => ({
  path: r.path,
  element: (
    <Suspense fallback={<Spin style={{ marginTop: 48 }} />}>
      <r.component />
    </Suspense>
  ),
}));

export const router = createHashRouter([
  {
    path: '/',
    element: <AppLayout />,
    children: [
      { index: true, element: <Navigate to="/hosts" replace /> },
      ...childRoutes,
      { path: '*', element: <div style={{ padding: 24 }}>页面不存在</div> },
    ],
  },
]);
