import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./api", () => ({
  api: vi.fn(),
  clusterPath: (clusterId: string, path: string) => `/api/v1/clusters/${clusterId}${path}`,
  jetStreamUIBase: (clusterId: string, accountName = "Default") =>
    `/systems/${clusterId}/accounts/${accountName}/jetstream`,
}));

import { api } from "./api";
import {
  EVENT_CATALOG_HREF,
  eventCatalogConsumerHref,
  filterEventCatalogEntries,
  formatEventCatalogSchema,
  fetchEventCatalog,
  parseEventCatalogSchema,
  sortEventCatalogEntries,
  eventCatalogWikipediaHref,
  type EventCatalogEntry,
} from "./eventCatalog";

const mockedApi = vi.mocked(api);

const sample: EventCatalogEntry[] = [
  {
    subject: "orders.shipped",
    owner: "Ops",
    streams: ["ORDERS"],
    consumers: [],
    documented: true,
    orphan: false,
  },
  {
    subject: "orders.created",
    owner: "Growth Team",
    description: "Order successfully created",
    streams: ["ORDERS"],
    consumers: [{ name: "billing", stream: "ORDERS", service: "billing-svc" }],
    documented: true,
    orphan: false,
  },
];

describe("sortEventCatalogEntries", () => {
  it("sorts by subject", () => {
    const sorted = sortEventCatalogEntries(sample);
    expect(sorted.map((e) => e.subject)).toEqual(["orders.created", "orders.shipped"]);
  });
});

describe("filterEventCatalogEntries", () => {
  it("filters by subject and owner", () => {
    expect(filterEventCatalogEntries(sample, "growth")).toHaveLength(1);
    expect(filterEventCatalogEntries(sample, "shipped")[0]?.subject).toBe("orders.shipped");
    expect(filterEventCatalogEntries(sample, "")).toHaveLength(2);
  });
});

describe("schema helpers", () => {
  it("formats and parses JSON Schema objects", () => {
    const schema = { type: "object", properties: { id: { type: "string" } } };
    const text = formatEventCatalogSchema(schema);
    expect(parseEventCatalogSchema(text)).toEqual({ schema });
    expect(parseEventCatalogSchema("").schema).toBeNull();
    expect(parseEventCatalogSchema("[]").error).toBeTruthy();
    expect(parseEventCatalogSchema("{").error).toBeTruthy();
  });
});

describe("fetchEventCatalog", () => {
  beforeEach(() => {
    mockedApi.mockReset();
  });

  it("requests fresh catalog", async () => {
    mockedApi.mockResolvedValue({
      data: {
        entries: sample,
        totals: { total: 2, documented: 2, undocumented: 0, orphan: 0 },
      },
    });
    const snap = await fetchEventCatalog("cid", { fresh: true });
    expect(mockedApi).toHaveBeenCalledWith("/api/v1/clusters/cid/event-catalog?fresh=1");
    expect(snap.entries).toHaveLength(2);
    expect(EVENT_CATALOG_HREF).toBe("/docs/event-catalog");
  });
});

describe("eventCatalogConsumerHref", () => {
  it("builds consumer detail path", () => {
    expect(
      eventCatalogConsumerHref({ name: "billing", stream: "ORDERS" }, "cid", "Default"),
    ).toBe("/systems/cid/accounts/Default/jetstream/streams/ORDERS/consumers/billing");
  });
});

describe("eventCatalogWikipediaHref", () => {
  it("builds wikipedia deep link", () => {
    expect(eventCatalogWikipediaHref("orders.created")).toBe(
      "/docs/event-wikipedia?subject=orders.created",
    );
  });
});
