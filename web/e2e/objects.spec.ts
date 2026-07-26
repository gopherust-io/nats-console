import { expect, test } from "@playwright/test";
import { emptyBuckets, sampleObjectBucket } from "./fixtures/api";
import { accountBase } from "./fixtures/cluster";
import { mockClusterApis, mockObjectBucket, mockShell } from "./helpers/mockApi";

const objectsBase = `${accountBase()}/jetstream/objects`;

test.describe("objects", () => {
  test.beforeEach(async ({ page }) => {
    await mockShell(page);
    await mockClusterApis(page);
  });

  test("bucket list empty state", async ({ page }) => {
    await page.goto(objectsBase);
    await expect(page.getByRole("heading", { name: "Object Stores" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "No object buckets yet" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Create Object Bucket" }).first()).toBeVisible();
  });

  test("create object bucket", async ({ page }) => {
    let buckets = [...emptyBuckets.buckets];

    await page.route("**/api/v1/clusters/*/objects/buckets**", async (route) => {
      const method = route.request().method();
      const path = new URL(route.request().url()).pathname;
      if (method === "POST" && /\/objects\/buckets\/?$/.test(path)) {
        const body = route.request().postDataJSON() as { bucket: string };
        const created = sampleObjectBucket(body.bucket);
        buckets = [created];
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(created),
        });
        return;
      }
      if (method === "GET" && /\/objects\/buckets\/?$/.test(path)) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ buckets, total: buckets.length }),
        });
        return;
      }
      await route.fallback();
    });

    await page.goto(objectsBase);
    await page.getByRole("button", { name: "Create Object Bucket" }).first().click();
    await expect(page.getByRole("heading", { name: "Create Object Bucket" })).toBeVisible();
    await page.locator("#obj-cfg-name").fill("BLOBS");
    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.getByRole("link", { name: "BLOBS" })).toBeVisible();
  });

  test("bucket detail empty objects", async ({ page }) => {
    const bucket = sampleObjectBucket("BLOBS");
    await mockObjectBucket(page, bucket);

    await page.goto(`${objectsBase}/BLOBS`);
    await expect(page.getByRole("heading", { name: "BLOBS" })).toBeVisible();
    await expect(page.getByText("No objects in this bucket")).toBeVisible();
  });

  test("delete object bucket", async ({ page }) => {
    let buckets = [sampleObjectBucket("BLOBS")];
    let deleted = false;

    await page.unroute("**/api/v1/clusters/*/objects/buckets**");
    await page.route(/\/api\/v1\/clusters\/[^/]+\/objects\/buckets/, async (route) => {
      const method = route.request().method();
      const path = new URL(route.request().url()).pathname;
      if (method === "DELETE") {
        deleted = true;
        buckets = [];
        await route.fulfill({ status: 204, body: "" });
        return;
      }
      if (method === "GET" && /\/objects\/buckets\/?$/.test(path)) {
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

    await page.goto(objectsBase);
    await expect(page.getByRole("link", { name: "BLOBS" })).toBeVisible();
    page.once("dialog", (dialog) => {
      void dialog.accept();
    });
    await page.getByRole("button", { name: "Delete" }).click();
    await expect.poll(() => deleted).toBe(true);
    await expect(page.getByRole("heading", { name: "No object buckets yet" })).toBeVisible();
  });
});
