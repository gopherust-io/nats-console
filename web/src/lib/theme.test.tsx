import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { STORAGE_KEYS } from "./constants";
import { DEFAULT_THEME, ThemeProvider, THEME_IDS, THEMES, useTheme } from "./theme";

vi.mock("./themeStyles", () => ({
  loadThemeStyles: vi.fn(async () => undefined),
}));

function ThemeProbe() {
  const { theme, setTheme } = useTheme();
  return (
    <div>
      <span data-testid="theme">{theme}</span>
      <button type="button" onClick={() => setTheme("control-light")}>
        light
      </button>
      <button type="button" onClick={() => setTheme("control")}>
        dark
      </button>
      <button type="button" onClick={() => setTheme("aurora" as never)}>
        invalid
      </button>
    </div>
  );
}

describe("theme persistence", () => {
  it("exposes only console dark and light theme ids", () => {
    expect(THEME_IDS).toEqual(["control", "control-light"]);
    expect(DEFAULT_THEME).toBe("control");
    expect(THEMES.control.preview.mode).toBe("dark");
    expect(THEMES["control-light"].preview.mode).toBe("light");
  });

  it("writes the chosen theme to localStorage immediately", async () => {
    const user = userEvent.setup();
    render(
      <ThemeProvider>
        <ThemeProbe />
      </ThemeProvider>,
    );

    await user.click(screen.getByRole("button", { name: "light" }));
    expect(localStorage.getItem(STORAGE_KEYS.theme)).toBe("control-light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("control-light");
    expect(screen.getByTestId("theme")).toHaveTextContent("control-light");
  });

  it("restores a stored theme on mount", () => {
    localStorage.setItem(STORAGE_KEYS.theme, "control-light");
    render(
      <ThemeProvider>
        <ThemeProbe />
      </ThemeProvider>,
    );
    expect(screen.getByTestId("theme")).toHaveTextContent("control-light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("control-light");
  });

  it("falls back to control for unknown stored ids", () => {
    localStorage.setItem(STORAGE_KEYS.theme, "aurora");
    render(
      <ThemeProvider>
        <ThemeProbe />
      </ThemeProvider>,
    );
    expect(screen.getByTestId("theme")).toHaveTextContent("control");
    expect(document.documentElement.getAttribute("data-theme")).toBe("control");
  });

  it("ignores invalid setTheme values", async () => {
    const user = userEvent.setup();
    render(
      <ThemeProvider>
        <ThemeProbe />
      </ThemeProvider>,
    );
    await user.click(screen.getByRole("button", { name: "invalid" }));
    await waitFor(() => {
      expect(screen.getByTestId("theme")).toHaveTextContent("control");
    });
    expect(localStorage.getItem(STORAGE_KEYS.theme)).not.toBe("aurora");
  });
});
