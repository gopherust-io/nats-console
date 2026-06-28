const STORAGE_KEY = "nats-consol-auth";
const CLUSTER_KEY = "nats-consol-cluster";

export function getAuthHeader(): string | undefined {
  const value = localStorage.getItem(STORAGE_KEY);
  return value ?? undefined;
}

export function setAuth(username: string, password: string) {
  localStorage.setItem(STORAGE_KEY, `Basic ${btoa(`${username}:${password}`)}`);
}

export function clearAuth() {
  localStorage.removeItem(STORAGE_KEY);
}

export function getSelectedClusterId(): string | null {
  return localStorage.getItem(CLUSTER_KEY);
}

export function setSelectedClusterId(id: string) {
  localStorage.setItem(CLUSTER_KEY, id);
}

export class UnauthorizedError extends Error {
  constructor() {
    super("Unauthorized");
    this.name = "UnauthorizedError";
  }
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
  nats_url: string;
  monitoring_url: string;
  has_creds: boolean;
  has_token: boolean;
  is_default: boolean;
  created_at: string;
  updated_at: string;
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
  cluster_id: string;
  resource_type: string;
  resource_name: string;
  request_id: string;
  details: Record<string, unknown>;
  ip: string;
};

export type UserRecord = {
  id: string;
  username: string;
  email: string;
  roles: string[];
  created_at: string;
};

export type AccountInfo = {
  memory: number;
  storage: number;
  streams: number;
  consumers: number;
  limits: {
    max_memory: number;
    max_storage: number;
    max_streams: number;
    max_consumers: number;
  };
};

export type StreamInfo = {
  config: {
    name: string;
    subjects?: string[];
    retention: string;
    storage: string;
    max_msgs?: number;
    max_bytes?: number;
    max_age?: number;
  };
  state: {
    messages: number;
    bytes: number;
    first_seq: number;
    last_seq: number;
    consumer_count: number;
  };
};

export type ConsumerInfo = {
  name: string;
  stream_name: string;
  config: {
    durable_name?: string;
    deliver_policy: string;
    ack_policy: string;
    filter_subject?: string;
  };
  num_pending: number;
  num_ack_pending: number;
  delivered?: {
    consumer_seq: number;
    stream_seq: number;
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
    prev_seq?: number;
    next_seq?: number;
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
  if (fromSeq) params.set("from_seq", String(fromSeq));
  const auth = getAuthHeader();
  if (auth) params.set("authorization", auth);
  return `${proto}//${window.location.host}/api/v1/clusters/${encodeURIComponent(clusterId)}/live/ws?${params}`;
}
