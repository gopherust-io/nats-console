export type ReplicaPeer = {
  name: string;
  role: string;
  online: boolean;
  current?: boolean;
  leader?: boolean;
  uptime?: string;
  rtt?: string;
  idle?: string;
  version?: string;
  ip?: string;
  inMsgs?: number;
  outMsgs?: number;
  pending?: number;
  connections?: number;
  cpu?: number;
  mem?: number;
};

export type ReplicasSnapshot = {
  capturedAt?: string;
  clusterName?: string;
  monitoredServer?: string;
  jetstreamLeader?: string;
  clusterSize: number;
  peerCount: number;
  onlineCount: number;
  peers: ReplicaPeer[];
};

/**
 * Prefer newer capturedAt when both sides are stamped.
 * SSE payloads omit capturedAt — accept them over prior data, but do not let a
 * stamped REST scrape clobber a live untimestamped SSE snapshot.
 */
export function isReplicasSnapshotNewer(
  incoming: ReplicasSnapshot,
  previous: ReplicasSnapshot | undefined,
): boolean {
  if (!previous) return true;
  if (!incoming.capturedAt && previous.capturedAt) return true;
  if (incoming.capturedAt && !previous.capturedAt) return false;
  if (!incoming.capturedAt && !previous.capturedAt) return true;
  const next = Date.parse(incoming.capturedAt!);
  const prev = Date.parse(previous.capturedAt!);
  if (Number.isNaN(next) || Number.isNaN(prev)) return true;
  return next >= prev;
}
