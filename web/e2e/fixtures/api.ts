import { CLUSTER } from "./cluster";

export const emptyAccount = {
  streams: 0,
  consumers: 0,
  storage: 0,
  memory: 0,
  limits: {
    maxMemory: -1,
    maxStorage: -1,
    maxStreams: -1,
    maxConsumers: -1,
  },
};

export const emptyStreams = { streams: [], total: 0 };
export const emptyBuckets = { buckets: [], total: 0 };
export const emptyConsumers = { consumers: [], total: 0, offset: 0, limit: 50 };
export const emptyKeys = { keys: [], total: 0, offset: 0, limit: 50 };
export const emptyObjects = { objects: [], total: 0, offset: 0, limit: 50 };
export const emptyGrants = { grants: [] };
export const emptyPeople = { users: [], total: 0 };
export const emptyAlerts = { alerts: [], total: 0 };
export const emptyAudit = { entries: [], total: 0 };
export const emptyRules = { rules: [], total: 0 };
export const emptyExports = { exports: [], total: 0 };
export const emptyNatsUsers = { users: [], total: 0 };
export const emptySigningGroups = { groups: [], total: 0 };

export const alertRuleMetrics = {
  metrics: ["server.cpu_percent", "server.connections"],
  comparators: ["gt", "gte", "lt", "lte"],
  severities: ["info", "warning", "critical"],
};

export const emptyVarz = {
  connections: 0,
  in_msgs: 0,
  in_bytes: 0,
  out_msgs: 0,
  out_bytes: 0,
};

export const emptyConnz = {
  connections: [],
  num_connections: 0,
};

export const emptyJsz = {
  account_details: [],
  total: { streams: 0, consumers: 0 },
};

export const emptyMetricsHistory = {
  clusterId: CLUSTER.id,
  from: "2024-01-01T00:00:00Z",
  to: "2024-01-01T01:00:00Z",
  series: [],
};

export const connectedStatus = {
  connections: [{ clusterId: CLUSTER.id, connected: true }],
  total: 1,
};

export function sampleStream(name = "ORDERS") {
  return {
    config: {
      name,
      subjects: ["orders.>"],
      retention: "limits",
      storage: "file",
      discard: "old",
      replicas: 1,
    },
    state: {
      messages: 0,
      bytes: 0,
      firstSeq: 0,
      lastSeq: 0,
      consumerCount: 0,
    },
  };
}

export function sampleConsumer(name = "worker", streamName = "ORDERS") {
  return {
    name,
    streamName,
    config: {
      durableName: name,
      deliverPolicy: "all",
      ackPolicy: "explicit",
      replayPolicy: "instant",
    },
    numPending: 0,
    numAckPending: 0,
  };
}

export function sampleKVBucket(bucket = "CONFIG") {
  return {
    bucket,
    values: 0,
    history: 1,
    storage: "file",
  };
}

export function sampleObjectBucket(bucket = "BLOBS") {
  return {
    bucket,
    size: 0,
    storage: "file",
  };
}

export function sampleKVEntry(bucket = "CONFIG", key = "feature.flag") {
  return {
    bucket,
    key,
    value: btoa(JSON.stringify({ enabled: true })),
    revision: 1,
    created: "2024-01-01T00:00:00Z",
  };
}

export function sampleAlert(id = "alert-1") {
  return {
    id,
    ruleId: "rule-1",
    clusterId: CLUSTER.id,
    status: "open",
    severity: "warning",
    metric: "server.cpu_percent",
    message: "CPU high",
    firingValue: 95,
    threshold: 90,
    firstSeenAt: "2024-01-01T00:00:00Z",
    lastSeenAt: "2024-01-01T00:05:00Z",
    ruleName: "High CPU",
  };
}

export function samplePerson(id = "user-2", username = "alice") {
  return {
    id,
    username,
    email: `${username}@example.com`,
    roles: ["viewer"],
    createdAt: "2024-01-01T00:00:00Z",
  };
}
