import { useEffect, useRef } from "react";
import { getConnectionEventsURL, type NATSConnectionStatus } from "../lib/api";
import { clusterQueryKey, queryClient } from "../lib/query";

const RECONNECT_BASE_MS = 1_000;
const RECONNECT_MAX_MS = 30_000;

function applyConnectionStatus(clusterId: string, status: NATSConnectionStatus) {
  queryClient.setQueryData(clusterQueryKey(clusterId, "connection"), status);
  queryClient.setQueryData<NATSConnectionStatus[]>(["clusters", "connections"], (prev) => {
    if (!prev) {
      return [{ clusterId, connected: status.connected }];
    }
    let found = false;
    const next = prev.map((item) => {
      if (item.clusterId !== clusterId) return item;
      found = true;
      return { ...item, ...status, connected: status.connected };
    });
    return found ? next : [...next, status];
  });
}

/**
 * Subscribes to connection-status SSE and writes live updates into the
 * React Query cache so Overview (and peers) reflect disconnect immediately.
 */
export function useConnectionEvents(clusterId: string | null) {
  const attemptRef = useRef(0);
  const esRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!clusterId) return;

    let cancelled = false;

    const clearReconnect = () => {
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
    };

    const closeSource = () => {
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
    };

    const connect = () => {
      if (cancelled) return;
      if (typeof document !== "undefined" && document.visibilityState === "hidden") return;

      closeSource();
      const es = new EventSource(getConnectionEventsURL(clusterId));
      esRef.current = es;

      es.addEventListener("connection", (event) => {
        attemptRef.current = 0;
        try {
          const status = JSON.parse((event as MessageEvent).data) as NATSConnectionStatus;
          if (!cancelled) applyConnectionStatus(clusterId, status);
        } catch {
          // ignore malformed frames
        }
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
      closeSource();
    };
  }, [clusterId]);
}
