import { createContext, useContext } from "react";

interface ThemeContextValue {
  isDark: boolean;
  toggleTheme: () => void;
}

export const ThemeContext = createContext<ThemeContextValue>({
  isDark: true,
  toggleTheme: () => {},
});

export const useThemeToggle = (): ThemeContextValue => useContext(ThemeContext);
