import { THEMES, THEME_IDS, useTheme, type ThemeId } from "../lib/theme";
import ThemeIcon from "./ThemeIcon";

export default function ThemeSwitcher({ compact = false }: { compact?: boolean }) {
  const { theme, setTheme } = useTheme();

  if (compact) {
    return (
      <div className="theme-switcher theme-switcher--compact">
        {THEME_IDS.map((id) => (
          <button
            key={id}
            type="button"
            className={`theme-chip${theme === id ? " active" : ""}`}
            onClick={() => setTheme(id)}
            title={THEMES[id].label}
            aria-label={`${THEMES[id].label} theme`}
            aria-pressed={theme === id}
          >
            <ThemeIcon preview={THEMES[id].preview} />
          </button>
        ))}
      </div>
    );
  }

  return (
    <div className="theme-switcher">
      <label htmlFor="theme-select" className="theme-switcher__label">
        Theme
      </label>
      <div className="theme-switcher__current">
        <ThemeIcon preview={THEMES[theme].preview} size={16} />
        <select
          id="theme-select"
          value={theme}
          onChange={(e) => setTheme(e.target.value as ThemeId)}
          className="theme-switcher__select"
        >
          {THEME_IDS.map((id) => (
            <option key={id} value={id}>
              {THEMES[id].label}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}
