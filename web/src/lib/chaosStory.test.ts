import { describe, expect, it } from "vitest";
import { actDurationMs, demoChaosStory, nextChaosActIndex } from "./chaosStory";

describe("nextChaosActIndex", () => {
  it("advances until the last act", () => {
    expect(nextChaosActIndex(0, 3)).toEqual({ next: 1, done: false });
    expect(nextChaosActIndex(2, 3)).toEqual({ next: 2, done: true });
    expect(nextChaosActIndex(0, 0).done).toBe(true);
  });
});

describe("demoChaosStory", () => {
  it("has Black Friday multi-act shape", () => {
    const story = demoChaosStory();
    expect(story.demo).toBe(true);
    expect(story.acts.length).toBeGreaterThanOrEqual(3);
    expect(story.summary).toMatch(/Black Friday/);
  });
});

describe("actDurationMs", () => {
  it("clamps duration", () => {
    expect(actDurationMs({ title: "", description: "", kind: "", durationSec: 5 })).toBe(5000);
    expect(actDurationMs(undefined)).toBe(5000);
  });
});
