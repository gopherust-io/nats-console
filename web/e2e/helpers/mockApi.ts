import { type Page, type Route } from "@playwright/test";
import {
  alertRuleMetrics,
  connectedStatus,
  emptyAccount,
  emptyBuckets,
  emptyConnz,
  emptyConsumers,
  emptyExports,
  emptyGrants,
  emptyJsz,
  emptyKeys,
  emptyMetricsHistory,
  emptyNatsUsers,
  emptyObjects,
  emptyPeople,
  emptySigningGroups,
  emptyStreams,
  emptyVarz,
} from "../fixtures/api";
import { CLUSTER } from "../fixtures/cluster";
import { ADMIN, type AuthUserFixture } from "../fixtures/users";

async function fulfillJson(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

export async function mockJson(
  page: Page,
  glob: string,
  body: unknown | ((route: Route) => unknown | Promise<unknown>),
  status = 200,
) {
  await page.route(glob, async (route) => {
    const resolved = typeof body === "function" ? await body(route) : body;
    await fulfillJson(route, resolved, status);
  });
}

/** Shell APIs needed on every authenticated page. Catch-all first; overrides register after. */
export async function mockShell(page: Page, user: AuthUserFixture = ADMIN) {
  await page.route("**/api/**", async (route) => {
    if (route.request().resourceType() === "xhr" || route.request().resourceType() === "fetch") {
      await fulfillJson(route, {});
      return;
    }
    await route.fallback();
  });

  await mockJson(page, "**/api/v1/alerts/open-summary", { count: 0, alerts: [] });
  await mockJson(page, "**/api/v1/auth/config", { basicEnabled: true, authEnabled: true });
  await mockJson(page, "**/api/v1/auth/me", user);
  await mockJson(page, "**/api/v1/clusters**", { clusters: [CLUSTER], total: 1 });
}

/** Common cluster-scoped GETs so account/jetstream pages render cleanly. */
export async function mockClusterApis(page: Page) {
  await mockJson(page, "**/api/v1/clusters/connections", connectedStatus);
  await mockJson(page, "**/api/v1/clusters/*/account", emptyAccount);
  await mockJson(page, "**/api/v1/clusters/*/streams**", emptyStreams);
  await mockJson(page, "**/api/v1/clusters/*/kv/buckets**", emptyBuckets);
  await mockJson(page, "**/api/v1/clusters/*/objects/buckets**", emptyBuckets);
  await mockJson(page, "**/api/v1/clusters/*/monitoring/varz**", emptyVarz);
  await mockJson(page, "**/api/v1/clusters/*/monitoring/connz**", emptyConnz);
  await mockJson(page, "**/api/v1/clusters/*/monitoring/jsz**", emptyJsz);
  await mockJson(page, "**/api/v1/clusters/*/metrics/history**", emptyMetricsHistory);
  await mockJson(page, "**/api/v1/clusters/*/access**", emptyGrants);
  await mockJson(page, "**/api/v1/clusters/*/accounts/*/access**", emptyGrants);
  await mockJson(page, "**/api/v1/clusters/*/sharing/exports**", emptyExports);
  await mockJson(page, "**/api/v1/clusters/*/nats-users**", emptyNatsUsers);
  await mockJson(page, "**/api/v1/clusters/*/signing-groups**", emptySigningGroups);
  await mockJson(page, "**/api/v1/people**", emptyPeople);
}

export async function mockStreamDetail(
  page: Page,
  stream: ReturnType<typeof import("../fixtures/api").sampleStream>,
  consumers: unknown[] = [],
) {
  await mockJson(page, `**/api/v1/clusters/*/streams/${encodeURIComponent(stream.config.name)}`, stream);
  await mockJson(page, `**/api/v1/clusters/*/streams/${encodeURIComponent(stream.config.name)}/consumers**`, {
    ...emptyConsumers,
    consumers,
    total: consumers.length,
  });
}

export async function mockKVBucket(
  page: Page,
  bucket: ReturnType<typeof import("../fixtures/api").sampleKVBucket>,
  keys: string[] = [],
) {
  await mockJson(page, `**/api/v1/clusters/*/kv/buckets/${encodeURIComponent(bucket.bucket)}`, bucket);
  await mockJson(page, `**/api/v1/clusters/*/kv/buckets/${encodeURIComponent(bucket.bucket)}/keys**`, {
    ...emptyKeys,
    keys,
    total: keys.length,
  });
}

export async function mockObjectBucket(
  page: Page,
  bucket: ReturnType<typeof import("../fixtures/api").sampleObjectBucket>,
  objects: string[] = [],
) {
  await mockJson(page, `**/api/v1/clusters/*/objects/buckets/${encodeURIComponent(bucket.bucket)}`, bucket);
  await mockJson(page, `**/api/v1/clusters/*/objects/buckets/${encodeURIComponent(bucket.bucket)}/objects**`, {
    ...emptyObjects,
    objects,
    total: objects.length,
  });
}

export { fulfillJson };
