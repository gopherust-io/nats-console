import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./api", () => ({
  api: vi.fn(),
  clusterPath: (clusterId: string, path: string) => `/api/v1/clusters/${clusterId}${path}`,
  jetStreamUIBase: (clusterId: string, accountName = "Default") =>
    `/systems/${clusterId}/accounts/${accountName}/jetstream`,
}));

import { api } from "./api";
import {
  EVENT_WIKIPEDIA_HREF,
  eventWikipediaCatalogHref,
  eventWikipediaIncidentHref,
  eventWikipediaSubjectHref,
  fetchEventWikipedia,
  filterEventWikipediaPages,
  sortEventWikipediaPages,
  type EventWikipediaPage,
} from "./eventWikipedia";

const mockedApi = vi.mocked(api);

const sample: EventWikipediaPage[] = [
  {
    subject: "orders.shipped",
    purpose: "Shipment",
    history: { streams: ["ORDERS"] },
    owner: "Ops",
    consumers: [],
    relatedEvents: [],
    knownIncidents: [],
    deprecation: { deprecated: false },
    documented: true,
    orphan: false,
  },
  {
    subject: "orders.created",
    purpose: "Order successfully created",
    history: { streams: ["ORDERS"] },
    owner: "Growth Team",
    consumers: [{ name: "billing", stream: "ORDERS", service: "billing-svc" }],
    relatedEvents: ["orders.new"],
    knownIncidents: [{ stream: "ORDERS", consumer: "billing" }],
    deprecation: { deprecated: true, successorSubject: "orders.v2" },
    documented: true,
    orphan: false,
  },
];

describe("sortEventWikipediaPages", () => {
  it("sorts by subject", () => {
    expect(sortEventWikipediaPages(sample).map((p) => p.subject)).toEqual([
      "orders.created",
      "orders.shipped",
    ]);
  });
});

describe("filterEventWikipediaPages", () => {
  it("filters by subject, owner, and related", () => {
    expect(filterEventWikipediaPages(sample, "growth")).toHaveLength(1);
    expect(filterEventWikipediaPages(sample, "orders.new")[0]?.subject).toBe("orders.created");
    expect(filterEventWikipediaPages(sample, "")).toHaveLength(2);
  });
});

describe("href helpers", () => {
  it("builds catalog, subject, and incident links", () => {
    expect(eventWikipediaCatalogHref("orders.created")).toBe(
      "/docs/event-catalog?q=orders.created",
    );
    expect(eventWikipediaSubjectHref("orders.created")).toBe(
      "/docs/event-wikipedia?subject=orders.created",
    );
    expect(
      eventWikipediaIncidentHref("cid", { stream: "ORDERS", consumer: "billing" }),
    ).toBe("/admin/audit?cluster=cid&stream=ORDERS&consumer=billing");
    expect(EVENT_WIKIPEDIA_HREF).toBe("/docs/event-wikipedia");
  });
});

describe("fetchEventWikipedia", () => {
  beforeEach(() => {
    mockedApi.mockReset();
  });

  it("requests fresh wikipedia with subject filter", async () => {
    mockedApi.mockResolvedValue({
      data: {
        pages: sample,
        totals: { total: 2, documented: 2, deprecated: 1, orphan: 0 },
      },
    });
    const snap = await fetchEventWikipedia("cid", { fresh: true, subject: "orders.created" });
    expect(mockedApi).toHaveBeenCalledWith(
      "/api/v1/clusters/cid/event-wikipedia?fresh=1&subject=orders.created",
    );
    expect(snap.pages).toHaveLength(2);
  });
});
