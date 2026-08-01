import { useTranslation } from "react-i18next";
import { THEMES, useTheme } from "../lib/theme";

function MoonIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden>
      <path
        d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function SunIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" strokeLinecap="round" />
    </svg>
  );
}

function themeShortcutHint(): string {
  const mod = typeof navigator !== "undefined" && /Mac|iPhone|iPad|iPod/.test(navigator.platform) ? "⌘" : "Ctrl";
  return `${mod}+Shift+D`;
}

export default function ThemeSwitcher() {
  const { t } = useTranslation();
  const { theme, setTheme } = useTheme();
  const isDark = THEMES[theme].preview.mode === "dark";
  const shortcut = themeShortcutHint();
  const label = isDark ? t("nav.switchToLightAria") : t("nav.switchToDarkAria");
  const title = isDark
    ? t("nav.switchToLightShortcut", { shortcut })
    : t("nav.switchToDarkShortcut", { shortcut });

  return (
    <button
      type="button"
      className="nc-icon-btn theme-toggle"
      aria-label={`${label} (${shortcut})`}
      title={title}
      onClick={(event) => {
        setTheme(isDark ? "control-light" : "control", event.currentTarget);
      }}
    >
      {isDark ? <MoonIcon /> : <SunIcon />}
    </button>
  );
}
