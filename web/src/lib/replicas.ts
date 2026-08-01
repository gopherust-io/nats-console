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

/** Prefer the snapshot with the later capturedAt; equal/missing keeps incoming. */
export function isReplicasSnapshotNewer(
  incoming: ReplicasSnapshot,
  previous: ReplicasSnapshot | undefined,
): boolean {
  if (!previous?.capturedAt) return true;
  if (!incoming.capturedAt) return true;
  const next = Date.parse(incoming.capturedAt);
  const prev = Date.parse(previous.capturedAt);
  if (Number.isNaN(next) || Number.isNaN(prev)) return true;
  return next >= prev;
}
