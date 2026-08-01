import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { STORAGE_KEYS } from "./constants";
import { loadThemeStyles, preloadThemeStyles } from "./themeStyles";
import { runThemeViewTransition, warmViewTransitionPipeline } from "./themeTransition";

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
  setTheme: (theme: ThemeId, sourceEl?: HTMLElement | null) => void;
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

/**
 * Touch theme custom properties on an offscreen node so CSSOM caches warm
 * without mutating live <html data-theme> (that caused flash → snap → wipe).
 */
function warmThemePaint(theme: ThemeId) {
  if (typeof document === "undefined") return;
  const probe = document.createElement("div");
  probe.setAttribute("data-theme", theme);
  probe.setAttribute("aria-hidden", "true");
  probe.style.cssText =
    "position:fixed;left:0;top:0;width:0;height:0;overflow:hidden;opacity:0;pointer-events:none;";
  document.body.appendChild(probe);
  const cs = getComputedStyle(probe);
  void cs.getPropertyValue("--bg-body");
  void cs.getPropertyValue("--bg-card");
  void cs.getPropertyValue("--bg-sidebar");
  void cs.getPropertyValue("--text-primary");
  void cs.getPropertyValue("--border");
  probe.remove();
}

function warmAllThemePaints() {
  for (const id of THEME_IDS) {
    warmThemePaint(id);
  }
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ThemeId>(() => {
    const stored = readStoredTheme();
    applyTheme(stored);
    return stored;
  });
  const themeRef = useRef(theme);
  themeRef.current = theme;

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

  // Eager warmup right after mount — do not wait for idle (first toggles were hitching).
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      await preloadThemeStyles(THEME_IDS);
      if (cancelled) return;
      warmAllThemePaints();
      await warmViewTransitionPipeline();
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const setTheme = useCallback((next: ThemeId, sourceEl?: HTMLElement | null) => {
    if (!(next in THEMES) || next === themeRef.current) return;
    void (async () => {
      await loadThemeStyles(next);
      // Build style caches for the target theme before the mid-animation swap.
      warmThemePaint(next);
      const previous = themeRef.current;
      themeRef.current = next;

      const ran = await runThemeViewTransition(() => {
        // CSS theme only here — avoid flushSync React render in the VT gap.
        localStorage.setItem(THEME_STORAGE_KEY, next);
        applyTheme(next);
      }, sourceEl);

      if (!ran) {
        themeRef.current = previous;
        return;
      }
      setThemeState(next);
    })();
  }, []);

  useEffect(() => {
    function isEditableTarget(target: EventTarget | null): boolean {
      if (!(target instanceof HTMLElement)) return false;
      const tag = target.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
      if (target.isContentEditable) return true;
      return Boolean(target.closest("[contenteditable='true']"));
    }

    function onKeyDown(event: KeyboardEvent) {
      if (!(event.metaKey || event.ctrlKey) || !event.shiftKey) return;
      if (event.key.toLowerCase() !== "d") return;
      if (isEditableTarget(event.target)) return;
      event.preventDefault();
      const current = themeRef.current;
      const next: ThemeId = THEMES[current].preview.mode === "dark" ? "control-light" : "control";
      setTheme(next, document.querySelector(".theme-toggle"));
    }

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [setTheme]);

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
