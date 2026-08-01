const themeModules = import.meta.glob("../styles/themes/*.css");
const loadedThemes = new Set<string>();

export async function loadThemeStyles(theme: string) {
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

/** Load the given theme stylesheets up front (avoids first-toggle CSS parse hitch). */
export async function preloadThemeStyles(themes: readonly string[]) {
  await Promise.all(themes.map((id) => loadThemeStyles(id)));
}
