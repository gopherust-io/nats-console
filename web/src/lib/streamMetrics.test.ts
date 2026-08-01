import { describe, expect, it } from "vitest";
import { streamMetric, streamRateMetricsCSV, StreamMetricKind } from "./streamMetrics";

describe("streamMetrics", () => {
  it("builds metric names", () => {
    expect(streamMetric("ORDERS", StreamMetricKind.LastSeq)).toBe("stream:ORDERS:last_seq");
  });

  it("builds CSV for rate charts", () => {
    expect(streamRateMetricsCSV("ORDERS")).toBe(
      "stream:ORDERS:last_seq,stream:ORDERS:delivered_seq,stream:ORDERS:ack_floor_seq,stream:ORDERS:bytes",
    );
  });
});
