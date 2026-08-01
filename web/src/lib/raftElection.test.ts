import { describe, expect, it } from "vitest";
import {
  applyVisualRoles,
  diffLeaderChange,
  pickCandidate,
  pickSimulateTarget,
  planElectionSequence,
} from "./raftElection";

const peers = [
  { name: "nats-1", online: true, leader: true },
  { name: "nats-2", online: true },
  { name: "nats-3", online: true },
  { name: "nats-4", online: false },
  { name: "nats-5", online: true },
];

describe("diffLeaderChange", () => {
  it("returns none when unchanged", () => {
    expect(diffLeaderChange("nats-1", "nats-1", peers)).toEqual({ kind: "none" });
    expect(diffLeaderChange(undefined, undefined, peers)).toEqual({ kind: "none" });
  });

  it("detects a leader change with preferred candidate", () => {
    expect(diffLeaderChange("nats-1", "nats-3", peers)).toEqual({
      kind: "change",
      old: "nats-1",
      next: "nats-3",
      candidate: "nats-3",
    });
  });

  it("detects lost leader", () => {
    expect(diffLeaderChange("nats-1", "", peers)).toEqual({ kind: "lost", old: "nats-1" });
  });

  it("treats first-seen leader as change from empty", () => {
    const d = diffLeaderChange(undefined, "nats-2", peers);
    expect(d).toEqual({ kind: "change", old: "", next: "nats-2", candidate: "nats-2" });
  });
});

describe("pickCandidate", () => {
  it("prefers the nominated next leader when online", () => {
    expect(pickCandidate(peers, "nats-1", "nats-5")).toBe("nats-5");
  });

  it("skips offline preferred and picks first standby", () => {
    expect(pickCandidate(peers, "nats-1", "nats-4")).toBe("nats-2");
  });
});

describe("planElectionSequence", () => {
  it("builds demote → candidate → promote → settled", () => {
    const steps = planElectionSequence(peers, "nats-1", "nats-3");
    expect(steps.map((s) => s.phase)).toEqual(["demoting", "candidate", "promoting", "settled"]);
    expect(steps[1]?.candidate).toBe("nats-3");
    expect(steps[3]?.toLeader).toBe("nats-3");
  });
});

describe("pickSimulateTarget", () => {
  it("picks an online non-leader", () => {
    expect(pickSimulateTarget(peers, "nats-1")).toEqual({ from: "nats-1", to: "nats-2" });
  });

  it("returns null with fewer than two online peers", () => {
    expect(pickSimulateTarget([{ name: "only", online: true, leader: true }], "only")).toBeNull();
  });
});

describe("applyVisualRoles", () => {
  it("marks leader, hot standby, and offline in stable state", () => {
    const roles = applyVisualRoles(peers, "nats-1");
    expect(roles["nats-1"]).toBe("leader");
    expect(roles["nats-2"]).toBe("hotStandby");
    expect(roles["nats-4"]).toBe("offline");
  });

  it("marks lagging peers as follower instead of hot standby", () => {
    const lagging = [
      { name: "nats-1", online: true, leader: true, current: true },
      { name: "nats-2", online: true, current: false },
      { name: "nats-3", online: true, current: true },
    ];
    const roles = applyVisualRoles(lagging, "nats-1");
    expect(roles["nats-2"]).toBe("follower");
    expect(roles["nats-3"]).toBe("hotStandby");
  });

  it("shows candidate during candidate phase", () => {
    const roles = applyVisualRoles(peers, "nats-1", {
      phase: "candidate",
      fromLeader: "nats-1",
      toLeader: "nats-3",
      candidate: "nats-3",
    });
    expect(roles["nats-1"]).toBe("hotStandby");
    expect(roles["nats-3"]).toBe("candidate");
  });

  it("promotes candidate to leader in promoting phase", () => {
    const roles = applyVisualRoles(peers, "nats-1", {
      phase: "promoting",
      fromLeader: "nats-1",
      toLeader: "nats-3",
      candidate: "nats-3",
    });
    expect(roles["nats-3"]).toBe("leader");
    expect(roles["nats-1"]).toBe("hotStandby");
  });
});
