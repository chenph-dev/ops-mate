import { useState, useMemo } from 'react';
import { RouterProvider } from 'react-router-dom';
import { ConfigProvider, theme as antdTheme, App as AntdApp } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import enUS from 'antd/locale/en_US';
import { useTranslation } from 'react-i18next';
import '@/i18n';
import { router } from '@/router';
import { ThemeContext } from '@/context/ThemeContext';
import { buildTheme } from '@/theme';

type AlgorithmFn = typeof antdTheme.darkAlgorithm;

export default function App(): React.JSX.Element {
  // 订阅语言变化：切换语言时 ConfigProvider 的 antd locale 随之刷新
  const { i18n } = useTranslation();
  const [algorithm, setAlgorithm] = useState<AlgorithmFn>(
    () => antdTheme.darkAlgorithm,
  );

  const toggleAlgorithm = (): void => {
    setAlgorithm((prev: AlgorithmFn) =>
      prev === antdTheme.darkAlgorithm
        ? antdTheme.defaultAlgorithm
        : antdTheme.darkAlgorithm,
    );
  };

  const isDark = algorithm === antdTheme.darkAlgorithm;

  const theme = useMemo(() => buildTheme(isDark), [isDark]);

  // antd 内置组件文案跟随当前语言（en 前缀才用英文，其余回落中文）
  const antdLocale = i18n.language.startsWith('en') ? enUS : zhCN;

  return (
    <ThemeContext.Provider value={{ isDark, toggleTheme: toggleAlgorithm }}>
      <ConfigProvider locale={antdLocale} theme={theme}>
        <AntdApp>
          <RouterProvider router={router} />
        </AntdApp>
      </ConfigProvider>
    </ThemeContext.Provider>
  );
}
