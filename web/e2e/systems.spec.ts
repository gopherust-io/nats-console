import { expect, test } from "@playwright/test";
import { emptyGrants, samplePerson } from "./fixtures/api";
import { ACCOUNT, accountBase, CLUSTER } from "./fixtures/cluster";
import { mockClusterApis, mockJson, mockShell } from "./helpers/mockApi";

const base = accountBase();

test.describe("systems and accounts", () => {
  test.beforeEach(async ({ page }) => {
    await mockShell(page);
    await mockClusterApis(page);
  });

  test("systems list shows connected cluster", async ({ page }) => {
    await page.goto("/systems");
    await expect(page.getByRole("heading", { name: "Systems" })).toBeVisible();
    await expect(page.getByText(CLUSTER.name)).toBeVisible();
    await expect(page.getByText("Connected")).toBeVisible();
  });

  test("system accounts page lists Default", async ({ page }) => {
    await page.goto(`/systems/${CLUSTER.id}`);
    await expect(page.getByRole("heading", { name: "Accounts" })).toBeVisible();
    await expect(page.getByText(ACCOUNT)).toBeVisible();
    await page.getByRole("link", { name: ACCOUNT }).click();
    await expect(page).toHaveURL(new RegExp(`${CLUSTER.id}/accounts/${ACCOUNT}`));
  });

  test("system usage page shows usage cards", async ({ page }) => {
    await page.goto(`/systems/${CLUSTER.id}/usage`);
    await expect(page.getByRole("heading", { name: "Usage" })).toBeVisible();
    await expect(page.getByText("Streams")).toBeVisible();
    await expect(page.getByText("Consumers")).toBeVisible();
  });

  test("system access empty and add user", async ({ page }) => {
    const person = samplePerson();
    let grants: Array<{
      id: string;
      userId: string;
      username?: string;
      email?: string;
      resourceType: string;
      resourceKey: string;
      role: string;
    }> = [];

    await mockJson(page, "**/api/v1/people**", { data: [person], meta: { total: 1 } });
    await page.route(`**/api/v1/clusters/${CLUSTER.id}/access**`, async (route) => {
      const method = route.request().method();
      if (method === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: grants, meta: { total: grants.length } }),
        });
        return;
      }
      if (method === "POST") {
        const body = route.request().postDataJSON() as { userId: string; role: string };
        grants = [
          {
            id: "grant-1",
            userId: body.userId,
            username: person.username,
            email: person.email,
            resourceType: "system",
            resourceKey: CLUSTER.id,
            role: body.role,
          },
        ];
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: grants[0] }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(emptyGrants),
      });
    });

    await page.goto(`/systems/${CLUSTER.id}/access`);
    await expect(page.getByRole("heading", { name: "Access" })).toBeVisible();
    await expect(page.getByText("No access grants yet.")).toBeVisible();

    await page.getByLabel("Person").selectOption(person.id);
    await page.getByRole("button", { name: "Add User" }).click();
    await expect(page.getByRole("cell", { name: person.username })).toBeVisible();
  });

  test("account overview loads", async ({ page }) => {
    await page.goto(base);
    await expect(page.getByRole("heading", { name: "Overview", level: 1 })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Streams" }).first()).toBeVisible();
    await expect(page.getByRole("heading", { name: "Consumers" }).first()).toBeVisible();
  });

  test("account connections empty state", async ({ page }) => {
    await page.goto(`${base}/connections`);
    await expect(page.getByRole("heading", { name: "Account Connections" })).toBeVisible();
    await expect(page.getByText("No connections found")).toBeVisible();
  });

  test("account overview shows settings sections", async ({ page }) => {
    // Legacy /settings redirects onto the account overview (settings live there).
    await page.goto(`${base}/settings`);
    await expect(page).toHaveURL(new RegExp(`/systems/${CLUSTER.id}/accounts/${ACCOUNT}$`));
    await expect(page.getByRole("heading", { name: "General" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Limits" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "JetStream" })).toBeVisible();
  });

  test("account sharing empty and create export", async ({ page }) => {
    let exports: Array<{
      id: string;
      name: string;
      subject: string;
      description: string;
      kind: string;
      createdAt: string;
    }> = [];

    await page.route("**/api/v1/clusters/*/sharing/exports**", async (route) => {
      const method = route.request().method();
      if (method === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: exports, meta: { total: exports.length } }),
        });
        return;
      }
      if (method === "POST") {
        const body = route.request().postDataJSON() as {
          name: string;
          subject: string;
          description?: string;
          kind: string;
        };
        const item = {
          id: "exp-1",
          name: body.name,
          subject: body.subject,
          description: body.description ?? "",
          kind: body.kind,
          createdAt: "2024-01-01T00:00:00Z",
        };
        exports = [item];
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: item }),
        });
        return;
      }
      await route.fallback();
    });

    await page.goto(`${base}/sharing`);
    await expect(page.getByRole("heading", { name: "Sharing" })).toBeVisible();
    await expect(page.getByText("No exports found.")).toBeVisible();

    await page.getByRole("button", { name: "Export Service" }).click();
    const form = page.locator("form.nc-settings-section");
    await expect(form.getByRole("heading", { name: "Export Service" })).toBeVisible();
    await form.locator("input").nth(0).fill("orders-svc");
    await form.locator("input").nth(1).fill("svc.orders");
    await form.getByRole("button", { name: "Save" }).click();

    await expect(page.getByRole("cell", { name: "orders-svc" })).toBeVisible();
  });

  test("account access empty state", async ({ page }) => {
    await page.goto(`${base}/access`);
    await expect(page.getByRole("heading", { name: "Access" })).toBeVisible();
    await expect(page.getByText("No access grants yet.")).toBeVisible();
  });

  test("nats users empty and create group button", async ({ page }) => {
    await page.goto(`${base}/users`);
    await expect(page.getByRole("heading", { name: "NATS Users" })).toBeVisible();
    await expect(page.getByText("No users found.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Create Group" })).toBeVisible();
  });
});
