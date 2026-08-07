import { theme as antdTheme } from 'antd';
import type { ThemeConfig } from 'antd';
import type { ITheme } from '@xterm/xterm';

/**
 * 全局 antd 主题配置 —— 单一来源。
 *
 * 所有组件的默认样式都在这里集中定义，组件内部不要再手写硬编码颜色：
 * - antd 组件的默认样式 → 在下方 `token` / `components` 里配置
 * - 自定义组件（非 antd）→ 用 `theme.useToken()` 读取 token，而不是写死颜色
 *
 * 深浅色切换由 `algorithm` 驱动，antd 会自动产出两套默认 token。
 */
export function buildTheme(isDark: boolean): ThemeConfig {
  return {
    algorithm: isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
    cssVar: { prefix: 'antd' },
    hashed: false,
    token: {
      colorPrimary: '#1677ff',
      borderRadius: 6,
    },
    components: {
      // 布局：左侧 Sider + 顶栏底色
      Layout: {
        headerBg: isDark ? '#001529' : '#f0f2f5',
        siderBg: isDark ? '#001529' : '#f0f2f5',
      },
      // 弹窗：新建目录 / 删除确认等
      Modal: {
        borderRadiusLG: 8,
      },
      // 主机树
      Tree: {
        nodeHoverBg: 'rgba(22, 119, 255, 0.08)',
        indentSize: 8,
        switcherSize: 20,
        titleHeight: 20,
      },
      // 侧边栏菜单
      Menu: {
        itemBorderRadius: 6,
      },
    },
  };
}

/**
 * xterm.js 终端配色（VS Code 风格），跟随主题切换。
 * 这是终端模拟器自己的 ANSI 调色板，不属于 antd 组件，故单独抽出来统一管理。
 */
export function terminalTheme(isDark: boolean): ITheme {
  return {
    background: isDark ? '#1e1e1e' : '#ffffff',
    foreground: isDark ? '#d4d4d4' : '#333333',
    cursor: isDark ? '#d4d4d4' : '#333333',
    selectionBackground: isDark ? '#264f78' : '#add6ff',
    black: '#000000',
    red: '#cd3131',
    green: '#0dbc79',
    yellow: '#e5e510',
    blue: '#2472c8',
    magenta: '#bc3fbc',
    cyan: '#11a8cd',
    white: '#e5e5e5',
    brightBlack: '#666666',
    brightRed: '#f14c4c',
    brightGreen: '#23d18b',
    brightYellow: '#f5f543',
    brightBlue: '#3b8eea',
    brightMagenta: '#d670d6',
    brightCyan: '#29b8db',
    brightWhite: '#ffffff',
  };
}
