import { expect, test } from "@playwright/test";
import { CLUSTER } from "./fixtures/cluster";
import { ADMIN, VIEWER } from "./fixtures/users";
import { mockClusterApis, mockShell } from "./helpers/mockApi";

test.describe("rbac and redirects", () => {
  test("admin user menu shows gated links", async ({ page }) => {
    await mockShell(page, ADMIN);
    await mockClusterApis(page);
    await page.goto("/systems");
    await page.getByRole("button", { name: "Open user menu" }).click();
    await expect(page.getByRole("link", { name: "Audit Log" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Console Users" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Alert rules" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Alerts" })).toBeVisible();
  });

  test("viewer user menu hides admin-only links", async ({ page }) => {
    await mockShell(page, VIEWER);
    await mockClusterApis(page);
    await page.goto("/systems");
    await page.getByRole("button", { name: "Open user menu" }).click();
    await expect(page.getByRole("link", { name: "Alerts" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Audit Log" })).toHaveCount(0);
    await expect(page.getByRole("link", { name: "Console Users" })).toHaveCount(0);
    await expect(page.getByRole("link", { name: "Alert rules" })).toHaveCount(0);
  });

  test("viewer is redirected from audit", async ({ page }) => {
    await mockShell(page, VIEWER);
    await mockClusterApis(page);
    await page.goto("/admin/audit");
    await expect(page).toHaveURL(/\/systems/);
  });

  test("viewer is redirected from console users", async ({ page }) => {
    await mockShell(page, VIEWER);
    await mockClusterApis(page);
    await page.goto("/admin/users");
    await expect(page).toHaveURL(/\/systems/);
  });

  test("viewer is redirected from alert rules", async ({ page }) => {
    await mockShell(page, VIEWER);
    await mockClusterApis(page);
    await page.goto("/admin/alert-rules");
    await expect(page).toHaveURL(/\/admin\/alerts/);
  });

  test("legacy dashboard redirects to systems", async ({ page }) => {
    await mockShell(page);
    await mockClusterApis(page);
    await page.goto("/dashboard");
    await expect(page).toHaveURL(/\/systems/);
  });

  test("legacy clusters redirects to systems clusters", async ({ page }) => {
    await mockShell(page);
    await mockClusterApis(page);
    await page.goto("/clusters");
    await expect(page).toHaveURL(/\/systems\/clusters/);
  });

  test("legacy audit redirects to admin audit", async ({ page }) => {
    await mockShell(page);
    await mockClusterApis(page);
    await page.goto("/audit");
    await expect(page).toHaveURL(/\/admin\/audit/);
  });

  test("legacy users redirects to admin users", async ({ page }) => {
    await mockShell(page);
    await mockClusterApis(page);
    await page.goto("/users");
    await expect(page).toHaveURL(/\/admin\/users/);
  });

  test("legacy topology redirects to admin topology", async ({ page }) => {
    await mockShell(page);
    await mockClusterApis(page);
    await page.goto("/topology");
    await expect(page).toHaveURL(/\/admin\/topology/);
  });

  test("legacy stream path redirects into account jetstream", async ({ page }) => {
    await mockShell(page);
    await mockClusterApis(page);
    await page.addInitScript((id) => {
      localStorage.setItem("nats-consol-cluster", id);
    }, CLUSTER.id);
    await page.goto("/streams/ORDERS");
    await expect(page).toHaveURL(
      new RegExp(`/systems/${CLUSTER.id}/accounts/Default/jetstream/streams/ORDERS`),
    );
  });
});
