import { STORAGE_KEYS } from "./constants";

const AUTH_STORAGE_KEY = STORAGE_KEYS.auth;
const CLUSTER_STORAGE_KEY = STORAGE_KEYS.cluster;

export function getAuthHeader(): string | undefined {
  const value = localStorage.getItem(AUTH_STORAGE_KEY);
  return value ?? undefined;
}

export function setAuth(username: string, password: string) {
  localStorage.setItem(AUTH_STORAGE_KEY, `Basic ${btoa(`${username}:${password}`)}`);
}

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

export function getCSRFToken(): string | undefined {
  const match = document.cookie.match(/(?:^|;\s*)nats_consol_csrf=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : undefined;
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body) {
    headers.set("Content-Type", "application/json");
  }

  const auth = getAuthHeader();
  if (auth) {
    headers.set("Authorization", auth);
  }

  const method = (init.method ?? "GET").toUpperCase();
  if (method !== "GET" && method !== "HEAD" && method !== "OPTIONS") {
    const csrf = getCSRFToken();
    if (csrf) {
      headers.set("X-CSRF-Token", csrf);
    }
  }

  const response = await fetch(path, { ...init, headers, credentials: "include" });
  if (response.status === 401) {
    clearAuth();
    if (!window.location.pathname.startsWith("/login")) {
      window.location.href = "/login";
    }
    throw new UnauthorizedError();
  }
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error ?? `Request failed (${response.status})`);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
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

export type ClusterListResponse = {
  clusters: Cluster[];
  total: number;
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
};

export type ReplayConsumerRequest = {
  mode?: "reset" | "sidecar";
  from: "seq" | "time" | "beginning" | "new";
  seq?: number;
  time?: string;
  replayPolicy?: "instant" | "original";
  filterSubject?: string;
  durable?: string;
};

export type ReplayConsumerResult = {
  durable: string;
  mode: "reset" | "sidecar";
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
    return atob(data);
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

export function getWebSocketURL(clusterId: string, stream: string, subject?: string, fromSeq?: number): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  const params = new URLSearchParams({ stream });
  if (subject) params.set("subject", subject);
  if (fromSeq) params.set("fromSeq", String(fromSeq));
  // Auth uses the session cookie (credentials included by the browser); never put Basic in the query string.
  return `${proto}//${window.location.host}/api/v1/clusters/${encodeURIComponent(clusterId)}/live/ws?${params}`;
}
