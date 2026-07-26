import { THEMES, useTheme } from "../lib/theme";
import { useTranslation } from "react-i18next";

function MoonIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden>
      <path
        d="M21 14.5A8.5 8.5 0 0 1 9.5 3 7 7 0 1 0 21 14.5Z"
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

export default function ThemeSwitcher() {
  const { t } = useTranslation();
  const { theme, setTheme } = useTheme();
  const isDark = THEMES[theme].preview.mode === "dark";

  return (
    <button
      type="button"
      className="nc-icon-btn theme-toggle"
      aria-label={isDark ? t("nav.switchToLightAria") : t("nav.switchToDarkAria")}
      title={isDark ? t("nav.switchToLight") : t("nav.switchToDark")}
      onClick={() => setTheme(isDark ? "control-light" : "control")}
    >
      {isDark ? <MoonIcon /> : <SunIcon />}
    </button>
  );
}
