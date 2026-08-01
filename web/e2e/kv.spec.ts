import { expect, test } from "@playwright/test";
import { emptyBuckets, sampleKVBucket, sampleKVEntry } from "./fixtures/api";
import { accountBase } from "./fixtures/cluster";
import { mockClusterApis, mockJson, mockKVBucket, mockShell } from "./helpers/mockApi";

const kvBase = `${accountBase()}/jetstream/kv`;

test.describe("kv", () => {
  test.beforeEach(async ({ page }) => {
    await mockShell(page);
    await mockClusterApis(page);
  });

  test("bucket list empty state", async ({ page }) => {
    await page.goto(kvBase);
    await expect(page.getByRole("heading", { name: "KV Stores" })).toBeVisible();
    await expect(page.getByText("No KV buckets")).toBeVisible();
    await expect(page.getByRole("button", { name: "Create KV Bucket" }).first()).toBeVisible();
  });

  test("create kv bucket", async ({ page }) => {
    let buckets = [...emptyBuckets.buckets];

    await page.route("**/api/v1/clusters/*/kv/buckets**", async (route) => {
      const method = route.request().method();
      const url = route.request().url();
      const path = new URL(url).pathname;
      if (method === "POST" && /\/kv\/buckets\/?$/.test(path)) {
        const body = route.request().postDataJSON() as { bucket: string };
        const created = sampleKVBucket(body.bucket);
        buckets = [created];
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(created),
        });
        return;
      }
      if (method === "GET" && /\/kv\/buckets\/?$/.test(path)) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ buckets, total: buckets.length }),
        });
        return;
      }
      await route.fallback();
    });

    await page.goto(kvBase);
    await page.getByRole("button", { name: "Create KV Bucket" }).click();
    await expect(page.getByRole("heading", { name: "Create KV Bucket" })).toBeVisible();
    await page.locator("#kv-cfg-name").fill("CONFIG");
    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.getByRole("link", { name: "CONFIG" })).toBeVisible();
  });

  test("bucket detail empty keys", async ({ page }) => {
    const bucket = sampleKVBucket("CONFIG");
    await mockKVBucket(page, bucket);

    await page.goto(`${kvBase}/CONFIG`);
    await expect(page.getByRole("heading", { name: "CONFIG" })).toBeVisible();
    await expect(page.getByText("No keys")).toBeVisible();
    await expect(page.getByRole("button", { name: "Edit config" })).toBeVisible();
  });

  test("key page shows value and history", async ({ page }) => {
    const entry = sampleKVEntry("CONFIG", "feature.flag");
    await mockJson(page, "**/api/v1/clusters/*/kv/buckets/CONFIG/keys/feature.flag", entry);
    await mockJson(page, "**/api/v1/clusters/*/kv/buckets/CONFIG/keys/feature.flag/history", {
      entries: [entry, { ...entry, revision: 2, value: btoa(JSON.stringify({ enabled: false })) }],
      total: 2,
    });

    await page.goto(`${kvBase}/CONFIG/feature.flag`);
    await expect(page.getByRole("heading", { name: "feature.flag" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "History" })).toBeVisible();
  });

  test("delete kv bucket", async ({ page }) => {
    let buckets = [sampleKVBucket("CONFIG")];
    let deleted = false;

    await page.unroute("**/api/v1/clusters/*/kv/buckets**");
    await page.route(/\/api\/v1\/clusters\/[^/]+\/kv\/buckets/, async (route) => {
      const method = route.request().method();
      const path = new URL(route.request().url()).pathname;
      if (method === "DELETE") {
        deleted = true;
        buckets = [];
        await route.fulfill({ status: 204, body: "" });
        return;
      }
      if (method === "GET" && /\/kv\/buckets\/?$/.test(path)) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ buckets, total: buckets.length }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(buckets[0] ?? {}),
      });
    });

    await page.goto(kvBase);
    await expect(page.getByRole("link", { name: "CONFIG" })).toBeVisible();
    await page.getByRole("button", { name: "Delete" }).click();
    await expect(page.getByRole("alertdialog")).toBeVisible();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect.poll(() => deleted).toBe(true);
    await expect(page.getByText("No KV buckets")).toBeVisible();
  });
});
