import { describe, expect, it, beforeEach } from "vitest";
import { STORAGE_KEYS } from "./constants";
import {
  isFavoriteStream,
  readFavoriteStreams,
  sortStreamsFavoritesFirst,
  toggleFavoriteStream,
  writeFavoriteStreams,
} from "./favoriteStreams";

function stream(name: string, messages = 0, consumerCount = 0) {
  return { config: { name }, state: { messages, consumerCount } };
}

describe("favoriteStreams", () => {
  beforeEach(() => {
    localStorage.removeItem(STORAGE_KEYS.favoriteStreams);
  });

  it("toggles favorites by cluster and stream", () => {
    expect(readFavoriteStreams()).toEqual([]);
    toggleFavoriteStream("c1", "ORDERS");
    expect(isFavoriteStream("c1", "ORDERS")).toBe(true);
    expect(isFavoriteStream("c2", "ORDERS")).toBe(false);
    toggleFavoriteStream("c1", "ORDERS");
    expect(isFavoriteStream("c1", "ORDERS")).toBe(false);
  });

  it("ignores corrupt storage", () => {
    localStorage.setItem(STORAGE_KEYS.favoriteStreams, "{not-json");
    expect(readFavoriteStreams()).toEqual([]);
    writeFavoriteStreams([{ clusterId: "c1", streamName: "A" }]);
    expect(readFavoriteStreams()).toEqual([{ clusterId: "c1", streamName: "A" }]);
  });

  it("sorts favorites first, then by messages, then consumers", () => {
    const streams = [
      stream("ALPHA", 10, 1),
      stream("BRAVO", 50, 2),
      stream("CHARLIE", 100, 1),
      stream("DELTA", 100, 5),
      stream("ECHO", 5, 9),
    ];
    const favorites = [
      { clusterId: "c1", streamName: "ALPHA" },
      { clusterId: "c1", streamName: "CHARLIE" },
      { clusterId: "c1", streamName: "DELTA" },
      { clusterId: "c2", streamName: "BRAVO" },
    ];

    expect(sortStreamsFavoritesFirst(streams, "c1", favorites).map((s) => s.config.name)).toEqual([
      "DELTA", // favorite, 100 msgs, 5 consumers
      "CHARLIE", // favorite, 100 msgs, 1 consumer
      "ALPHA", // favorite, 10 msgs
      "BRAVO", // not favorite, 50 msgs
      "ECHO", // not favorite, 5 msgs
    ]);
  });

  it("sorts by messages then consumers when there are no favorites", () => {
    const streams = [stream("B", 1, 9), stream("A", 10, 1), stream("C", 10, 3)];
    expect(sortStreamsFavoritesFirst(streams, "c1", []).map((s) => s.config.name)).toEqual([
      "C",
      "A",
      "B",
    ]);
  });
});
