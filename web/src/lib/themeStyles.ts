import { ThemeId } from "./theme";

const themeModules = import.meta.glob("../styles/themes/*.css");
const loadedThemes = new Set<string>();

export async function loadThemeStyles(theme: ThemeId) {
  if (loadedThemes.has(theme)) {
    return;
  }
  const path = `../styles/themes/${theme}.css`;
  const loader = themeModules[path];
  if (loader) {
    await loader();
    loadedThemes.add(theme);
  }
}
