/** localStorage keys — fixed identifiers, not runtime config */
export const STORAGE_KEYS = {
  auth: "nats-consol-auth",
  cluster: "nats-consol-cluster",
  theme: "nats-consol-theme",
  sidebar: "nats-consol-sidebar-open",
  assistantLayout: "nats-consol-assistant-layout",
} as const;

/** Default cluster form values — match backend NATS_URL / NATS_MONITORING_URL defaults */
export const DEFAULT_NATS_URL = "nats://localhost:4222";
export const DEFAULT_MONITORING_URL = "http://localhost:8222";

/** Pagination and list limits */
export const DEFAULT_PAGE_SIZE = 100;
export const TOPOLOGY_PAGE_SIZE = 500;
export const AUDIT_PAGE_LIMIT = 100;

export function pageQuery(offset: number, limit = DEFAULT_PAGE_SIZE) {
  return `?offset=${offset}&limit=${limit}`;
}

/** Live stream UI */
export const LIVE_STREAM_MAX_MESSAGES = 500;
export const LIVE_SUBJECT_FILTER_DEBOUNCE_MS = 400;

/** Assistant UI */
export const ASSISTANT_RETRY_COUNTDOWN_INTERVAL_MS = 1000;

/** React Query defaults */
export const QUERY_STALE_TIME_MS = 30_000;
export const QUERY_GC_TIME_MS = 5 * 60_000;
