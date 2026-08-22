import { describe, expect, it } from "vitest";
import {
  isExpectedMetricNa,
  isReplicasSnapshotNewer,
  monitoringRaftRole,
  type ReplicaPeer,
  type ReplicasSnapshot,
} from "./replicas";

const base = (partial: Partial<ReplicasSnapshot> = {}): ReplicasSnapshot => ({
  clusterSize: 1,
  peerCount: 1,
  onlineCount: 1,
  peers: [],
  ...partial,
});

describe("isReplicasSnapshotNewer", () => {
  it("accepts first snapshot", () => {
    expect(isReplicasSnapshotNewer(base({ capturedAt: "2026-01-01T00:00:01Z" }), undefined)).toBe(true);
  });

  it("accepts SSE over prior REST", () => {
    const prev = base({ capturedAt: "2026-01-01T00:00:01Z", jetstreamLeader: "old" });
    const sse = base({ jetstreamLeader: "live" });
    expect(isReplicasSnapshotNewer(sse, prev)).toBe(true);
  });

  it("lets stamped REST replace untimestamped SSE", () => {
    const sse = base({ jetstreamLeader: "live" });
    const rest = base({ capturedAt: "2026-01-01T00:00:02Z", jetstreamLeader: "stale", onlineCount: 0 });
    expect(isReplicasSnapshotNewer(rest, sse)).toBe(true);
  });

  it("lets untimestamped SSE recover after stamped all-offline", () => {
    const offline = base({ capturedAt: "2026-01-01T00:00:02Z", onlineCount: 0 });
    const sse = base({ onlineCount: 3 });
    expect(isReplicasSnapshotNewer(sse, offline)).toBe(true);
  });

  it("compares stamped snapshots by time", () => {
    const older = base({ capturedAt: "2026-01-01T00:00:01Z" });
    const newer = base({ capturedAt: "2026-01-01T00:00:02Z" });
    expect(isReplicasSnapshotNewer(newer, older)).toBe(true);
    expect(isReplicasSnapshotNewer(older, newer)).toBe(false);
  });
});

describe("monitoringRaftRole", () => {
  const peer = (partial: Partial<ReplicaPeer>): ReplicaPeer => ({
    name: "n1",
    role: "route",
    online: true,
    ...partial,
  });

  it("returns null when offline", () => {
    expect(monitoringRaftRole(peer({ online: false }), "n2")).toBeNull();
  });

  it("returns leader from jetstreamLeader or peer.leader", () => {
    expect(monitoringRaftRole(peer({ name: "n2" }), "n2")).toBe("leader");
    expect(monitoringRaftRole(peer({ leader: true }), undefined)).toBe("leader");
  });

  it("returns follower for other online peers", () => {
    expect(monitoringRaftRole(peer({ name: "n3" }), "n2")).toBe("follower");
  });
});

describe("isExpectedMetricNa", () => {
  it("marks route-link gaps on monitored server", () => {
    expect(isExpectedMetricNa({ name: "n1", role: "monitored", online: true }, "routeLink")).toBe(true);
  });

  it("marks varz gaps on route peers", () => {
    expect(isExpectedMetricNa({ name: "n2", role: "route", online: true }, "varzHealth")).toBe(true);
  });
});
