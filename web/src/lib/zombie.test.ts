import { describe, expect, it } from "vitest";
import {
  groupZombiesByKind,
  isFromZombies,
  sortZombieFindings,
  zombieFindingHref,
  zombieFindingLabel,
  ZOMBIES_LOCATION_STATE,
  type ZombieFinding,
} from "./zombie";

const samples: ZombieFinding[] = [
  { kind: "unpublished_subject", stream: "A", subject: "a.>", reasons: ["empty_stream"] },
  { kind: "empty_stream", stream: "B", reasons: ["never_received"] },
  { kind: "idle_consumer", stream: "A", consumer: "w", reasons: ["zero_delivered"] },
];

describe("sortZombieFindings", () => {
  it("orders by kind then stream", () => {
    const sorted = sortZombieFindings(samples);
    expect(sorted.map((f) => f.kind)).toEqual([
      "empty_stream",
      "idle_consumer",
      "unpublished_subject",
    ]);
  });
});

describe("groupZombiesByKind", () => {
  it("groups sorted findings", () => {
    const groups = groupZombiesByKind(samples);
    expect([...groups.keys()]).toEqual(["empty_stream", "idle_consumer", "unpublished_subject"]);
    expect(groups.get("empty_stream")).toHaveLength(1);
  });
});

describe("zombieFindingHref", () => {
  it("links stream and consumer pages", () => {
    expect(zombieFindingHref({ kind: "empty_stream", stream: "ORDERS", reasons: [] }, "c1")).toBe(
      "/systems/c1/accounts/Default/jetstream/streams/ORDERS",
    );
    expect(
      zombieFindingHref(
        { kind: "idle_consumer", stream: "ORDERS", consumer: "w", reasons: [] },
        "c1",
        "APP",
      ),
    ).toBe("/systems/c1/accounts/APP/jetstream/streams/ORDERS/consumers/w");
  });
});

describe("zombieFindingLabel", () => {
  it("joins identifying fields", () => {
    expect(
      zombieFindingLabel({
        kind: "unbound_consumer",
        stream: "S",
        consumer: "C",
        subject: "x.>",
        reasons: [],
      }),
    ).toBe("S · C · x.>");
  });
});

describe("isFromZombies", () => {
  it("detects zombie location state", () => {
    expect(isFromZombies(ZOMBIES_LOCATION_STATE)).toBe(true);
    expect(isFromZombies({ from: "topology" })).toBe(false);
    expect(isFromZombies(null)).toBe(false);
  });
});
