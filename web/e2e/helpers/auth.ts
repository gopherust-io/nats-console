import { type Page } from "@playwright/test";
import { ADMIN, type AuthUserFixture } from "../fixtures/users";
import { mockJson } from "./mockApi";

async function mockApiCatchAll(page: Page) {
  await page.route("**/api/**", async (route) => {
    if (route.request().resourceType() === "xhr" || route.request().resourceType() === "fetch") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({}),
      });
      return;
    }
    await route.fallback();
  });
}

/** Logged-out session: config ok, me unauthorized. */
export async function mockLoggedOut(page: Page) {
  await mockApiCatchAll(page);
  await mockJson(page, "**/api/v1/auth/config", { data: { basicEnabled: true, authEnabled: true } });
  await mockJson(page, "**/api/v1/auth/me", { error: "unauthorized" }, 401);
}

/**
 * Login flow: me starts 401; POST /login returns user; subsequent me returns user.
 * Call before goto /login. Register clusters etc. after login success as needed.
 */
export async function mockLoginSuccess(page: Page, user: AuthUserFixture = ADMIN) {
  let loggedIn = false;

  await mockApiCatchAll(page);
  await mockJson(page, "**/api/v1/auth/config", { data: { basicEnabled: true, authEnabled: true } });
  await mockJson(page, "**/api/v1/alerts/open-summary", { data: { count: 0, alerts: [] } });

  await page.route("**/api/v1/auth/me", async (route) => {
    if (!loggedIn) {
      await route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: "unauthorized" }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: user }),
    });
  });

  await page.route("**/api/v1/auth/login", async (route) => {
    if (route.request().method() !== "POST") {
      await route.fallback();
      return;
    }
    loggedIn = true;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: user }),
    });
  });
}

export async function mockInvite(page: Page, token: string, user: AuthUserFixture = ADMIN) {
  let accepted = false;

  await mockApiCatchAll(page);
  await mockJson(page, "**/api/v1/auth/config", { data: { basicEnabled: true, authEnabled: true } });
  await mockJson(page, "**/api/v1/alerts/open-summary", { data: { count: 0, alerts: [] } });

  await page.route("**/api/v1/auth/me", async (route) => {
    if (!accepted) {
      // Use 403 (not 401): api() redirects to /login on 401 for non-login paths.
      await route.fulfill({
        status: 403,
        contentType: "application/json",
        body: JSON.stringify({ error: "forbidden" }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: user }),
    });
  });

  await mockJson(page, `**/api/v1/auth/invite/${encodeURIComponent(token)}`, {
    data: {
      username: "invited",
      email: "invited@example.com",
      expiresAt: "2099-01-01T00:00:00Z",
    },
  });

  await page.route("**/api/v1/auth/invite/accept", async (route) => {
    if (route.request().method() !== "POST") {
      await route.fallback();
      return;
    }
    accepted = true;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: { ok: true } }),
    });
  });
}
