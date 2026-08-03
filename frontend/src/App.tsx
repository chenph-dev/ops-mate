import { useState, useMemo } from 'react';
import { RouterProvider } from 'react-router-dom';
import { ConfigProvider, theme as antdTheme, App as AntdApp } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { router } from '@/router';
import { ThemeContext } from '@/context/ThemeContext';
import { buildTheme } from '@/theme';

type AlgorithmFn = typeof antdTheme.darkAlgorithm;

export default function App() {
  const [algorithm, setAlgorithm] = useState<AlgorithmFn>(() => antdTheme.darkAlgorithm);

  const toggleAlgorithm = () => {
    setAlgorithm((prev: AlgorithmFn) =>
      prev === antdTheme.darkAlgorithm
        ? antdTheme.defaultAlgorithm
        : antdTheme.darkAlgorithm,
    );
  };

  const isDark = algorithm === antdTheme.darkAlgorithm;

  const theme = useMemo(() => buildTheme(isDark), [isDark]);

  return (
    <ThemeContext.Provider value={{ isDark, toggleTheme: toggleAlgorithm }}>
      <ConfigProvider locale={zhCN} theme={theme}>
        <AntdApp>
          <RouterProvider router={router} />
        </AntdApp>
      </ConfigProvider>
    </ThemeContext.Provider>
  );
}
