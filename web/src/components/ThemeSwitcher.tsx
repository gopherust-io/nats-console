import { useEffect, useMemo, useRef, useState } from "react";
import { THEMES, THEME_IDS, useTheme, type ThemeId } from "../lib/theme";
import ThemeIcon from "./ThemeIcon";

function ThemeOption({
  id,
  active,
  onSelect,
}: {
  id: ThemeId;
  active: boolean;
  onSelect: (id: ThemeId) => void;
}) {
  const meta = THEMES[id];
  return (
    <button
      type="button"
      role="menuitemradio"
      className={`theme-menu__option${active ? " theme-menu__option--active" : ""}`}
      aria-checked={active}
      onClick={() => onSelect(id)}
    >
      <span className="theme-menu__option-icon">
        <ThemeIcon preview={meta.preview} size={18} />
      </span>
      <span className="theme-menu__option-body">
        <span className="theme-menu__option-label">{meta.label}</span>
        <span className="theme-menu__option-mode">{meta.preview.mode === "dark" ? "Dark" : "Light"}</span>
      </span>
      {active && (
        <span className="theme-menu__option-check" aria-hidden>
          ✓
        </span>
      )}
    </button>
  );
}

export default function ThemeSwitcher() {
  const { theme, setTheme } = useTheme();
  const [menuOpen, setMenuOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const rootRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  const filteredThemeIds = useMemo(() => {
    const normalizedQuery = searchQuery.trim().toLowerCase();
    if (!normalizedQuery) return THEME_IDS;
    return THEME_IDS.filter((id) => THEMES[id].label.toLowerCase().includes(normalizedQuery));
  }, [searchQuery]);

  const darkThemeIds = useMemo(
    () => filteredThemeIds.filter((id) => THEMES[id].preview.mode === "dark"),
    [filteredThemeIds],
  );
  const lightThemeIds = useMemo(
    () => filteredThemeIds.filter((id) => THEMES[id].preview.mode === "light"),
    [filteredThemeIds],
  );

  useEffect(() => {
    if (!menuOpen) return;

    const onPointerDown = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMenuOpen(false);
      }
    };

    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    searchRef.current?.focus();

    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [menuOpen]);

  function selectTheme(id: ThemeId) {
    setTheme(id);
    setMenuOpen(false);
    setSearchQuery("");
  }

  function toggleThemeMenu() {
    setMenuOpen((isOpen) => {
      if (isOpen) setSearchQuery("");
      return !isOpen;
    });
  }

  return (
    <div className={`theme-menu${menuOpen ? " theme-menu--open" : ""}`} ref={rootRef}>
      <button
        type="button"
        className="theme-menu__trigger"
        aria-haspopup="menu"
        aria-expanded={menuOpen}
        onClick={toggleThemeMenu}
      >
        <span className="theme-menu__trigger-icon">
          <ThemeIcon preview={THEMES[theme].preview} size={18} />
        </span>
        <span className="theme-menu__trigger-text">
          <span className="theme-menu__trigger-label">Theme</span>
          <span className="theme-menu__trigger-value">{THEMES[theme].label}</span>
        </span>
        <span className="theme-menu__chevron" aria-hidden />
      </button>

      {menuOpen && (
        <div className="theme-menu__panel" role="menu" aria-label="Choose theme">
          <div className="theme-menu__search-wrap">
            <input
              ref={searchRef}
              className="theme-menu__search"
              type="search"
              placeholder="Search themes…"
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              aria-label="Search themes"
            />
          </div>

          <div className="theme-menu__list">
            {filteredThemeIds.length === 0 && (
              <p className="theme-menu__empty">No themes match &ldquo;{searchQuery}&rdquo;</p>
            )}

            {darkThemeIds.length > 0 && (
              <div className="theme-menu__group">
                <div className="theme-menu__group-label">Dark</div>
                {darkThemeIds.map((id) => (
                  <ThemeOption key={id} id={id} active={theme === id} onSelect={selectTheme} />
                ))}
              </div>
            )}

            {lightThemeIds.length > 0 && (
              <div className="theme-menu__group">
                <div className="theme-menu__group-label">Light</div>
                {lightThemeIds.map((id) => (
                  <ThemeOption key={id} id={id} active={theme === id} onSelect={selectTheme} />
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
