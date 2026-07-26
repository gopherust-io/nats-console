import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import ThemeSwitcher from "./ThemeSwitcher";
import { STORAGE_KEYS } from "../lib/constants";
import { ThemeProvider } from "../lib/theme";

vi.mock("../lib/themeStyles", () => ({
  loadThemeStyles: vi.fn(async () => undefined),
}));

describe("ThemeSwitcher", () => {
  it("shows moon affordance in dark mode and toggles to light", async () => {
    const user = userEvent.setup();
    render(
      <ThemeProvider>
        <ThemeSwitcher />
      </ThemeProvider>,
    );

    const toggle = screen.getByRole("button", { name: "Switch to Console Light" });
    expect(toggle).toBeInTheDocument();
    await user.click(toggle);

    expect(document.documentElement.getAttribute("data-theme")).toBe("control-light");
    expect(localStorage.getItem(STORAGE_KEYS.theme)).toBe("control-light");
    expect(screen.getByRole("button", { name: "Switch to Console Dark" })).toBeInTheDocument();
  });

  it("toggles back to dark from light", async () => {
    const user = userEvent.setup();
    localStorage.setItem(STORAGE_KEYS.theme, "control-light");
    render(
      <ThemeProvider>
        <ThemeSwitcher />
      </ThemeProvider>,
    );

    await user.click(screen.getByRole("button", { name: "Switch to Console Dark" }));
    expect(document.documentElement.getAttribute("data-theme")).toBe("control");
    expect(localStorage.getItem(STORAGE_KEYS.theme)).toBe("control");
  });
});
