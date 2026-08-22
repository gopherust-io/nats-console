/** Cache-bust when favicon assets are replaced. Keep in sync with index.html + theme-boot.js. */
const FAVICON_REV = "20260822b";

type FaviconSlot = "16" | "32" | "any" | "apple" | "alternate";

const FAVICON_HREFS = {
  control: {
    "16": `/favicon-16x16.png?v=${FAVICON_REV}`,
    "32": `/favicon-32x32.png?v=${FAVICON_REV}`,
    any: `/favicon.png?v=${FAVICON_REV}`,
    apple: `/apple-touch-icon.png?v=${FAVICON_REV}`,
    alternate: `/favicon.png?v=${FAVICON_REV}`,
  },
} as const satisfies Record<string, Record<FaviconSlot, string>>;

/** Ensure browser-tab icons stay on the dark (control) set. */
export function syncFavicons(theme: keyof typeof FAVICON_HREFS = "control") {
  if (typeof document === "undefined") return;
  const hrefs = FAVICON_HREFS[theme];
  for (const link of document.querySelectorAll<HTMLLinkElement>("link[data-nc-favicon]")) {
    const slot = link.dataset.ncFavicon as FaviconSlot | undefined;
    if (!slot || !(slot in hrefs)) continue;
    const next = hrefs[slot];
    if (link.getAttribute("href") !== next) {
      link.setAttribute("href", next);
    }
  }
}
