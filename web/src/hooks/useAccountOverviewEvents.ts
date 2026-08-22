import { useEffect, useRef, useState } from "react";
import { AccountInfo, getAccountOverviewEventsURL } from "../lib/api";
import type { RequestReplySnapshot } from "../lib/requestReplyInspector";
import { clusterQueryKey, queryClient } from "../lib/query";

const RECONNECT_BASE_MS = 1_000;
const RECONNECT_MAX_MS = 30_000;
const SSE_THROTTLE_MS = 200;

type AccountVarzLite = {
  connections?: number;
  in_msgs?: number;
  in_bytes?: number;
};

type AccountOverviewPayload = {
  account?: AccountInfo;
  requestReply?: RequestReplySnapshot;
  varz?: AccountVarzLite;
};

function overviewFingerprint(payload: AccountOverviewPayload): string {
  const a = payload.account;
  const rr = payload.requestReply;
  const v = payload.varz;
  const rrConns = Array.isArray(rr?.connections) ? rr.connections : [];
  const rrMembership = rrConns
    .map((c) => {
      const row = c as { cid?: number; name?: string };
      return `${row.cid ?? ""}:${row.name ?? ""}`;
    })
    .sort()
    .join(",");
  return [
    a?.streams ?? "",
    a?.consumers ?? "",
    a?.storage ?? "",
    a?.memory ?? "",
    v?.connections ?? "",
    v?.in_msgs ?? "",
    v?.in_bytes ?? "",
    rr?.requesters ?? "",
    rr?.responders ?? "",
    rr?.medianRttMs ?? "",
    rrConns.length,
    rrMembership,
  ].join("|");
}

/**
 * Subscribes to demand-driven Accounts / Account Overview SSE and writes into React Query.
 * Mount from Accounts or Account Overview so the backend scrapes while those pages are open.
 */
export function useAccountOverviewEvents(clusterId: string | null): { live: boolean } {
  const [live, setLive] = useState(false);
  const attemptRef = useRef(0);
  const esRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pendingRef = useRef<AccountOverviewPayload | null>(null);
  const throttleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastFpRef = useRef("");

  useEffect(() => {
    if (!clusterId) {
      setLive(false);
      return;
    }

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

    const applyPayload = (payload: AccountOverviewPayload) => {
      const fp = overviewFingerprint(payload);
      if (fp === lastFpRef.current) return;
      lastFpRef.current = fp;
      if (payload.account) {
        queryClient.setQueryData(clusterQueryKey(clusterId, "account"), payload.account);
      }
      if (payload.requestReply) {
        queryClient.setQueryData(clusterQueryKey(clusterId, "request-reply"), payload.requestReply);
      }
      if (payload.varz) {
        queryClient.setQueryData(clusterQueryKey(clusterId, "varz-lite"), payload.varz);
      }
    };

    const flushPending = () => {
      throttleTimerRef.current = null;
      const payload = pendingRef.current;
      pendingRef.current = null;
      if (payload) applyPayload(payload);
    };

    const enqueuePayload = (payload: AccountOverviewPayload) => {
      pendingRef.current = payload;
      if (throttleTimerRef.current) return;
      throttleTimerRef.current = setTimeout(flushPending, SSE_THROTTLE_MS);
    };

    const invalidateRest = () => {
      void queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "account") });
      void queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "request-reply") });
      void queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "varz-lite") });
    };

    const connect = () => {
      if (cancelled) return;
      if (typeof document !== "undefined" && document.visibilityState === "hidden") return;

      closeSource();
      const es = new EventSource(getAccountOverviewEventsURL(clusterId));
      esRef.current = es;

      es.addEventListener("account-overview", (ev) => {
        attemptRef.current = 0;
        const raw = (ev as MessageEvent).data;
        if (typeof raw !== "string" || !raw) return;
        try {
          enqueuePayload(JSON.parse(raw) as AccountOverviewPayload);
          if (!cancelled) setLive(true);
        } catch {
          // Ignore malformed frames; next event will retry.
        }
      });

      es.onopen = () => {
        attemptRef.current = 0;
        // Do not set live on open — wait for a parseable frame.
      };

      es.onerror = () => {
        if (!cancelled) setLive(false);
        closeSource();
        if (cancelled) return;
        if (typeof document !== "undefined" && document.visibilityState === "hidden") return;
        // Fall back to REST while SSE is down so gauges do not freeze.
        invalidateRest();
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
      pendingRef.current = null;
      closeSource();
      setLive(false);
    };
  }, [clusterId]);

  return { live };
}
