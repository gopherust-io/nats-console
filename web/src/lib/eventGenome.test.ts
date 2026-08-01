import { describe, expect, it, vi, beforeEach } from "vitest";
import {
  eventGenomeCatalogHref,
  eventGenomeFindingHref,
  eventGenomeFindingLabel,
  fetchEventGenome,
  GENOME_LOCATION_STATE,
  GENOME_TOPOLOGY_HREF,
  isFromGenome,
  sortEventGenomeFindings,
  type EventGenomeFinding,
} from "./eventGenome";

vi.mock("./api", () => ({
  api: vi.fn(),
  clusterPath: (id: string, path: string) => `/api/v1/clusters/${id}${path}`,
  jetStreamUIBase: (clusterId: string, accountName = "Default") =>
    `/systems/${clusterId}/accounts/${encodeURIComponent(accountName)}/jetstream`,
}));

import { api } from "./api";

const mockedApi = vi.mocked(api);

describe("sortEventGenomeFindings", () => {
  it("orders by cluster size then genome/subject", () => {
    const findings: EventGenomeFinding[] = [
      {
        subject: "a.new",
        suggested: "a.created",
        genome: "a.created",
        cluster: ["a.new", "a.created"],
        reasons: [],
      },
      {
        subject: "b.new",
        suggested: "b.created",
        genome: "b.created",
        cluster: ["b.new", "b.created", "b.added"],
        reasons: [],
      },
    ];
    const sorted = sortEventGenomeFindings(findings);
    expect(sorted.map((f) => f.genome)).toEqual(["b.created", "a.created"]);
  });
});

describe("eventGenomeFindingHref", () => {
  it("links to stream or consumer", () => {
    expect(
      eventGenomeFindingHref(
        { subject: "x", suggested: "y", genome: "x", cluster: [], stream: "ORDERS", reasons: [] },
        "c1",
      ),
    ).toBe("/systems/c1/accounts/Default/jetstream/streams/ORDERS");
    expect(
      eventGenomeFindingHref(
        {
          subject: "x",
          suggested: "y",
          genome: "x",
          cluster: [],
          stream: "ORDERS",
          consumer: "worker",
          reasons: [],
        },
        "c1",
      ),
    ).toBe("/systems/c1/accounts/Default/jetstream/streams/ORDERS/consumers/worker");
  });

  it("returns null without stream", () => {
    expect(
      eventGenomeFindingHref({ subject: "x", suggested: "y", genome: "x", cluster: [], reasons: [] }, "c1"),
    ).toBeNull();
  });
});

describe("eventGenomeFindingLabel", () => {
  it("joins stream consumer subject", () => {
    expect(
      eventGenomeFindingLabel({
        stream: "ORDERS",
        consumer: "worker",
        subject: "orders.new",
        suggested: "orders.order.created",
        genome: "order.created",
        cluster: [],
        reasons: [],
      }),
    ).toBe("ORDERS · worker · orders.new");
  });
});

describe("eventGenomeCatalogHref", () => {
  it("links to event catalog with query", () => {
    expect(eventGenomeCatalogHref("orders.new")).toBe("/docs/event-catalog?q=orders.new");
  });
});

describe("isFromGenome", () => {
  it("detects genome location state", () => {
    expect(isFromGenome(GENOME_LOCATION_STATE)).toBe(true);
    expect(isFromGenome({ from: "naming" })).toBe(false);
    expect(isFromGenome(null)).toBe(false);
  });
});

describe("GENOME_TOPOLOGY_HREF", () => {
  it("points at genome view", () => {
    expect(GENOME_TOPOLOGY_HREF).toBe("/admin/topology?view=genome");
  });
});

describe("fetchEventGenome", () => {
  beforeEach(() => {
    mockedApi.mockReset();
  });

  it("normalizes empty payload", async () => {
    mockedApi.mockResolvedValueOnce({ data: {} });
    const snap = await fetchEventGenome("cid", { fresh: true });
    expect(mockedApi).toHaveBeenCalledWith("/api/v1/clusters/cid/event-genome?fresh=1");
    expect(snap.findings).toEqual([]);
    expect(snap.totals.total).toBe(0);
  });

  it("passes through findings", async () => {
    mockedApi.mockResolvedValueOnce({
      data: {
        findings: [
          {
            subject: "orders.new",
            suggested: "orders.order.created",
            genome: "order.created",
            cluster: ["orders.new", "orders.created"],
            reasons: ["action_synonym"],
          },
        ],
        totals: { clusters: 1, duplicates: 1, total: 1 },
      },
    });
    const snap = await fetchEventGenome("cid");
    expect(snap.findings).toHaveLength(1);
    expect(snap.totals.clusters).toBe(1);
  });
});
