import { useEffect, useRef, useState } from "react";
import { getConnzEventsURL } from "../lib/api";
import { clusterQueryKey, queryClient } from "../lib/query";

const RECONNECT_BASE_MS = 1_000;
const RECONNECT_MAX_MS = 30_000;
/** Throttle noisy counter/RTT churn; membership changes bypass this. */
const SSE_THROTTLE_MS = 200;
/** If no connz frame arrives, treat SSE as dead so REST polling resumes. */
const SSE_STALE_MS = 20_000;

type ConnzPayload = {
  now?: string;
  connections?: Array<{ cid?: number; name?: string } & Record<string, unknown>>;
  num_connections?: number;
  total?: number;
};

/** Stable set of connection IDs — used to detect connect/disconnect immediately. */
export function connzMembershipKey(payload: ConnzPayload): string {
  const ids = (payload.connections ?? [])
    .map((c) => String(c.cid ?? ""))
    .filter(Boolean)
    .sort();
  return `${payload.num_connections ?? payload.total ?? ids.length}:${ids.join(",")}`;
}

function connzFingerprint(payload: ConnzPayload): string {
  const conns = payload.connections ?? [];
  let fp = connzMembershipKey(payload);
  for (const c of conns) {
    fp += `|${c.cid ?? ""}:${c.name ?? ""}:${c.in_msgs ?? ""}:${c.out_msgs ?? ""}:${c.rtt ?? ""}:${c.pending_bytes ?? ""}`;
  }
  return fp;
}

function connzNowMs(payload: ConnzPayload): number {
  if (!payload.now) return 0;
  const ms = Date.parse(payload.now);
  return Number.isFinite(ms) ? ms : 0;
}

/**
 * Subscribes to demand-driven connz SSE and writes payloads into the React Query cache.
 * Mount only from the Connections page so the backend scrapes while viewers are present.
 * `live` is true only after a real connz frame (not onopen) so silent/zombie streams
 * do not disable the page's REST fallback.
 */
export function useConnzEvents(clusterId: string | null): { live: boolean } {
  const [live, setLive] = useState(false);
  const attemptRef = useRef(0);
  const esRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pendingRef = useRef<ConnzPayload | null>(null);
  const throttleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const staleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastFpRef = useRef("");
  const lastMembershipRef = useRef("");
  const lastNowRef = useRef(0);

  useEffect(() => {
    if (!clusterId) {
      setLive(false);
      return;
    }

    let cancelled = false;
    lastFpRef.current = "";
    lastMembershipRef.current = "";
    lastNowRef.current = 0;
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

    const clearStale = () => {
      if (staleTimerRef.current) {
        clearTimeout(staleTimerRef.current);
        staleTimerRef.current = null;
      }
    };

    const armStale = () => {
      clearStale();
      staleTimerRef.current = setTimeout(() => {
        if (!cancelled) setLive(false);
      }, SSE_STALE_MS);
    };

    const closeSource = () => {
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
    };

    const applyPayload = (payload: ConnzPayload) => {
      const nowMs = connzNowMs(payload);
      // Ignore out-of-order frames (e.g. slow REST completing after a newer SSE).
      if (nowMs > 0 && lastNowRef.current > 0 && nowMs < lastNowRef.current) {
        return;
      }
      const fp = connzFingerprint(payload);
      if (fp === lastFpRef.current) return;
      lastFpRef.current = fp;
      lastMembershipRef.current = connzMembershipKey(payload);
      if (nowMs > 0) lastNowRef.current = nowMs;
      queryClient.setQueryData(clusterQueryKey(clusterId, "connz"), payload);
    };

    const flushPending = () => {
      throttleTimerRef.current = null;
      const payload = pendingRef.current;
      pendingRef.current = null;
      if (payload) applyPayload(payload);
    };

    const enqueuePayload = (payload: ConnzPayload) => {
      const membership = connzMembershipKey(payload);
      // Connect/disconnect: paint immediately; do not wait for the counter throttle.
      if (membership !== lastMembershipRef.current) {
        clearThrottle();
        pendingRef.current = null;
        applyPayload(payload);
        return;
      }
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
        armStale();
        const raw = (ev as MessageEvent).data;
        if (typeof raw !== "string" || !raw) return;
        try {
          enqueuePayload(JSON.parse(raw) as ConnzPayload);
          if (!cancelled) setLive(true);
        } catch {
          // Ignore malformed frames; next event will retry. Do not flip live
          // here — that would disable REST while the cache stays frozen.
        }
      });

      es.onopen = () => {
        // Do not set live here — open without frames would disable REST fallback.
        attemptRef.current = 0;
      };

      es.onerror = () => {
        clearStale();
        if (!cancelled) setLive(false);
        closeSource();
        if (cancelled) return;
        if (typeof document !== "undefined" && document.visibilityState === "hidden") return;
        // Fall back to REST while SSE is down so new connections do not freeze.
        void queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "connz") });
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
        clearStale();
        pendingRef.current = null;
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
      clearThrottle();
      clearStale();
      pendingRef.current = null;
      closeSource();
      setLive(false);
    };
  }, [clusterId]);

  return { live };
}
