export const CLUSTER = {
  id: "cluster-1",
  name: "local",
  natsUrl: "nats://localhost:4222",
  monitoringUrl: "http://localhost:8222",
  hasCreds: false,
  hasToken: false,
  isDefault: true,
  createdAt: "2024-01-01T00:00:00Z",
  updatedAt: "2024-01-01T00:00:00Z",
};

export const ACCOUNT = "Default";

export function accountBase(clusterId = CLUSTER.id, account = ACCOUNT) {
  return `/systems/${clusterId}/accounts/${encodeURIComponent(account)}`;
}

export function clusterApi(suffix: string, clusterId = CLUSTER.id) {
  return `/api/v1/clusters/${encodeURIComponent(clusterId)}${suffix}`;
}
