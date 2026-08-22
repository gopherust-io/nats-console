import { render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { STORAGE_KEYS } from "./constants";
import { applyTheme, DEFAULT_THEME, ThemeProvider, THEME_IDS, THEMES } from "./theme";

function installFaviconLinks() {
  for (const el of document.querySelectorAll("link[data-nc-favicon]")) {
    el.remove();
  }
  for (const [slot, href] of [
    ["16", "/favicon-light-16x16.png?v=test"],
    ["32", "/favicon-light-32x32.png?v=test"],
    ["any", "/favicon-light.png?v=test"],
  ] as const) {
    const link = document.createElement("link");
    link.rel = "icon";
    link.dataset.ncFavicon = slot;
    link.href = href;
    document.head.appendChild(link);
  }
}

describe("theme persistence", () => {
  beforeEach(() => {
    localStorage.clear();
    installFaviconLinks();
  });

  it("exposes only console dark theme id", () => {
    expect(THEME_IDS).toEqual(["control"]);
    expect(DEFAULT_THEME).toBe("control");
    expect(THEMES.control.preview.mode).toBe("dark");
  });

  it("applies control on mount and migrates legacy light storage", async () => {
    localStorage.setItem(STORAGE_KEYS.theme, "control-light");
    render(
      <ThemeProvider>
        <div data-testid="child" />
      </ThemeProvider>,
    );
    await waitFor(() => {
      expect(document.documentElement.getAttribute("data-theme")).toBe("control");
    });
    expect(localStorage.getItem(STORAGE_KEYS.theme)).toBe("control");
    expect(document.documentElement.style.colorScheme).toBe("dark");
  });

  it("falls back to control for unknown stored ids", async () => {
    localStorage.setItem(STORAGE_KEYS.theme, "aurora");
    render(
      <ThemeProvider>
        <div />
      </ThemeProvider>,
    );
    await waitFor(() => {
      expect(document.documentElement.getAttribute("data-theme")).toBe("control");
    });
    expect(localStorage.getItem(STORAGE_KEYS.theme)).toBe("control");
  });

  it("syncs dark favicon hrefs via applyTheme", () => {
    applyTheme("control");
    expect(document.querySelector('link[data-nc-favicon="32"]')?.getAttribute("href")).toContain(
      "favicon-32x32.png",
    );
    expect(document.querySelector('link[data-nc-favicon="16"]')?.getAttribute("href")).toContain(
      "favicon-16x16.png",
    );
    expect(document.querySelector('link[data-nc-favicon="any"]')?.getAttribute("href")).toContain(
      "favicon.png",
    );
  });
});
