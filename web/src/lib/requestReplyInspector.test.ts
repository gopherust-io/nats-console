import { describe, expect, it } from "vitest";
import { formatCount, formatLatencyMs } from "./requestReplyInspector";

describe("requestReplyInspector", () => {
  it("formats latency", () => {
    expect(formatLatencyMs(12.5)).toBe("12.5 ms");
    expect(formatLatencyMs(null)).toBe("—");
    expect(formatLatencyMs(0.25)).toBe("0.25 ms");
    expect(formatLatencyMs(1500)).toBe("1.50 s");
  });

  it("formats counts", () => {
    expect(formatCount(12)).toBe("12");
  });
});
