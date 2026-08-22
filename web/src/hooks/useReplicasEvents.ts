import { useEffect, useRef, useState } from "react";
import { getReplicasEventsURL } from "../lib/api";
import type { ReplicasSnapshot } from "../lib/replicas";
import { isReplicasSnapshotNewer } from "../lib/replicas";
import { clusterQueryKey, queryClient } from "../lib/query";

const RECONNECT_BASE_MS = 1_000;
const RECONNECT_MAX_MS = 30_000;

/**
 * Subscribes to demand-driven replicas SSE and writes snapshots into the React Query cache.
 * Mount only from the Replicas page so the backend scrapes while viewers are present.
 */
export function useReplicasEvents(clusterId: string | null): { live: boolean } {
  const [live, setLive] = useState(false);
  const attemptRef = useRef(0);
  const esRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!clusterId) {
      setLive(false);
      return;
    }

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
      const es = new EventSource(getReplicasEventsURL(clusterId));
      esRef.current = es;

      es.addEventListener("replicas", (ev) => {
        attemptRef.current = 0;
        if (!cancelled) setLive(true);
        const raw = (ev as MessageEvent).data;
        if (typeof raw !== "string" || !raw) return;
        try {
          const payload = JSON.parse(raw) as ReplicasSnapshot;
          queryClient.setQueryData<ReplicasSnapshot>(
            clusterQueryKey(clusterId, "replicas"),
            (prev) => (isReplicasSnapshotNewer(payload, prev) ? payload : prev),
          );
        } catch {
          // Ignore malformed frames; next event will retry.
        }
      });

      es.onopen = () => {
        attemptRef.current = 0;
        if (!cancelled) setLive(true);
      };

      es.onerror = () => {
        if (!cancelled) setLive(false);
        closeSource();
        if (cancelled) return;
        if (typeof document !== "undefined" && document.visibilityState === "hidden") return;
        // Fall back to REST while SSE is down so online/offline does not freeze.
        void queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "replicas") });
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
        if (!cancelled) setLive(false);
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
      setLive(false);
    };
  }, [clusterId]);

  return { live };
}
