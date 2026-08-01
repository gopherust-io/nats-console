import { describe, expect, it, vi, beforeEach } from "vitest";
import {
  fetchSubjectNaming,
  isFromNaming,
  NAMING_LOCATION_STATE,
  NAMING_TOPOLOGY_HREF,
  sortSubjectNamingFindings,
  subjectNamingFindingHref,
  subjectNamingFindingLabel,
  type SubjectNamingFinding,
} from "./subjectNaming";

vi.mock("./api", () => ({
  api: vi.fn(),
  clusterPath: (id: string, path: string) => `/api/v1/clusters/${id}${path}`,
  jetStreamUIBase: (clusterId: string, accountName = "Default") =>
    `/systems/${clusterId}/accounts/${encodeURIComponent(accountName)}/jetstream`,
}));

import { api } from "./api";

const mockedApi = vi.mocked(api);

describe("sortSubjectNamingFindings", () => {
  it("orders by kind priority then stream/subject", () => {
    const findings: SubjectNamingFinding[] = [
      { kind: "shallow_hierarchy", stream: "B", subject: "a.b", suggested: "a.a.b", reasons: [] },
      { kind: "inconsistent_variant", stream: "A", subject: "x", suggested: "a.b.c", reasons: [] },
      { kind: "wrong_case", stream: "A", subject: "Foo.Bar", suggested: "foo.bar", reasons: [] },
    ];
    const sorted = sortSubjectNamingFindings(findings);
    expect(sorted.map((f) => f.kind)).toEqual([
      "inconsistent_variant",
      "wrong_case",
      "shallow_hierarchy",
    ]);
  });
});

describe("subjectNamingFindingHref", () => {
  it("links to stream or consumer", () => {
    expect(subjectNamingFindingHref({ kind: "wrong_case", stream: "ORDERS", subject: "x", suggested: "y", reasons: [] }, "c1")).toBe(
      "/systems/c1/accounts/Default/jetstream/streams/ORDERS",
    );
    expect(
      subjectNamingFindingHref(
        { kind: "wrong_case", stream: "ORDERS", consumer: "worker", subject: "x", suggested: "y", reasons: [] },
        "c1",
      ),
    ).toBe("/systems/c1/accounts/Default/jetstream/streams/ORDERS/consumers/worker");
  });

  it("returns null without stream", () => {
    expect(subjectNamingFindingHref({ kind: "wrong_case", subject: "x", suggested: "y", reasons: [] }, "c1")).toBeNull();
  });
});

describe("subjectNamingFindingLabel", () => {
  it("joins stream consumer subject", () => {
    expect(
      subjectNamingFindingLabel({
        kind: "wrong_case",
        stream: "ORDERS",
        consumer: "worker",
        subject: "Orders.Created",
        suggested: "orders.created",
        reasons: [],
      }),
    ).toBe("ORDERS · worker · Orders.Created");
  });
});

describe("isFromNaming", () => {
  it("detects naming location state", () => {
    expect(isFromNaming(NAMING_LOCATION_STATE)).toBe(true);
    expect(isFromNaming({ from: "zombies" })).toBe(false);
    expect(isFromNaming(null)).toBe(false);
  });
});

describe("NAMING_TOPOLOGY_HREF", () => {
  it("points at naming view", () => {
    expect(NAMING_TOPOLOGY_HREF).toBe("/admin/topology?view=naming");
  });
});

describe("fetchSubjectNaming", () => {
  beforeEach(() => {
    mockedApi.mockReset();
  });

  it("normalizes empty payload", async () => {
    mockedApi.mockResolvedValueOnce({ data: {} });
    const snap = await fetchSubjectNaming("cid", { fresh: true });
    expect(mockedApi).toHaveBeenCalledWith("/api/v1/clusters/cid/subject-naming?fresh=1");
    expect(snap.findings).toEqual([]);
    expect(snap.totals.total).toBe(0);
  });

  it("passes through findings", async () => {
    mockedApi.mockResolvedValueOnce({
      data: {
        findings: [{ kind: "wrong_case", subject: "Foo", suggested: "foo", reasons: ["uppercase"] }],
        totals: { wrongCase: 1, missingDots: 0, nonDotSeparator: 0, shallowHierarchy: 0, inconsistentVariants: 0, total: 1 },
      },
    });
    const snap = await fetchSubjectNaming("cid");
    expect(snap.findings).toHaveLength(1);
    expect(snap.totals.wrongCase).toBe(1);
  });
});
