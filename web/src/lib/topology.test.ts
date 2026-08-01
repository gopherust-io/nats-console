import { describe, expect, it } from "vitest";
import { filterTopology, type TopologyNode } from "./topology";

function buildTree(): TopologyNode {
  return {
    id: "cluster:root",
    kind: "cluster",
    name: "Cluster",
    children: [
      {
        id: "stream:ORDERS",
        kind: "stream",
        name: "ORDERS",
        meta: ["12 msgs"],
        children: [
          { id: "subject:ORDERS:new.subject", kind: "subject", name: "new.subject", children: [] },
          { id: "consumer:ORDERS:billing-worker", kind: "consumer", name: "billing-worker", children: [] },
        ],
      },
      {
        id: "stream:PAYMENTS",
        kind: "stream",
        name: "PAYMENTS",
        meta: ["3 msgs"],
        children: [
          { id: "subject:PAYMENTS:payments.new", kind: "subject", name: "payments.new", children: [] },
        ],
      },
    ],
  };
}

describe("filterTopology", () => {
  it("returns the tree unchanged when the filter is empty", () => {
    const tree = buildTree();
    expect(filterTopology(tree, "")).toBe(tree);
  });

  it("keeps a self-matching node and expands its full children", () => {
    // "ORDERS" matches the stream's own name; consumers/subjects stay visible.
    const result = filterTopology(buildTree(), "ORDERS");
    expect(result).not.toBeNull();
    const streams = result!.children.filter((n) => n.kind === "stream");
    expect(streams).toHaveLength(1);
    expect(streams[0].name).toBe("ORDERS");
    expect(streams[0].children.map((c) => c.name)).toEqual(["new.subject", "billing-worker"]);
  });

  it("includes a node whose only match is a descendant", () => {
    const result = filterTopology(buildTree(), "payments.new");
    expect(result).not.toBeNull();
    const streams = result!.children.filter((n) => n.kind === "stream");
    expect(streams).toHaveLength(1);
    expect(streams[0].name).toBe("PAYMENTS");
    expect(streams[0].children.map((c) => c.name)).toEqual(["payments.new"]);
  });

  it("returns null when nothing matches", () => {
    expect(filterTopology(buildTree(), "no-such-thing")).toBeNull();
  });
});
