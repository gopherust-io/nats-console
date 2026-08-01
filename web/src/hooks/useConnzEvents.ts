import { useEffect, useRef } from "react";
import { getConnzEventsURL } from "../lib/api";
import { clusterQueryKey, queryClient } from "../lib/query";

const RECONNECT_BASE_MS = 1_000;
const RECONNECT_MAX_MS = 30_000;
const SSE_THROTTLE_MS = 350;

type ConnzPayload = {
  connections?: Array<{ cid?: number } & Record<string, unknown>>;
  num_connections?: number;
  total?: number;
};

function connzFingerprint(payload: ConnzPayload): string {
  const conns = payload.connections ?? [];
  let fp = `${payload.num_connections ?? payload.total ?? conns.length}:${conns.length}`;
  for (const c of conns) {
    fp += `|${c.cid ?? ""}:${c.in_msgs ?? ""}:${c.out_msgs ?? ""}:${c.rtt ?? ""}:${c.pending_bytes ?? ""}`;
  }
  return fp;
}

/**
 * Subscribes to demand-driven connz SSE and writes payloads into the React Query cache.
 * Mount only from the Connections page so the backend scrapes while viewers are present.
 */
export function useConnzEvents(clusterId: string | null) {
  const attemptRef = useRef(0);
  const esRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pendingRef = useRef<ConnzPayload | null>(null);
  const throttleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastFpRef = useRef("");

  useEffect(() => {
    if (!clusterId) return;

    let cancelled = false;
    lastFpRef.current = "";
    pendingRef.current = null;

    const clearReconnect = () => {
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
    };

    const clearThrottle = () => {
      if (throttleTimerRef.current) {
        clearTimeout(throttleTimerRef.current);
        throttleTimerRef.current = null;
      }
    };

    const closeSource = () => {
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
    };

    const applyPayload = (payload: ConnzPayload) => {
      const fp = connzFingerprint(payload);
      if (fp === lastFpRef.current) return;
      lastFpRef.current = fp;
      queryClient.setQueryData(clusterQueryKey(clusterId, "connz"), payload);
    };

    const flushPending = () => {
      throttleTimerRef.current = null;
      const payload = pendingRef.current;
      pendingRef.current = null;
      if (payload) applyPayload(payload);
    };

    const enqueuePayload = (payload: ConnzPayload) => {
      pendingRef.current = payload;
      if (throttleTimerRef.current) return;
      throttleTimerRef.current = setTimeout(flushPending, SSE_THROTTLE_MS);
    };

    const connect = () => {
      if (cancelled) return;
      if (typeof document !== "undefined" && document.visibilityState === "hidden") return;

      closeSource();
      const es = new EventSource(getConnzEventsURL(clusterId));
      esRef.current = es;

      es.addEventListener("connz", (ev) => {
        attemptRef.current = 0;
        const raw = (ev as MessageEvent).data;
        if (typeof raw !== "string" || !raw) return;
        try {
          enqueuePayload(JSON.parse(raw) as ConnzPayload);
        } catch {
          // Ignore malformed frames; next event will retry.
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
        clearThrottle();
        pendingRef.current = null;
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
      clearThrottle();
      pendingRef.current = null;
      closeSource();
    };
  }, [clusterId]);
}
