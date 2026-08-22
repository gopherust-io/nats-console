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

export const emptyStreams = { data: [], meta: { total: 0 } };
export const emptyBuckets = { data: [], meta: { total: 0 } };
export const emptyConsumers = { data: [], meta: { total: 0, offset: 0, limit: 50 } };
export const emptyKeys = { data: [], meta: { total: 0, offset: 0, limit: 50 } };
export const emptyObjects = { data: [], meta: { total: 0, offset: 0, limit: 50 } };
export const emptyGrants = { data: [], meta: { total: 0 } };
export const emptyPeople = { data: [], meta: { total: 0 } };
export const emptyAlerts = { data: [], meta: { total: 0 } };
export const emptyAudit = { data: [], meta: { total: 0 } };
export const emptyRules = { data: [], meta: { total: 0 } };
export const emptyExports = { data: [], meta: { total: 0 } };
export const emptyNatsUsers = { data: [], meta: { total: 0 } };
export const emptySigningGroups = { data: [], meta: { total: 0 } };
export const emptyTopology = {
  data: {
    id: "cluster:root",
    kind: "cluster",
    name: "Cluster",
    children: [],
  },
};
export const emptyUsers = { data: [], meta: { total: 0 } };

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

export const emptyRequestReply = {
  patterns: [],
  connections: [],
  requesters: 0,
  responders: 0,
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

export const connectedStatus = [
  { clusterId: CLUSTER.id, connected: true, jetstreamOk: true, serverName: "n1" },
];

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
      ackWaitNs: 30_000_000_000,
    },
    numPending: 3,
    numAckPending: 1,
    numWaiting: 0,
    numRedelivered: 2,
    delivered: {
      consumerSeq: 10,
      streamSeq: 97,
    },
    ackFloor: {
      consumerSeq: 9,
      streamSeq: 96,
    },
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
