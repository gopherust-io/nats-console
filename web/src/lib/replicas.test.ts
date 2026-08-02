import { describe, expect, it } from "vitest";
import { isReplicasSnapshotNewer, type ReplicasSnapshot } from "./replicas";

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

  it("does not let REST clobber untimestamped SSE", () => {
    const sse = base({ jetstreamLeader: "live" });
    const rest = base({ capturedAt: "2026-01-01T00:00:02Z", jetstreamLeader: "stale" });
    expect(isReplicasSnapshotNewer(rest, sse)).toBe(false);
  });

  it("compares stamped snapshots by time", () => {
    const older = base({ capturedAt: "2026-01-01T00:00:01Z" });
    const newer = base({ capturedAt: "2026-01-01T00:00:02Z" });
    expect(isReplicasSnapshotNewer(newer, older)).toBe(true);
    expect(isReplicasSnapshotNewer(older, newer)).toBe(false);
  });
});
