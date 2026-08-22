import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import shell from "./en-shell.json";

if (typeof document !== "undefined") {
  document.documentElement.lang = "en";
}

void i18n.use(initReactI18next).init({
  resources: {
    // Eager shell only; other namespaces merge into `translation` so existing t() keys keep working.
    en: { translation: shell },
  },
  lng: "en",
  fallbackLng: "en",
  interpolation: { escapeValue: false },
  returnNull: false,
});

const LAZY_NAMESPACES = ["jetstream", "admin", "docs"] as const;

/** Merge jetstream / admin / docs translation bundles after first paint. */
export function loadLazyTranslations(): Promise<void> {
  return Promise.all(
    LAZY_NAMESPACES.map((ns) =>
      import(`./en-${ns}.json`).then((mod) => {
        i18n.addResourceBundle("en", "translation", mod.default, true, true);
      }),
    ),
  ).then(() => undefined);
}

if (typeof window !== "undefined") {
  const schedule =
    typeof window.requestIdleCallback === "function"
      ? (cb: () => void) => window.requestIdleCallback(() => cb(), { timeout: 1500 })
      : (cb: () => void) => window.setTimeout(cb, 0);
  schedule(() => {
    void loadLazyTranslations();
  });
}

export default i18n;
