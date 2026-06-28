import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { STORAGE_KEYS } from "./constants";
import { loadThemeStyles } from "./themeStyles";

export type ThemePreview = {
  bg: string;
  accent: string;
  mode: "light" | "dark";
};

export const THEMES = {
  aurora: { label: "Aurora", preview: { bg: "#060912", accent: "#38bdf8", mode: "dark" } },
  dark: { label: "Dark", preview: { bg: "#0b1220", accent: "#3b82f6", mode: "dark" } },
  light: { label: "Light", preview: { bg: "#f8fafc", accent: "#2563eb", mode: "light" } },
  nord: { label: "Nord", preview: { bg: "#2e3440", accent: "#88c0d0", mode: "dark" } },
  dracula: { label: "Dracula", preview: { bg: "#282a36", accent: "#bd93f9", mode: "dark" } },
  solarized: { label: "Solarized", preview: { bg: "#002b36", accent: "#268bd2", mode: "dark" } },
  midnight: { label: "Midnight", preview: { bg: "#0f0a1a", accent: "#8b5cf6", mode: "dark" } },
  catppuccin: { label: "Catppuccin", preview: { bg: "#1e1e2e", accent: "#cba6f7", mode: "dark" } },
  latte: { label: "Latte", preview: { bg: "#eff1f5", accent: "#8839ef", mode: "light" } },
  gruvbox: { label: "Gruvbox", preview: { bg: "#282828", accent: "#fabd2f", mode: "dark" } },
  tokyo: { label: "Tokyo Night", preview: { bg: "#1a1b26", accent: "#7aa2f7", mode: "dark" } },
  onedark: { label: "One Dark", preview: { bg: "#282c34", accent: "#61afef", mode: "dark" } },
  rosepine: { label: "Rosé Pine", preview: { bg: "#191724", accent: "#eb6f92", mode: "dark" } },
  dawn: { label: "Dawn", preview: { bg: "#faf4ed", accent: "#d7827e", mode: "light" } },
  forest: { label: "Forest", preview: { bg: "#0d1f17", accent: "#34d399", mode: "dark" } },
  ocean: { label: "Ocean", preview: { bg: "#0a1628", accent: "#22d3ee", mode: "dark" } },
  graphite: { label: "Graphite", preview: { bg: "#1c1c1e", accent: "#a1a1aa", mode: "dark" } },
  monokai: { label: "Monokai", preview: { bg: "#272822", accent: "#a6e22e", mode: "dark" } },
  github: { label: "GitHub", preview: { bg: "#0d1117", accent: "#58a6ff", mode: "dark" } },
  amber: { label: "Amber", preview: { bg: "#1a1208", accent: "#f59e0b", mode: "dark" } },
  crimson: { label: "Crimson", preview: { bg: "#1a0a0e", accent: "#f43f5e", mode: "dark" } },
} as const satisfies Record<string, { label: string; preview: ThemePreview }>;

export type ThemeId = keyof typeof THEMES;

export const THEME_IDS = Object.keys(THEMES) as ThemeId[];

export const DEFAULT_THEME: ThemeId = "aurora";

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
      localStorage.setItem(THEME_STORAGE_KEY, theme);
    });
    return () => {
      active = false;
    };
  }, [theme]);

  const setTheme = useCallback((next: ThemeId) => setThemeState(next), []);

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
