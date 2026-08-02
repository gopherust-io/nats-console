import { STORAGE_KEYS } from "./constants";
import { safeDecodeURIComponent } from "./safeDecode";

const AUTH_STORAGE_KEY = STORAGE_KEYS.auth;
const CLUSTER_STORAGE_KEY = STORAGE_KEYS.cluster;

/**
 * Auth is session-cookie based (set by the server on login); we never keep
 * credentials in localStorage. clearAuth() only exists to wipe any legacy
 * `Basic ...` value left over from older builds.
 */
export function clearAuth() {
  localStorage.removeItem(AUTH_STORAGE_KEY);
}

export function getSelectedClusterId(): string | null {
  return localStorage.getItem(CLUSTER_STORAGE_KEY);
}

export function setSelectedClusterId(id: string) {
  localStorage.setItem(CLUSTER_STORAGE_KEY, id);
}

export class UnauthorizedError extends Error {
  constructor() {
    super("Unauthorized");
    this.name = "UnauthorizedError";
  }
}

export type ApiErrorCode =
  | "not_found"
  | "forbidden"
  | "unauthorized"
  | "validation"
  | "conflict"
  | "timeout"
  | "unavailable"
  | "internal"
  | "rate_limit"
  | "csrf_invalid"
  | "gone"
  | "network"
  | "unknown";

export class ApiError extends Error {
  status: number;
  code: ApiErrorCode;
  retryable: boolean;
  retryAfterSeconds?: number;

  constructor(
    message: string,
    options: {
      status?: number;
      code?: ApiErrorCode;
      retryable?: boolean;
      retryAfterSeconds?: number;
    } = {},
  ) {
    super(message);
    this.name = "ApiError";
    this.status = options.status ?? 0;
    this.code = options.code ?? "unknown";
    this.retryable = options.retryable ?? false;
    this.retryAfterSeconds = options.retryAfterSeconds;
  }
}

export type ApiMeta = { total: number; offset?: number; limit?: number };
export type ApiResult<T> = { data: T; meta?: ApiMeta };

type ErrorBody = {
  error?: {
    message?: string;
    code?: string;
    retryable?: boolean;
    retryAfterSeconds?: number;
  };
};

export function codeFromStatus(status: number): ApiErrorCode {
  if (status === 404) return "not_found";
  if (status === 403) return "forbidden";
  if (status === 401) return "unauthorized";
  if (status === 409) return "conflict";
  if (status === 410) return "gone";
  if (status === 408 || status === 504) return "timeout";
  if (status === 429) return "rate_limit";
  if (status === 502 || status === 503) return "unavailable";
  if (status === 400 || status === 422 || (status >= 400 && status < 500)) return "validation";
  if (status >= 500) return "internal";
  return "unknown";
}

function normalizeCode(raw: string | undefined, status: number): ApiErrorCode {
  const known: ApiErrorCode[] = [
    "not_found",
    "forbidden",
    "unauthorized",
    "validation",
    "conflict",
    "timeout",
    "unavailable",
    "internal",
    "rate_limit",
    "csrf_invalid",
    "gone",
    "network",
  ];
  if (raw && (known as string[]).includes(raw)) {
    return raw as ApiErrorCode;
  }
  return codeFromStatus(status);
}

export function getCSRFToken(): string | undefined {
  const match = document.cookie.match(/(?:^|;\s*)nats_consol_csrf=([^;]*)/);
  return match ? safeDecodeURIComponent(match[1]) : undefined;
}

let refreshInFlight: Promise<boolean> | null = null;

async function tryRefreshSession(): Promise<boolean> {
  if (refreshInFlight) {
    return refreshInFlight;
  }
  refreshInFlight = (async () => {
    try {
      const headers = new Headers();
      const csrf = getCSRFToken();
      if (csrf) {
        headers.set("X-CSRF-Token", csrf);
      }
      const response = await fetch("/api/v1/auth/refresh", {
        method: "POST",
        headers,
        credentials: "include",
      });
      return response.ok;
    } catch {
      return false;
    } finally {
      refreshInFlight = null;
    }
  })();
  return refreshInFlight;
}

function isAuthBootstrapPath(path: string): boolean {
  return (
    path.startsWith("/api/v1/auth/login") ||
    path.startsWith("/api/v1/auth/logout") ||
    path.startsWith("/api/v1/auth/refresh") ||
    path.startsWith("/api/v1/auth/config") ||
    path.startsWith("/api/v1/auth/invite")
  );
}

/** Compress request bodies only when strictly larger than 32 KiB. */
const REQUEST_COMPRESS_MIN_BYTES = 32 * 1024;

type RequestCompressFormat = { stream: CompressionFormat; encoding: "br" | "gzip" };

function preferredRequestCompressFormats(): RequestCompressFormat[] {
  // Prefer brotli for large JSON; fall back to gzip where CompressionStream("brotli") is unsupported.
  const formats: RequestCompressFormat[] = [
    { stream: "brotli" as CompressionFormat, encoding: "br" },
    { stream: "gzip", encoding: "gzip" },
  ];
  return formats.filter((f) => {
    try {
      // Probe constructor support without retaining the stream.
      new CompressionStream(f.stream);
      return true;
    } catch {
      return false;
    }
  });
}

async function compressJSONBody(body: string, format: CompressionFormat): Promise<ArrayBuffer> {
  const encoded = new TextEncoder().encode(body);
  // Cast: DOM lib types CompressionStream.writable as BufferSource, which conflicts with Uint8Array streams.
  const transform = new CompressionStream(format) as unknown as TransformStream<Uint8Array, Uint8Array>;
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(encoded);
      controller.close();
    },
  }).pipeThrough(transform);
  return new Response(stream).arrayBuffer();
}

async function maybeCompressJSONBody(
  body: BodyInit | null | undefined,
  headers: Headers,
): Promise<BodyInit | null | undefined> {
  if (typeof body !== "string") {
    return body;
  }
  const plainBytes = new TextEncoder().encode(body);
  // Small payloads: skip compression — overhead often outweighs savings.
  if (plainBytes.byteLength <= REQUEST_COMPRESS_MIN_BYTES) {
    return body;
  }
  if (typeof CompressionStream === "undefined") {
    return body;
  }
  for (const format of preferredRequestCompressFormats()) {
    try {
      const compressed = await compressJSONBody(body, format.stream);
      // Only send compressed if it actually shrinks the wire payload.
      if (compressed.byteLength >= plainBytes.byteLength) {
        continue;
      }
      headers.set("Content-Encoding", format.encoding);
      return compressed;
    } catch {
      // try next format
    }
  }
  return body;
}

async function buildAPIRequest(init: RequestInit = {}): Promise<{ headers: Headers; body: BodyInit | null | undefined }> {
  const headers = new Headers(init.headers);
  if (init.body) {
    headers.set("Content-Type", "application/json");
  }

  const method = (init.method ?? "GET").toUpperCase();
  if (method !== "GET" && method !== "HEAD" && method !== "OPTIONS") {
    const csrf = getCSRFToken();
    if (csrf) {
      headers.set("X-CSRF-Token", csrf);
    }
  }

  const body = await maybeCompressJSONBody(init.body, headers);
  return { headers, body };
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<ApiResult<T>> {
  const req = await buildAPIRequest(init);

  let response: Response;
  try {
    response = await fetch(path, { ...init, headers: req.headers, body: req.body, credentials: "include" });
  } catch {
    throw new ApiError("Network request failed. Check your connection and try again.", {
      status: 0,
      code: "network",
    });
  }

  if (response.status === 401 && !isAuthBootstrapPath(path)) {
    const refreshed = await tryRefreshSession();
    if (refreshed) {
      const retry = await buildAPIRequest(init);
      try {
        response = await fetch(path, { ...init, headers: retry.headers, body: retry.body, credentials: "include" });
      } catch {
        throw new ApiError("Network request failed. Check your connection and try again.", {
          status: 0,
          code: "network",
        });
      }
    }
  }

  if (response.status === 401) {
    clearAuth();
    if (!window.location.pathname.startsWith("/login")) {
      window.location.href = "/login";
    }
    throw new UnauthorizedError();
  }
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as ErrorBody;
    const nested = body.error;
    const retryable =
      nested?.retryable === true || response.status === 429 || response.status === 502 || response.status === 503 || response.status === 504;
    throw new ApiError(nested?.message ?? `Request failed (${response.status})`, {
      status: response.status,
      code: normalizeCode(nested?.code, response.status),
      retryable,
      retryAfterSeconds: nested?.retryAfterSeconds,
    });
  }
  if (response.status === 204) {
    return { data: undefined as T };
  }
  const body = (await response.json()) as { data?: T; meta?: ApiMeta };
  // Monitoring proxy endpoints return raw NATS JSON (no envelope).
  if (body && typeof body === "object" && "data" in body) {
    return { data: body.data as T, meta: body.meta };
  }
  return { data: body as T };
}

export function errorMessage(err: unknown, fallback = "Request failed"): string {
  if (err instanceof Error && err.message) return err.message;
  return fallback;
}

type TranslateFn = (key: string, options?: Record<string, unknown>) => string;

/** Map ApiError codes to actionable i18n copy; falls back to the error message. */
export function userFacingError(err: unknown, t: TranslateFn): string {
  if (err instanceof ApiError) {
    const key = `errors.${err.code}`;
    const translated = t(key);
    if (translated && translated !== key) {
      return translated;
    }
    if (err.message) return err.message;
  }
  return errorMessage(err, t("errors.requestFailed"));
}

export function clusterPath(clusterId: string, suffix: string): string {
  return `/api/v1/clusters/${encodeURIComponent(clusterId)}${suffix}`;
}

/** Account-scoped JetStream UI base path (not the API). */
export function jetStreamUIBase(clusterId: string, accountName = "Default"): string {
  return `/systems/${clusterId}/accounts/${encodeURIComponent(accountName || "Default")}/jetstream`;
}

export type Cluster = {
  id: string;
  name: string;
  natsUrl: string;
  monitoringUrl: string;
  hasCreds: boolean;
  hasToken: boolean;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
};

export type AuditEntry = {
  id: string;
  timestamp: string;
  actor: string;
  action: string;
  clusterId: string;
  resourceType: string;
  resourceName: string;
  requestId: string;
  details: Record<string, unknown>;
  ip: string;
};

export type AccessRules = {
  clusterIds?: string[];
  manageUsers: boolean;
  viewAudit: boolean;
  deleteClusters: boolean;
  assignableRoles?: string[];
};

export type UserRecord = {
  id: string;
  username: string;
  email: string;
  roles: string[];
  isRoot?: boolean;
  accessRules?: AccessRules;
  grants?: Array<{
    id: string;
    userId: string;
    resourceType: string;
    resourceKey: string;
    role: string;
  }>;
  createdAt: string;
};

export type AccountInfo = {
  memory: number;
  storage: number;
  streams: number;
  consumers: number;
  limits: {
    maxMemory: number;
    maxStorage: number;
    maxStreams: number;
    maxConsumers: number;
  };
};

export type StreamPlacement = {
  cluster?: string;
  tags?: string[];
};

export type StreamSource = {
  name: string;
  filterSubject?: string;
  optStartSeq?: number;
  optStartTime?: string;
  external?: { api?: string; deliver?: string };
};

export type StreamConfig = {
  name: string;
  description?: string;
  subjects?: string[];
  retention: string;
  storage: string;
  discard?: string;
  compression?: string;
  maxMsgs?: number;
  maxBytes?: number;
  maxAge?: number;
  maxConsumers?: number;
  maxMsgSize?: number;
  maxMsgsPerSubject?: number;
  replicas?: number;
  duplicates?: number;
  firstSeq?: number;
  subjectDeleteMarkerTTL?: number;
  allowRollup?: boolean;
  denyDelete?: boolean;
  denyPurge?: boolean;
  discardNewPerSubject?: boolean;
  noAck?: boolean;
  sealed?: boolean;
  allowDirect?: boolean;
  mirrorDirect?: boolean;
  allowMsgTTL?: boolean;
  placement?: StreamPlacement;
  mirror?: StreamSource;
  sources?: StreamSource[];
  subjectTransform?: { src?: string; dest: string };
  republish?: { src?: string; dest: string; headersOnly?: boolean };
  consumerLimits?: { inactiveThreshold?: number; maxAckPending?: number };
  metadata?: Record<string, string>;
};

export type StreamInfo = {
  config: StreamConfig;
  state: {
    messages: number;
    bytes: number;
    firstSeq: number;
    lastSeq: number;
    consumerCount: number;
  };
  isDlq?: boolean;
};

/** Pre-delete impact summary from GET .../streams/{name}/impact */
export type BlastRadius = {
  stream: string;
  services: number;
  streams: number;
  consumers: number;
  critical: string[];
  serviceNames: string[];
  relatedStreams: string[];
  consumerNames: string[];
};

export type DLQMessage = {
  seq: number;
  subject: string;
  time: string;
  data: string;
  headers?: Record<string, string>;
  reason?: string;
  originalSubject?: string;
  sourceStream?: string;
  sourceSeq?: number;
  consumer?: string;
  numDelivered?: number;
  autopsyError?: string;
  autopsyHash?: string;
  autopsyStack?: string;
};

export type DLQListResult = {
  messages: DLQMessage[];
  truncated?: boolean;
  nextSeq?: number;
};

export type DLQRetryRequest = {
  seqs?: number[];
  all?: boolean;
  limit?: number;
};

export type DLQRetryResult = {
  retried: number;
  failed?: { seq: number; error: string }[];
  truncated?: boolean;
  remaining?: number;
};

export type ConsumerConfig = {
  durableName?: string;
  name?: string;
  description?: string;
  deliverPolicy: string;
  ackPolicy: string;
  replayPolicy?: string;
  filterSubject?: string;
  filterSubjects?: string[];
  optStartSeq?: number;
  optStartTime?: string;
  ackWaitNs?: number;
  maxDeliver?: number;
  backoffNs?: number[];
  maxAckPending?: number;
  rateLimitBps?: number;
  sampleFreq?: string;
  maxWaiting?: number;
  inactiveThresholdNs?: number;
  maxRequestBatch?: number;
  maxRequestExpiresNs?: number;
  maxRequestMaxBytes?: number;
  deliverSubject?: string;
  deliverGroup?: string;
  flowControl?: boolean;
  heartbeatNs?: number;
  headersOnly?: boolean;
  replicas?: number;
  memoryStorage?: boolean;
  metadata?: Record<string, string>;
};

export type ConsumerInfo = {
  name: string;
  streamName: string;
  config: ConsumerConfig;
  numPending: number;
  numAckPending: number;
  numRedelivered?: number;
  numWaiting?: number;
  created?: string;
  delivered?: {
    consumerSeq: number;
    streamSeq: number;
  };
  ackFloor?: {
    consumerSeq: number;
    streamSeq: number;
  };
  slowConsumer?: boolean;
  slowReasons?: Array<"pending" | "lag" | "ack_pending">;
};

export type ReplayConsumerRequest = {
  mode?: "reset" | "sidecar";
  from: "seq" | "time" | "beginning" | "new";
  seq?: number;
  time?: string;
  untilSeq?: number;
  untilTime?: string;
  limit?: number;
  replayPolicy?: "instant" | "original";
  filterSubject?: string;
  durable?: string;
};

export type ReplayConsumerResult = {
  durable: string;
  mode: "reset" | "sidecar";
  startSeq?: number;
  untilSeq?: number;
  limit?: number;
  startTime?: string;
  untilTime?: string;
};

/** Pre-replay impact estimate from POST .../consumers/{c}/replay/dry-run */
export type ReplayDryRun = {
  messages: number;
  estimatedDurationMs: number;
  consumersAffected: number;
  potentialDuplicates: string[];
  unbounded?: boolean;
  approximate?: boolean;
};

/** Worker-reported behavior fingerprint from GET .../behavior-fingerprint */
export type BehaviorFingerprintSnapshot = {
  msgPerMin: number;
  processingMs: number;
};

export type BehaviorFingerprintReport = {
  available: boolean;
  stream?: string;
  durable?: string;
  anomaly?: boolean;
  normal?: BehaviorFingerprintSnapshot;
  current?: BehaviorFingerprintSnapshot;
  sustainedForMs?: number;
  updatedAt?: string;
};

/** Auto-generated incident timeline from GET .../incident-reconstruction */
export type IncidentTimelineEvent = {
  at: string;
  category: string;
  label: string;
  source: string;
  evidence?: string;
};

export type IncidentReconstruction = {
  clusterId: string;
  stream: string;
  consumer: string;
  from: string;
  to: string;
  events: IncidentTimelineEvent[];
  eventCount: number;
  usedDeployAnnotations?: boolean;
  usedAuditFallback?: boolean;
};

export type RawMessage = {
  message: {
    seq: number;
    subject: string;
    time: string;
    data: string;
    headers?: Record<string, string>;
  };
  navigation?: {
    prevSeq?: number;
    nextSeq?: number;
  };
};

export type KVBucketInfo = {
  bucket: string;
  description?: string;
  storage?: string;
  values: number;
  bytes?: number;
  history: number;
  ttlNs?: number;
  limitMarkerTTLNs?: number;
  maxValueSize?: number;
  maxBytes?: number;
  replicas?: number;
  compressed?: boolean;
  placement?: {
    cluster?: string;
    tags?: string[];
  };
  republish?: { src?: string; dest: string; headersOnly?: boolean };
  mirror?: { name: string; filterSubject?: string };
  sources?: { name: string; filterSubject?: string }[];
  metadata?: Record<string, string>;
};

export type KVEntry = {
  bucket: string;
  key: string;
  value: string;
  revision: number;
  created: string;
};

export type ObjectBucketInfo = {
  bucket: string;
  description?: string;
  storage?: string;
  size: number;
  ttlNs?: number;
  maxBytes?: number;
  replicas?: number;
  compressed?: boolean;
  sealed?: boolean;
  placement?: {
    cluster?: string;
    tags?: string[];
  };
  metadata?: Record<string, string>;
};

export type ObjectInfo = {
  bucket: string;
  name: string;
  size: number;
  data: string;
  modified: string;
};

export function decodeBase64(data: string): string {
  try {
    const binary = atob(data);
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
    return new TextDecoder("utf-8", { fatal: false }).decode(bytes);
  } catch {
    return data;
  }
}

export function tryParseJSON(data: string): { parsed: unknown; isJSON: boolean } {
  try {
    return { parsed: JSON.parse(data), isJSON: true };
  } catch {
    return { parsed: data, isJSON: false };
  }
}

/** Pretty-print JSON payloads; leave non-JSON text unchanged. */
export function formatMessagePayload(decoded: string): string {
  const { parsed, isJSON } = tryParseJSON(decoded);
  if (!isJSON) return decoded;
  return JSON.stringify(parsed, null, 2);
}

export function getWebSocketURL(clusterId: string, stream: string, subject?: string, fromSeq?: number): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  const params = new URLSearchParams({ stream });
  if (subject) params.set("subject", subject);
  if (fromSeq) params.set("fromSeq", String(fromSeq));
  // Auth uses the session cookie (credentials included by the browser); never put Basic in the query string.
  return `${proto}//${window.location.host}/api/v1/clusters/${encodeURIComponent(clusterId)}/live/ws?${params}`;
}

/** Same-origin SSE URL; session cookie is sent automatically by EventSource. */
export function getSnapshotEventsURL(clusterId: string): string {
  return `/api/v1/clusters/${encodeURIComponent(clusterId)}/snapshots/events`;
}

/** Demand-driven connz SSE; session cookie is sent automatically by EventSource. */
export function getConnzEventsURL(clusterId: string): string {
  return `/api/v1/clusters/${encodeURIComponent(clusterId)}/monitoring/connz/events`;
}

/** Demand-driven replicas SSE; session cookie is sent automatically by EventSource. */
export function getReplicasEventsURL(clusterId: string): string {
  return `/api/v1/clusters/${encodeURIComponent(clusterId)}/replicas/events`;
}
