import { expect, test } from "@playwright/test";
import { sampleConsumer, sampleStream } from "./fixtures/api";
import { accountBase } from "./fixtures/cluster";
import { mockClusterApis, mockJson, mockShell, mockStreamDetail } from "./helpers/mockApi";

const base = accountBase();
const jetstream = `${base}/jetstream`;

test.describe("jetstream", () => {
  test.beforeEach(async ({ page }) => {
    await mockShell(page);
    await mockClusterApis(page);
  });

  test("hub shows empty state", async ({ page }) => {
    await page.goto(jetstream);
    await expect(page.getByRole("heading", { name: "JetStream" })).toBeVisible();
    await expect(page.getByText("No streams found. Click Create Stream to create one.")).toBeVisible();
    await expect(page.getByRole("button", { name: /Create Stream/i })).toBeVisible();
  });

  test("create stream from hub", async ({ page }) => {
    let streams: ReturnType<typeof sampleStream>[] = [];

    await page.route("**/api/v1/clusters/*/streams**", async (route) => {
      const method = route.request().method();
      const url = route.request().url();
      if (method === "POST" && !url.includes("/consumers") && !url.includes("/messages") && !url.includes("/purge")) {
        const body = route.request().postDataJSON() as { name: string; subjects?: string[] };
        const created = sampleStream(body.name);
        created.config.subjects = body.subjects ?? ["orders.>"];
        streams = [created];
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: created }),
        });
        return;
      }
      if (method === "GET" && /\/streams(\?|$)/.test(new URL(url).pathname)) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: streams, meta: { total: streams.length } }),
        });
        return;
      }
      await route.fallback();
    });

    await page.goto(jetstream);
    await page.getByRole("button", { name: /Create Stream/i }).click();
    await page.locator(".nc-dropdown__menu").getByRole("button", { name: "Stream", exact: true }).click();

    await expect(page.getByRole("heading", { name: "Create Stream" })).toBeVisible();
    await page.locator("#stream-cfg-name").fill("ORDERS");
    const subjectInput = page.locator('input[placeholder="orders.*"]');
    await subjectInput.fill("orders.>");
    await subjectInput.press("Enter");
    await page.getByRole("button", { name: "Save" }).click();

    await expect(page.getByRole("link", { name: "ORDERS" })).toBeVisible();
  });

  test("stream detail shows consumers and publish", async ({ page }) => {
    const stream = sampleStream("ORDERS");
    await mockStreamDetail(page, stream);

    await page.route("**/api/v1/clusters/*/streams/ORDERS/messages**", async (route) => {
      if (route.request().method() === "POST") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ data: { seq: 1 } }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: { message: { seq: 1, subject: "orders.new", time: "2024-01-01T00:00:00Z", data: btoa("{}") } },
        }),
      });
    });

    await page.goto(`${jetstream}/streams/ORDERS`);
    await expect(page.getByRole("heading", { name: "ORDERS", level: 1 })).toBeVisible();
    await expect(page.getByRole("link", { name: "Live Tail" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Purge Stream" })).toBeVisible();

    await page.getByRole("button", { name: "Consumers" }).click();
    await expect(page.getByRole("heading", { name: "Consumers", level: 2 })).toBeVisible();
    await expect(page.getByText("No consumers")).toBeVisible();

    await page.getByRole("button", { name: "Messages" }).click();
    await page.getByRole("button", { name: "Publish" }).click();
  });

  test("create consumer on stream detail", async ({ page }) => {
    const stream = sampleStream("ORDERS");
    let consumers: ReturnType<typeof sampleConsumer>[] = [];
    let lastCreateBody: Record<string, unknown> | null = null;

    await mockJson(page, "**/api/v1/clusters/*/streams/ORDERS", { data: stream });
    await page.route("**/api/v1/clusters/*/streams/ORDERS/consumers**", async (route) => {
      const method = route.request().method();
      if (method === "POST") {
        lastCreateBody = route.request().postDataJSON() as Record<string, unknown>;
        const body = lastCreateBody as { durableName?: string; name?: string };
        const created = sampleConsumer(body.durableName || body.name || "worker");
        consumers = [created];
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
        body: JSON.stringify({ data: consumers, meta: { total: consumers.length, offset: 0, limit: 50 } }),
      });
    });

    await page.goto(`${jetstream}/streams/ORDERS`);
    await page.getByRole("button", { name: "Consumers" }).click();
    await page.getByRole("button", { name: "Create Consumer" }).click();
    await expect(page.getByRole("heading", { name: "Create Consumer" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Apply recommended setup" })).toBeVisible();
    await page.getByRole("button", { name: "Apply recommended setup" }).click();
    await expect(page.locator("#cons-cfg-name")).toHaveValue("ORDERS-worker");
    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.getByRole("link", { name: "ORDERS-worker" })).toBeVisible();
    await expect.poll(() => lastCreateBody).not.toBeNull();
    expect(lastCreateBody).toMatchObject({
      durableName: "ORDERS-worker",
      deliverPolicy: "all",
      ackPolicy: "explicit",
      filterSubject: "orders.>",
    });
    expect(lastCreateBody?.deliverSubject).toBeUndefined();
    expect(lastCreateBody?.filterSubjects).toBeUndefined();
  });

  test("purge stream confirms and calls API", async ({ page }) => {
    const stream = sampleStream("ORDERS");
    await mockStreamDetail(page, stream);

    let purged = false;
    await page.route("**/api/v1/clusters/*/streams/ORDERS/purge**", async (route) => {
      purged = true;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: { purged: 0 } }),
      });
    });

    await page.goto(`${jetstream}/streams/ORDERS`);
    await page.getByRole("button", { name: "Purge Stream" }).click();
    await expect(page.getByRole("alertdialog")).toBeVisible();
    await page.getByRole("button", { name: "Purge" }).click();
    await expect.poll(() => purged).toBe(true);
  });

  test("consumer detail loads and delete", async ({ page }) => {
    const stream = {
      ...sampleStream("ORDERS"),
      state: {
        messages: 100,
        bytes: 1024,
        firstSeq: 1,
        lastSeq: 100,
        consumerCount: 1,
      },
    };
    const consumer = sampleConsumer("worker");
    await mockJson(page, "**/api/v1/clusters/*/streams/ORDERS", { data: stream });
    await mockJson(page, "**/api/v1/clusters/*/streams/ORDERS/consumers/worker", { data: consumer });

    let deleted = false;
    await page.route("**/api/v1/clusters/*/streams/ORDERS/consumers/worker", async (route) => {
      if (route.request().method() === "DELETE") {
        deleted = true;
        await route.fulfill({ status: 204, body: "" });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: consumer }),
      });
    });

    await page.goto(`${jetstream}/streams/ORDERS/consumers/worker`);
    await expect(page.getByRole("heading", { name: "worker" })).toBeVisible();
    await expect(page.getByText("Lag", { exact: true })).toBeVisible();
    await expect(page.getByText("Pending", { exact: true })).toBeVisible();
    await expect(page.getByText("Waiting", { exact: true })).toBeVisible();
    await expect(page.getByText("Redelivered", { exact: true })).toBeVisible();
    await expect(page.getByText("Ack Wait", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Delete Consumer" })).toBeVisible();
    await page.getByRole("button", { name: "Delete Consumer" }).click();
    await expect(page.getByRole("alertdialog")).toBeVisible();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect.poll(() => deleted).toBe(true);
  });

  test("live stream page shows waiting state", async ({ page }) => {
    const stream = sampleStream("ORDERS");
    await mockJson(page, "**/api/v1/clusters/*/streams/ORDERS", { data: stream });

    await page.goto(`${jetstream}/streams/ORDERS/live`);
    await expect(page.getByRole("heading", { name: "Live: ORDERS" })).toBeVisible();
    await expect(page.getByText(/Waiting for messages/)).toBeVisible();
    await expect(page.getByRole("button", { name: /Pause|Resume/i })).toBeVisible();
  });
});
