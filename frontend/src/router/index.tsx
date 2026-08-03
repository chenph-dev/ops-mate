import { Suspense } from "react";
import { createHashRouter, Navigate, type RouteObject } from "react-router-dom";
import { Spin } from "antd";
import AppLayout from "@/components/AppLayout";
import { routes } from "@/components/AppLayout/menuConfig";

const childRoutes: RouteObject[] = routes.map((r) => ({
  path: r.path,
  element: (
    <Suspense fallback={<Spin style={{ marginTop: 48 }} />}>
      <r.component />
    </Suspense>
  ),
}));

export const router = createHashRouter([
  {
    path: "/",
    element: <AppLayout />,
    children: [
      { index: true, element: <Navigate to="/hosts" replace /> },
      ...childRoutes,
      { path: "*", element: <div style={{ padding: 24 }}>页面不存在</div> },
    ],
  },
]);
