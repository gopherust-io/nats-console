import { api, clusterPath } from "./api";

export type SuperclusterOverview = {
  serverName: string;
  clusterName?: string;
  jetstreamDomain?: string;
  gatewayEnabled: boolean;
  routeCount: number;
  leafCount: number;
  gateways: SuperclusterGateway[];
  routes: SuperclusterRoute[];
  leafnodes: SuperclusterLeafnode[];
  metaCluster?: SuperclusterMeta;
  streamReplication: StreamReplication[];
  sourceErrors?: Record<string, string>;
  warnings?: string[];
  fetchedAt: string;
};

export type SuperclusterGateway = {
  name: string;
  direction: string;
  host?: string;
  port?: number;
  connections?: number;
  subscriptions?: number;
};

export type SuperclusterRoute = {
  remoteId?: string;
  url?: string;
  connected: boolean;
  inMsgs?: number;
  outMsgs?: number;
};

export type SuperclusterLeafnode = {
  name?: string;
  remote?: string;
  connected: boolean;
  rtt?: string;
};

export type SuperclusterMeta = {
  leader?: string;
  clusterSize?: number;
  replicas?: SuperclusterReplica[];
};

export type SuperclusterReplica = {
  name: string;
  id?: string;
  leader: boolean;
  current: boolean;
  online: boolean;
  active?: string;
  lag?: number;
};

export type StreamReplication = {
  streamName: string;
  kind: string;
  targetName: string;
  targetDomain?: string;
  active: boolean;
  lag?: number;
  error?: string;
};

export function fetchSupercluster(clusterId: string) {
  return api<SuperclusterOverview>(clusterPath(clusterId, "/supercluster"));
}

export function hasSuperclusterWarnings(overview: SuperclusterOverview) {
  return (overview.warnings?.length ?? 0) > 0 || Object.keys(overview.sourceErrors ?? {}).length > 0;
}

export function hasSuperclusterFeatures(overview: SuperclusterOverview) {
  const gateways = overview.gateways ?? [];
  const routes = overview.routes ?? [];
  const leafnodes = overview.leafnodes ?? [];
  const streamReplication = overview.streamReplication ?? [];
  return (
    overview.gatewayEnabled ||
    gateways.length > 0 ||
    routes.length > 0 ||
    leafnodes.length > 0 ||
    Boolean(overview.metaCluster) ||
    streamReplication.length > 0
  );
}
