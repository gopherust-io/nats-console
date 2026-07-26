import { describe, expect, it } from "vitest";
import type { StreamConfig } from "./api";
import {
  recommendFiltersLabel,
  recommendedConsumerFromStream,
  recommendedDurableName,
  recommendedFilterSubjects,
} from "./consumerRecommend";

function baseStream(over: Partial<StreamConfig> = {}): StreamConfig {
  return {
    name: "ORDERS",
    retention: "limits",
    storage: "file",
    subjects: ["orders.>"],
    ...over,
  };
}

describe("consumerRecommend", () => {
  it("builds durable name from stream", () => {
    expect(recommendedDurableName("ORDERS")).toBe("ORDERS-worker");
    expect(recommendedDurableName("my stream!")).toBe("my-stream-worker");
  });

  it("uses concrete subjects as filters and skips catch-all >", () => {
    expect(recommendedFilterSubjects(baseStream())).toEqual(["orders.>"]);
    expect(recommendedFilterSubjects(baseStream({ subjects: [">"] }))).toEqual([]);
    expect(recommendedFilterSubjects(baseStream({ subjects: undefined, mirror: { name: "SRC" } }))).toEqual(
      [],
    );
  });

  it("recommends durable pull with explicit ack", () => {
    const rec = recommendedConsumerFromStream(baseStream());
    expect(rec.durableName).toBe("ORDERS-worker");
    expect(rec.deliverSubject).toBe("");
    expect(rec.deliverPolicy).toBe("all");
    expect(rec.ackPolicy).toBe("explicit");
    expect(rec.filterSubjects).toEqual(["orders.>"]);
    expect(rec.ackWaitNs).toBe(30_000_000_000);
    expect(rec.maxDeliver).toBe(5);
  });

  it("uses none ack when stream has noAck", () => {
    expect(recommendedConsumerFromStream(baseStream({ noAck: true })).ackPolicy).toBe("none");
  });

  it("copies stream consumer limits when set", () => {
    const rec = recommendedConsumerFromStream(
      baseStream({
        consumerLimits: { maxAckPending: 250, inactiveThreshold: 3_600_000_000_000 },
      }),
    );
    expect(rec.maxAckPending).toBe(250);
    expect(rec.inactiveThresholdNs).toBe(3_600_000_000_000);
  });

  it("formats filter label", () => {
    expect(recommendFiltersLabel(["a", "b"])).toBe("a, b");
    expect(recommendFiltersLabel([])).toBe("");
  });
});
