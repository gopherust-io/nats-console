import { useEffect, useRef } from "react";
import { getSnapshotEventsURL } from "../lib/api";
import { queryClient } from "../lib/query";

const RECONNECT_BASE_MS = 1_000;
const RECONNECT_MAX_MS = 30_000;
const INVALIDATE_DEBOUNCE_MS = 1_500;

const SNAPSHOT_SUFFIXES = new Set([
  "topology",
  "account",
  "streams",
  "kv",
  "objects",
  "object-buckets",
  "varz-lite",
]);

function shouldInvalidateOnSnapshot(queryKey: readonly unknown[], clusterId: string): boolean {
  // Connz / alerts: freshness is owned elsewhere (SSE pages or alert polling).
  if (queryKey[0] === "clusters" && queryKey[1] === "connections") return true;
  if (queryKey[0] !== "cluster" || queryKey[1] !== clusterId) return false;
  const suffix = String(queryKey[2] ?? "");
  if (suffix === "connz" || suffix === "connz-subs") return false;
  if (SNAPSHOT_SUFFIXES.has(suffix)) return true;
  if (suffix.startsWith("metrics-history:")) return true;
  return false;
}

function invalidateSnapshotQueries(clusterId: string) {
  void queryClient.invalidateQueries({
    predicate: (query) => query.isActive() && shouldInvalidateOnSnapshot(query.queryKey, clusterId),
  });
}

/**
 * Subscribes to cluster snapshot SSE and invalidates monitoring / JetStream
 * React Query caches when a new snapshot is published.
 */
export function useSnapshotEvents(clusterId: string | null) {
  const attemptRef = useRef(0);
  const esRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!clusterId) return;

    let cancelled = false;

    const clearReconnect = () => {
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
    };

    const clearDebounce = () => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
        debounceTimerRef.current = null;
      }
    };

    const closeSource = () => {
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
    };

    const scheduleInvalidate = () => {
      if (debounceTimerRef.current) return;
      debounceTimerRef.current = setTimeout(() => {
        debounceTimerRef.current = null;
        if (!cancelled) invalidateSnapshotQueries(clusterId);
      }, INVALIDATE_DEBOUNCE_MS);
    };

    const connect = () => {
      if (cancelled) return;
      if (typeof document !== "undefined" && document.visibilityState === "hidden") return;

      closeSource();
      const es = new EventSource(getSnapshotEventsURL(clusterId));
      esRef.current = es;

      es.addEventListener("snapshot", () => {
        attemptRef.current = 0;
        scheduleInvalidate();
      });

      es.onopen = () => {
        attemptRef.current = 0;
      };

      es.onerror = () => {
        closeSource();
        if (cancelled) return;
        if (typeof document !== "undefined" && document.visibilityState === "hidden") return;
        const attempt = attemptRef.current++;
        const delay = Math.min(RECONNECT_MAX_MS, RECONNECT_BASE_MS * 2 ** attempt);
        clearReconnect();
        reconnectTimerRef.current = setTimeout(connect, delay);
      };
    };

    const onVisibility = () => {
      if (document.visibilityState === "hidden") {
        clearReconnect();
        clearDebounce();
        closeSource();
        return;
      }
      attemptRef.current = 0;
      connect();
    };

    connect();
    document.addEventListener("visibilitychange", onVisibility);

    return () => {
      cancelled = true;
      document.removeEventListener("visibilitychange", onVisibility);
      clearReconnect();
      clearDebounce();
      closeSource();
    };
  }, [clusterId]);
}
