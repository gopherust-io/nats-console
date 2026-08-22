import { expect, test } from "@playwright/test";
import {
  alertRuleMetrics,
  emptyAlerts,
  emptyAudit,
  emptyRules,
  emptyTopology,
  sampleAlert,
} from "./fixtures/api";
import { CLUSTER } from "./fixtures/cluster";
import { mockClusterApis, mockJson, mockShell } from "./helpers/mockApi";

test.describe("admin", () => {
  test.beforeEach(async ({ page }) => {
    await mockShell(page);
    await mockClusterApis(page);
  });

  test("clusters page check availability", async ({ page }) => {
    await page.route(`**/api/v1/clusters/${CLUSTER.id}/test`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ ok: true, message: "reachable", serverName: "n1", jetstream: true }),
      });
    });

    await page.goto("/systems/clusters");
    await expect(page.getByRole("heading", { name: "Cluster registry" })).toBeVisible();
    await expect(page.getByRole("cell", { name: CLUSTER.name, exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Check availability" }).click();
    await expect(page.getByText("Available")).toBeVisible();
  });

  test("topology page loads empty", async ({ page }) => {
    await mockJson(page, "**/api/v1/clusters/*/topology**", emptyTopology);
    await page.goto("/admin/topology");
    await expect(page.getByRole("heading", { name: "Topology", level: 1 })).toBeVisible();
    await expect(page.getByRole("heading", { name: "No JetStream topology yet" })).toBeVisible({
      timeout: 15_000,
    });
  });

  test("audit log empty", async ({ page }) => {
    await mockJson(page, "**/api/v1/audit**", emptyAudit);
    await page.goto("/admin/audit");
    await expect(page.getByRole("heading", { name: "Audit Log" })).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText("No audit entries yet")).toBeVisible();
  });

  test("console users invite person", async ({ page }) => {
    await mockJson(page, "**/api/v1/users**", { data: [], meta: { total: 0 } });
    await page.route("**/api/v1/people/invite", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: { inviteUrl: "http://127.0.0.1:4173/invite/abc" } }),
      });
    });

    await page.goto("/admin/users");
    await expect(page.getByRole("heading", { name: "Console Users" })).toBeVisible();
    const invite = page.locator(".card").filter({ hasText: "Invite person" });
    await expect(invite.getByRole("heading", { name: "Invite person" })).toBeVisible();

    await invite.getByLabel("Username").fill("newbie");
    await invite.getByLabel("Email").fill("newbie@example.com");
    await invite.getByRole("button", { name: "Create invite link" }).click();
    await expect(page.getByText("Invite URL:")).toBeVisible();
    await expect(page.getByText(/\/invite\/abc/)).toBeVisible();
  });

  test("alerts empty open tab", async ({ page }) => {
    await mockJson(page, "**/api/v1/alerts?*", emptyAlerts);
    await mockJson(page, "**/api/v1/alerts", emptyAlerts);
    await page.goto("/admin/alerts");
    await expect(page.getByRole("heading", { name: "Alerts", level: 1 })).toBeVisible();
    await expect(page.getByRole("heading", { name: "No alerts" })).toBeVisible();
  });

  test("acknowledge open alert", async ({ page }) => {
    const alert = sampleAlert();
    let current = { ...alert };

    await page.route("**/api/v1/alerts/open-summary", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: { count: current.acknowledgedAt ? 0 : 1, alerts: current.acknowledgedAt ? [] : [current] } }),
      });
    });

    await page.route("**/api/v1/alerts/**", async (route) => {
      const url = route.request().url();
      if (url.includes("/acknowledge") && route.request().method() === "POST") {
        current = {
          ...current,
          acknowledgedAt: "2024-01-01T00:10:00Z",
          acknowledgedBy: "admin",
        };
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: current }),
        });
        return;
      }
      await route.fallback();
    });

    await page.route("**/api/v1/alerts?*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: [current],
          meta: { total: 1 },
        }),
      });
    });

    // Also match /api/v1/alerts without query
    await page.route("**/api/v1/alerts", async (route) => {
      if (route.request().url().includes("open-summary")) {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: [current], meta: { total: 1 } }),
      });
    });

    await page.goto("/admin/alerts");
    await expect(page.getByText("CPU high")).toBeVisible();
    await page.getByRole("button", { name: "Acknowledge" }).click();
    await expect(page.getByText("Acknowledged")).toBeVisible();
  });

  test("alert rules create rule", async ({ page }) => {
    let rules: Array<{
      id: string;
      name: string;
      message: string;
      severity: string;
      metric: string;
      comparator: string;
      threshold: number;
      enabled: boolean;
      createdAt: string;
      updatedAt: string;
    }> = [];

    await mockJson(page, "**/api/v1/alert-rules/metrics", { data: alertRuleMetrics });
    await page.route("**/api/v1/alert-rules**", async (route) => {
      const method = route.request().method();
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith("/metrics")) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: alertRuleMetrics }),
        });
        return;
      }
      if (method === "POST") {
        const body = route.request().postDataJSON() as {
          name: string;
          message: string;
          severity: string;
          metric: string;
          comparator: string;
          threshold: number;
          enabled: boolean;
        };
        const created = {
          id: "rule-1",
          ...body,
          createdAt: "2024-01-01T00:00:00Z",
          updatedAt: "2024-01-01T00:00:00Z",
        };
        rules = [created];
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: created }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: rules, meta: { total: rules.length } }),
      });
    });

    await page.goto("/admin/alert-rules");
    await expect(page.getByRole("heading", { name: "Alert rules", level: 1 })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Create rule" })).toBeVisible();

    await page.locator("form.form-grid").getByLabel("Name", { exact: true }).fill("High CPU");
    await page.locator("form.form-grid").getByRole("button", { name: "Create rule" }).click();
    await expect(page.getByRole("cell", { name: "High CPU" })).toBeVisible();
  });
});
