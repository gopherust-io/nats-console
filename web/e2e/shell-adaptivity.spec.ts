import { expect, test, type Page } from "@playwright/test";
import { mockLoggedOut } from "./helpers/auth";
import { mockShell } from "./helpers/mockApi";

async function assertNoHorizontalOverflow(page: Page) {
  const overflow = await page.evaluate(() => {
    const doc = document.documentElement;
    return {
      scrollWidth: doc.scrollWidth,
      clientWidth: doc.clientWidth,
    };
  });
  expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.clientWidth + 1);
}

const VIEWPORTS = [
  { name: "mobile", width: 375, height: 812 },
  { name: "tablet", width: 768, height: 1024 },
  { name: "desktop", width: 1280, height: 720 },
] as const;

for (const vp of VIEWPORTS) {
  test.describe(`shell adaptivity (${vp.name})`, () => {
    test.use({ viewport: { width: vp.width, height: vp.height } });

    test("loads systems shell without horizontal overflow", async ({ page }) => {
      await mockShell(page);
      await page.goto("/systems");
      await expect(page.getByRole("button", { name: /Switch to Console/i })).toBeVisible();
      await expect(page.getByRole("button", { name: "Notifications" })).toBeVisible();
      await expect(page.getByRole("button", { name: "Open user menu" })).toBeVisible();
      await assertNoHorizontalOverflow(page);

      const boxes = await page.locator(".nc-topbar__actions .nc-icon-btn").evaluateAll((els) =>
        els.map((el) => {
          const r = el.getBoundingClientRect();
          return { left: r.left, right: r.right, top: r.top };
        }),
      );
      expect(boxes.length).toBeGreaterThanOrEqual(3);
      for (const box of boxes) {
        expect(box.left).toBeGreaterThanOrEqual(0);
        expect(box.right).toBeLessThanOrEqual(vp.width + 1);
      }
    });

    test("theme toggle persists across reload", async ({ page }) => {
      await mockShell(page);
      await page.goto("/systems");
      const toggle = page.getByRole("button", { name: /Switch to Console/i });
      await expect(toggle).toBeVisible();

      const before = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
      await toggle.click();
      const after = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
      expect(after).not.toBe(before);
      expect(["control", "control-light"]).toContain(after);

      await mockShell(page);
      await page.reload();
      await expect(page.getByRole("button", { name: /Switch to Console/i })).toBeVisible();
      await expect
        .poll(async () => page.evaluate(() => document.documentElement.getAttribute("data-theme")))
        .toBe(after);
    });
  });
}

test.describe("login adaptivity", () => {
  test.use({ viewport: { width: 375, height: 812 } });

  test("login form is usable without horizontal overflow", async ({ page }) => {
    await mockLoggedOut(page);

    await page.goto("/login");
    await expect(page.getByRole("heading", { name: "Sign In" })).toBeVisible();
    await expect(page.getByLabel("Username")).toBeVisible();
    await expect(page.getByLabel("Password")).toBeVisible();
    await assertNoHorizontalOverflow(page);
  });
});
