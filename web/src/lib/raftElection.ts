export type RaftVisualRole = "follower" | "candidate" | "leader" | "offline";

export type RaftElectionPeer = {
  name: string;
  online: boolean;
  leader?: boolean;
  /** JetStream meta current flag; false means lagging (Status shows “not current”). */
  current?: boolean;
};

export type LeaderDiff =
  | { kind: "none" }
  | { kind: "change"; old: string; next: string; candidate: string }
  | { kind: "lost"; old: string };

export type ElectionPhase =
  | "stable"
  | "demoting"
  | "candidate"
  | "promoting"
  | "settled"
  | "leaderUnreachable";

export type ElectionOverlay = {
  phase: ElectionPhase;
  fromLeader?: string;
  toLeader?: string;
  candidate?: string;
  /** True while waiting for JetStream meta after leader went offline. */
  optimistic?: boolean;
};

export type ElectionStep = {
  phase: ElectionPhase;
  /** Delay before advancing to the next step (ms). */
  holdMs: number;
  fromLeader?: string;
  toLeader?: string;
  candidate?: string;
  optimistic?: boolean;
};

/** Timings for a full live/simulate sequence (~1.5s). */
export const ELECTION_HOLD_MS = {
  demoting: 350,
  candidate: 650,
  promoting: 450,
} as const;

export function pickCandidate(
  peers: RaftElectionPeer[],
  fromLeader: string | undefined,
  preferred?: string,
): string | undefined {
  if (preferred) {
    const hit = peers.find((p) => p.name === preferred && p.online && p.name !== fromLeader);
    if (hit) return hit.name;
  }
  const standby = peers.find((p) => p.online && p.name !== fromLeader);
  return standby?.name;
}

export function diffLeaderChange(
  prevLeader: string | undefined,
  nextLeader: string | undefined,
  peers: RaftElectionPeer[],
): LeaderDiff {
  const prev = (prevLeader ?? "").trim();
  const next = (nextLeader ?? "").trim();
  if (!prev && !next) return { kind: "none" };
  if (prev === next) return { kind: "none" };
  if (prev && !next) return { kind: "lost", old: prev };
  if (!prev && next) {
    const candidate = pickCandidate(peers, undefined, next) ?? next;
    return { kind: "change", old: "", next, candidate };
  }
  const candidate = pickCandidate(peers, prev, next) ?? next;
  return { kind: "change", old: prev, next, candidate };
}

export function planElectionSequence(
  _peers: RaftElectionPeer[],
  fromLeader: string | undefined,
  toLeader: string,
): ElectionStep[] {
  // Always promote the meta winner — never a guessed online standby.
  return [
    {
      phase: "demoting",
      holdMs: ELECTION_HOLD_MS.demoting,
      fromLeader,
      toLeader,
      candidate: toLeader,
    },
    {
      phase: "candidate",
      holdMs: ELECTION_HOLD_MS.candidate,
      fromLeader,
      toLeader,
      candidate: toLeader,
    },
    {
      phase: "promoting",
      holdMs: ELECTION_HOLD_MS.promoting,
      fromLeader,
      toLeader,
      candidate: toLeader,
    },
    {
      phase: "settled",
      holdMs: 0,
      fromLeader,
      toLeader,
      candidate: toLeader,
    },
  ];
}

/**
 * Start election theater as soon as the leader goes offline.
 * Do not invent a provisional winner — NATS may elect any online peer.
 * Hold until JetStream meta reports the real leader (planSettleFromCandidate).
 */
export function planOptimisticElection(
  peers: RaftElectionPeer[],
  fromLeader: string | undefined,
): ElectionStep[] | null {
  const hasStandby = peers.some((p) => p.online && p.name !== fromLeader);
  if (!hasStandby) return null;
  return [
    {
      phase: "demoting",
      holdMs: ELECTION_HOLD_MS.demoting,
      fromLeader,
      optimistic: true,
    },
    {
      phase: "candidate",
      holdMs: 0,
      fromLeader,
      optimistic: true,
    },
  ];
}

/** Finish an optimistic election once meta publishes the real leader. */
export function planSettleFromCandidate(
  fromLeader: string | undefined,
  toLeader: string,
): ElectionStep[] {
  return [
    {
      phase: "promoting",
      holdMs: ELECTION_HOLD_MS.promoting,
      fromLeader,
      toLeader,
      candidate: toLeader,
    },
    {
      phase: "settled",
      holdMs: 0,
      fromLeader,
      toLeader,
      candidate: toLeader,
    },
  ];
}

/** Choose a simulate target: first online non-leader, falling back to any other online peer. */
export function pickSimulateTarget(
  peers: RaftElectionPeer[],
  currentLeader: string | undefined,
): { from: string | undefined; to: string } | null {
  const online = peers.filter((p) => p.online);
  if (online.length < 2) return null;
  const from = currentLeader || online.find((p) => p.leader)?.name || online[0]?.name;
  const to = online.find((p) => p.name !== from)?.name;
  if (!to) return null;
  return { from, to };
}

export function applyVisualRoles(
  peers: RaftElectionPeer[],
  leader: string | undefined,
  overlay?: ElectionOverlay | null,
): Record<string, RaftVisualRole> {
  const roles: Record<string, RaftVisualRole> = {};
  const phase = overlay?.phase ?? "stable";
  const effectiveLeader =
    phase === "demoting" || phase === "candidate"
      ? undefined
      : phase === "leaderUnreachable"
        ? leader
        : (overlay?.toLeader ?? leader);

  for (const peer of peers) {
    if (!peer.online) {
      roles[peer.name] = "offline";
      continue;
    }
    // Optimistic wait: every online peer is campaigning — do not invent a winner.
    if (phase === "candidate" && overlay?.optimistic) {
      roles[peer.name] = "candidate";
      continue;
    }
    if (phase === "candidate" || phase === "promoting") {
      if (overlay?.candidate && peer.name === overlay.candidate) {
        roles[peer.name] = phase === "candidate" ? "candidate" : "leader";
        continue;
      }
    }
    if (effectiveLeader && peer.name === effectiveLeader) {
      roles[peer.name] = "leader";
      continue;
    }
    roles[peer.name] = "follower";
  }

  // During promoting/settled ensure toLeader is leader even if not in overlay.candidate path.
  if ((phase === "promoting" || phase === "settled" || phase === "stable") && effectiveLeader) {
    if (roles[effectiveLeader] && roles[effectiveLeader] !== "offline") {
      roles[effectiveLeader] = "leader";
    }
  }

  return roles;
}

export function electionCaptionKey(overlay: ElectionOverlay | null | undefined): string {
  if (!overlay || overlay.phase === "stable") return "replicas.election.captionStable";
  if (overlay.phase === "leaderUnreachable") return "replicas.election.captionLeaderUnreachable";
  if (overlay.phase === "demoting") return "replicas.election.captionDemoting";
  if (overlay.phase === "candidate") {
    return overlay.optimistic
      ? "replicas.election.captionElecting"
      : "replicas.election.captionCandidate";
  }
  if (overlay.phase === "promoting") return "replicas.election.captionPromoting";
  return "replicas.election.captionSettled";
}
