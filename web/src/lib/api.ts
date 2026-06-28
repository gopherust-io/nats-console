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

export type StreamInfo = {
  config: {
    name: string;
    subjects?: string[];
    retention: string;
    storage: string;
    maxMsgs?: number;
    maxBytes?: number;
    maxAge?: number;
  };
  state: {
    messages: number;
    bytes: number;
    firstSeq: number;
    lastSeq: number;
    consumerCount: number;
  };
};

export type ConsumerInfo = {
  name: string;
  streamName: string;
  config: {
    durableName?: string;
    deliverPolicy: string;
    ackPolicy: string;
    filterSubject?: string;
  };
  numPending: number;
  numAckPending: number;
  delivered?: {
    consumerSeq: number;
    streamSeq: number;
  };
};

export type RawMessage = {
  message: {
    seq: number;
    subject: string;
    time: string;
    data: string;
    hdrs?: number[];
  };
  navigation?: {
    prevSeq?: number;
    nextSeq?: number;
  };
};

export type KVBucketInfo = {
  bucket: string;
  values: number;
  history: number;
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
  description: string;
  size: number;
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
  const auth = getAuthHeader();
  if (auth) params.set("authorization", auth);
  return `${proto}//${window.location.host}/api/v1/clusters/${encodeURIComponent(clusterId)}/live/ws?${params}`;
}
