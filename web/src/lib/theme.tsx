import { useEffect, type ReactNode } from "react";
import { STORAGE_KEYS } from "./constants";
import { syncFavicons } from "./themeFavicon";

export type ThemePreview = {
  bg: string;
  accent: string;
  mode: "dark";
};

export const THEMES = {
  control: { label: "Console Dark", preview: { bg: "#050505", accent: "#10b981", mode: "dark" } },
} as const satisfies Record<string, { label: string; preview: ThemePreview }>;

export type ThemeId = keyof typeof THEMES;

export const THEME_IDS = Object.keys(THEMES) as ThemeId[];

export const DEFAULT_THEME: ThemeId = "control";

const THEME_STORAGE_KEY = STORAGE_KEYS.theme;

export function applyTheme(theme: ThemeId = DEFAULT_THEME) {
  document.documentElement.setAttribute("data-theme", theme);
  document.documentElement.style.colorScheme = THEMES[theme].preview.mode;
  syncFavicons(theme);
  localStorage.setItem(THEME_STORAGE_KEY, theme);
}

/** Force dark theme; migrate any legacy stored id (e.g. control-light) to control. */
export function ThemeProvider({ children }: { children: ReactNode }) {
  useEffect(() => {
    applyTheme(DEFAULT_THEME);
  }, []);

  return <>{children}</>;
}
