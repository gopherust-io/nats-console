import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAccountOverviewEvents } from "./useAccountOverviewEvents";
import { clusterQueryKey, queryClient } from "../lib/query";

type Handler = ((ev: Event) => void) | null;

class MockEventSource {
  static instances: MockEventSource[] = [];
  onopen: Handler = null;
  onerror: Handler = null;
  listeners = new Map<string, Set<(ev: MessageEvent) => void>>();
  url: string;
  closed = false;

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: (ev: MessageEvent) => void) {
    const set = this.listeners.get(type) ?? new Set();
    set.add(listener);
    this.listeners.set(type, set);
  }

  close() {
    this.closed = true;
  }

  emit(type: string, data: string) {
    for (const listener of this.listeners.get(type) ?? []) {
      listener({ data } as MessageEvent);
    }
  }
}

describe("useAccountOverviewEvents", () => {
  const originalEventSource = globalThis.EventSource;

  beforeEach(() => {
    MockEventSource.instances = [];
    // @ts-expect-error test double
    globalThis.EventSource = MockEventSource;
    queryClient.clear();
  });

  afterEach(() => {
    globalThis.EventSource = originalEventSource;
    vi.restoreAllMocks();
  });

  it("sets live on first frame and applies account payload", async () => {
    const { result } = renderHook(() => useAccountOverviewEvents("c1"));
    expect(result.current.live).toBe(false);

    const es = MockEventSource.instances[0];
    expect(es).toBeTruthy();
    es.onopen?.(new Event("open"));
    // Open alone must not flip live — REST fallback stays enabled until a frame.
    expect(result.current.live).toBe(false);

    es.emit(
      "account-overview",
      JSON.stringify({ account: { streams: 4, consumers: 2, storage: 10, memory: 1 } }),
    );

    await waitFor(() => expect(result.current.live).toBe(true));

    await waitFor(() => {
      expect(queryClient.getQueryData(clusterQueryKey("c1", "account"))).toEqual({
        streams: 4,
        consumers: 2,
        storage: 10,
        memory: 1,
      });
    });
  });

  it("clears live and invalidates REST queries on SSE error", async () => {
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useAccountOverviewEvents("c1"));
    const es = MockEventSource.instances[0];
    es.onopen?.(new Event("open"));
    es.emit(
      "account-overview",
      JSON.stringify({ account: { streams: 1, consumers: 0, storage: 0, memory: 0 } }),
    );
    await waitFor(() => expect(result.current.live).toBe(true));

    es.onerror?.(new Event("error"));

    await waitFor(() => expect(result.current.live).toBe(false));
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: clusterQueryKey("c1", "account"),
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: clusterQueryKey("c1", "request-reply"),
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: clusterQueryKey("c1", "varz-lite"),
    });
  });
});
