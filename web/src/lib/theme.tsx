import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { STORAGE_KEYS } from "./constants";
import { loadThemeStyles } from "./themeStyles";

export type ThemePreview = {
  bg: string;
  accent: string;
  mode: "light" | "dark";
};

export const THEMES = {
  control: { label: "Console Dark", preview: { bg: "#101013", accent: "#10b981", mode: "dark" } },
  "control-light": { label: "Console Light", preview: { bg: "#fafafa", accent: "#10b981", mode: "light" } },
} as const satisfies Record<string, { label: string; preview: ThemePreview }>;

export type ThemeId = keyof typeof THEMES;

export const THEME_IDS = Object.keys(THEMES) as ThemeId[];

export const DEFAULT_THEME: ThemeId = "control";

const THEME_STORAGE_KEY = STORAGE_KEYS.theme;

type ThemeContextValue = {
  theme: ThemeId;
  setTheme: (theme: ThemeId) => void;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);

function readStoredTheme(): ThemeId {
  const stored = localStorage.getItem(THEME_STORAGE_KEY);
  if (stored && stored in THEMES) {
    return stored as ThemeId;
  }
  return DEFAULT_THEME;
}

export function applyTheme(theme: ThemeId) {
  document.documentElement.setAttribute("data-theme", theme);
  document.documentElement.style.colorScheme = THEMES[theme].preview.mode;
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ThemeId>(() => {
    const stored = readStoredTheme();
    applyTheme(stored);
    return stored;
  });

  useEffect(() => {
    let active = true;
    void loadThemeStyles(theme).then(() => {
      if (!active) return;
      applyTheme(theme);
    });
    return () => {
      active = false;
    };
  }, [theme]);

  const setTheme = useCallback((next: ThemeId) => {
    if (!(next in THEMES)) return;
    localStorage.setItem(THEME_STORAGE_KEY, next);
    applyTheme(next);
    setThemeState(next);
  }, []);

  const value = useMemo(() => ({ theme, setTheme }), [theme, setTheme]);

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const context = useContext(ThemeContext);
  if (!context) {
    throw new Error("useTheme must be used within ThemeProvider");
  }
  return context;
}
