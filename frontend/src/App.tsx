import { useState } from 'react';
import { RouterProvider } from 'react-router-dom';
import { ConfigProvider, theme as antdTheme } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { router } from '@/router';
import type { ThemeConfig } from 'antd';
import { ThemeContext } from '@/context/ThemeContext';

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

  const theme: ThemeConfig = {
    algorithm,
    token: { colorPrimary: '#1677ff' },
  };

  return (
    <ThemeContext.Provider
      value={{ isDark: algorithm === antdTheme.darkAlgorithm, toggleTheme: toggleAlgorithm }}
    >
      <ConfigProvider locale={zhCN} theme={theme}>
        <RouterProvider router={router} />
      </ConfigProvider>
    </ThemeContext.Provider>
  );
}
