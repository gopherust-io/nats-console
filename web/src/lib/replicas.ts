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
  lag?: number;
};

export type PeerMetricScope = "routeLink" | "varzHealth";

/** True when a dash is expected for this source, not a failed scrape. */
export function isExpectedMetricNa(peer: ReplicaPeer, scope: PeerMetricScope): boolean {
  if (scope === "routeLink") return peer.role === "monitored";
  return peer.role === "route" || peer.role === "meta";
}

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
 * Otherwise accept the incoming frame: live untimestamped SSE must be able to
 * recover after a stamped all-offline REST poll (and the reverse when SSE is down).
 */
export function isReplicasSnapshotNewer(
  incoming: ReplicasSnapshot,
  previous: ReplicasSnapshot | undefined,
): boolean {
  if (!previous) return true;

  const nextTs = incoming.capturedAt ? Date.parse(incoming.capturedAt) : Number.NaN;
  const prevTs = previous.capturedAt ? Date.parse(previous.capturedAt) : Number.NaN;
  if (!Number.isNaN(nextTs) && !Number.isNaN(prevTs)) return nextTs >= prevTs;
  return true;
}

/** Stable RAFT role from monitoring — not election theater. */
export function monitoringRaftRole(
  peer: ReplicaPeer,
  jetstreamLeader?: string,
): "leader" | "follower" | null {
  if (!peer.online) return null;
  if (peer.leader || (jetstreamLeader && peer.name === jetstreamLeader)) return "leader";
  return "follower";
}
