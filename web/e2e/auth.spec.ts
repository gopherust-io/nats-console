import { expect, test } from "@playwright/test";
import { CLUSTER } from "./fixtures/cluster";
import { mockInvite, mockLoggedOut, mockLoginSuccess } from "./helpers/auth";
import { mockJson, mockShell } from "./helpers/mockApi";

test.describe("auth", () => {
  test("login form is visible", async ({ page }) => {
    await mockLoggedOut(page);
    await page.goto("/login");
    await expect(page.getByRole("heading", { name: "Sign In" })).toBeVisible();
    await expect(page.getByLabel("Username")).toBeVisible();
    await expect(page.getByLabel("Password")).toBeVisible();
    await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
  });

  test("successful login redirects to systems", async ({ page }) => {
    await mockLoginSuccess(page);
    await mockJson(page, "**/api/v1/clusters**", { data: [CLUSTER], meta: { total: 1 } });

    await page.goto("/login");
    await page.getByLabel("Username").fill("admin");
    await page.getByLabel("Password").fill("secret");
    await page.getByRole("button", { name: "Sign in" }).click();

    await expect(page).toHaveURL(/\/systems/);
    await expect(page.getByRole("heading", { name: "Clusters" })).toBeVisible({ timeout: 15_000 });
  });

  test("unauthenticated visit to /systems redirects to login", async ({ page }) => {
    await mockLoggedOut(page);
    await page.goto("/systems");
    await expect(page).toHaveURL(/\/login/);
    await expect(page.getByRole("heading", { name: "Sign In" })).toBeVisible();
  });

  test("invite accept loads and signs in", async ({ page }) => {
    const token = "invite-token-1";
    await mockInvite(page, token);
    await mockJson(page, "**/api/v1/clusters**", { data: [CLUSTER], meta: { total: 1 } });

    await page.goto(`/invite/${token}`);
    await expect(page.getByRole("heading", { name: "Accept invite" })).toBeVisible();
    await expect(page.getByText("invited", { exact: true })).toBeVisible();

    await page.getByLabel("Password", { exact: true }).fill("password1");
    await page.getByLabel("Confirm password").fill("password1");
    await page.getByRole("button", { name: "Set password and sign in" }).click();

    await expect(page).toHaveURL(/\/systems/);
    await expect(page.getByRole("heading", { name: "Clusters" })).toBeVisible({ timeout: 15_000 });
  });

  test("invite rejects short password", async ({ page }) => {
    const token = "invite-token-2";
    await mockInvite(page, token);

    await page.goto(`/invite/${token}`);
    await expect(page.getByRole("heading", { name: "Accept invite" })).toBeVisible();
    await page.getByLabel("Password", { exact: true }).fill("short");
    await page.getByLabel("Confirm password").fill("short");
    await page.getByRole("button", { name: "Set password and sign in" }).click();

    await expect(page.getByText("Password must be at least 8 characters")).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/invite/${token}`));
  });

  test("invite rejects mismatched passwords", async ({ page }) => {
    const token = "invite-token-3";
    await mockInvite(page, token);

    await page.goto(`/invite/${token}`);
    await expect(page.getByRole("heading", { name: "Accept invite" })).toBeVisible();
    await page.getByLabel("Password", { exact: true }).fill("password1");
    await page.getByLabel("Confirm password").fill("password2");
    await page.getByRole("button", { name: "Set password and sign in" }).click();

    await expect(page.getByText("Passwords do not match")).toBeVisible();
  });

  test("authenticated user can open sign out from user menu", async ({ page }) => {
    await mockShell(page);
    await mockJson(page, "**/api/v1/auth/logout", {});

    await page.goto("/systems");
    await page.getByRole("button", { name: "Open user menu" }).click();
    await expect(page.getByRole("menuitem", { name: "Sign Out" })).toBeVisible();
  });
});
