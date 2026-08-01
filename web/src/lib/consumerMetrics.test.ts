import { describe, expect, it } from "vitest";
import { consumerLag, formatAckWaitNs, formatSeqPair, evaluateSlowConsumer } from "./consumerMetrics";

describe("consumerLag", () => {
  it("returns zero when caught up or ahead", () => {
    expect(consumerLag(100, 100)).toBe(0);
    expect(consumerLag(50, 100)).toBe(0);
  });

  it("treats missing delivered as zero", () => {
    expect(consumerLag(42)).toBe(42);
    expect(consumerLag(0)).toBe(0);
  });

  it("computes positive lag", () => {
    expect(consumerLag(1000, 900)).toBe(100);
  });
});

describe("formatAckWaitNs", () => {
  it("shows em dash when unset", () => {
    expect(formatAckWaitNs(undefined)).toBe("—");
    expect(formatAckWaitNs(0)).toBe("—");
  });

  it("formats whole units", () => {
    expect(formatAckWaitNs(30_000_000_000)).toBe("30s");
    expect(formatAckWaitNs(60_000_000_000)).toBe("1m");
    expect(formatAckWaitNs(3_600_000_000_000)).toBe("1h");
  });
});

describe("formatSeqPair", () => {
  it("formats stream and consumer seq", () => {
    expect(formatSeqPair(10, 3)).toBe("10 / 3");
    expect(formatSeqPair()).toBe("—");
  });
});

describe("evaluateSlowConsumer", () => {
  it("detects pending lag and ack pending", () => {
    expect(evaluateSlowConsumer({ pending: 999, lag: 0, ackPending: 0 }).slow).toBe(false);
    expect(evaluateSlowConsumer({ pending: 1000, lag: 0, ackPending: 0 }).reasons).toEqual(["pending"]);
    expect(evaluateSlowConsumer({ pending: 0, lag: 1000, ackPending: 0 }).reasons).toEqual(["lag"]);
    expect(
      evaluateSlowConsumer({ pending: 0, lag: 0, ackPending: 90, maxAckPending: 100 }).reasons,
    ).toEqual(["ack_pending"]);
  });
});
