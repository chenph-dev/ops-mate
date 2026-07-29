import { createContext, useContext } from "react";
import type { ThemeConfig } from "antd";

interface ThemeContextValue {
  isDark: boolean;
  toggleTheme: () => void;
}

export const ThemeContext = createContext<ThemeContextValue>({
  isDark: true,
  toggleTheme: () => {},
});

export const useThemeToggle = () => useContext(ThemeContext);
