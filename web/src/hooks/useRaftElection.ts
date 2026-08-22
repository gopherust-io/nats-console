import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useReducedMotion } from "motion/react";
import {
  applyVisualRoles,
  diffLeaderChange,
  electionCaptionKey,
  planElectionSequence,
  planOptimisticElection,
  planSettleFromCandidate,
  type ElectionOverlay,
  type ElectionPhase,
  type ElectionStep,
  type RaftElectionPeer,
  type RaftVisualRole,
} from "../lib/raftElection";

export type UseRaftElectionResult = {
  visualRoles: Record<string, RaftVisualRole>;
  phase: ElectionPhase;
  captionKey: string;
  captionParams: { from?: string; to?: string; candidate?: string };
};

export function useRaftElection(
  peers: RaftElectionPeer[],
  jetstreamLeader: string | undefined,
  clusterId?: string | null,
): UseRaftElectionResult {
  const reduceMotion = Boolean(useReducedMotion());
  const [overlay, setOverlay] = useState<ElectionOverlay | null>(null);
  const prevLeaderRef = useRef<string | undefined>(undefined);
  const primedRef = useRef(false);
  const busyRef = useRef(false);
  const optimisticRef = useRef(false);
  const unreachableForRef = useRef<string | undefined>(undefined);
  const timersRef = useRef<number[]>([]);
  const peersRef = useRef(peers);
  peersRef.current = peers;

  const clearTimers = useCallback(() => {
    for (const id of timersRef.current) window.clearTimeout(id);
    timersRef.current = [];
  }, []);

  const stopSequence = useCallback(() => {
    clearTimers();
    busyRef.current = false;
  }, [clearTimers]);

  const runSteps = useCallback(
    (steps: ElectionStep[], opts?: { force?: boolean }) => {
      if (busyRef.current && !opts?.force) return;
      if (steps.length === 0) return;

      stopSequence();
      busyRef.current = true;

      if (reduceMotion) {
        const last = steps[steps.length - 1]!;
        setOverlay({
          phase: last.optimistic ? "candidate" : "settled",
          fromLeader: last.fromLeader,
          toLeader: last.toLeader,
          candidate: last.candidate,
          optimistic: last.optimistic,
        });
        if (last.optimistic) return;
        const id = window.setTimeout(() => {
          setOverlay(null);
          busyRef.current = false;
          optimisticRef.current = false;
        }, 600);
        timersRef.current.push(id);
        return;
      }

      let elapsed = 0;
      for (const step of steps) {
        const delay = elapsed;
        const id = window.setTimeout(() => {
          setOverlay({
            phase: step.phase,
            fromLeader: step.fromLeader,
            toLeader: step.toLeader,
            candidate: step.candidate,
            optimistic: step.optimistic,
          });
          if (step.phase === "settled") {
            const clearId = window.setTimeout(() => {
              setOverlay(null);
              busyRef.current = false;
              optimisticRef.current = false;
            }, 900);
            timersRef.current.push(clearId);
          }
        }, delay);
        timersRef.current.push(id);
        elapsed += step.holdMs;
      }
    },
    [reduceMotion, stopSequence],
  );

  const startOptimisticElection = useCallback(
    (fromLeader: string) => {
      const steps = planOptimisticElection(peersRef.current, fromLeader);
      if (!steps) {
        optimisticRef.current = false;
        stopSequence();
        setOverlay({ phase: "leaderUnreachable", fromLeader });
        return;
      }
      optimisticRef.current = true;
      runSteps(steps, { force: true });
    },
    [runSteps, stopSequence],
  );

  useEffect(() => () => clearTimers(), [clearTimers]);

  useEffect(() => {
    primedRef.current = false;
    prevLeaderRef.current = undefined;
    unreachableForRef.current = undefined;
    optimisticRef.current = false;
    stopSequence();
    setOverlay(null);
  }, [clusterId, stopSequence]);

  useEffect(() => {
    const next = (jetstreamLeader ?? "").trim() || undefined;
    const peersNow = peersRef.current;

    if (!primedRef.current) {
      // Wait for first snapshot (peers and/or leader) so cold load does not
      // treat "" → firstLeader as a fake election.
      if (peersNow.length === 0 && !next) return;
      primedRef.current = true;
      prevLeaderRef.current = next;
      return;
    }

    const prev = prevLeaderRef.current;

    if (prev !== next) {
      const wasOptimistic = optimisticRef.current;
      // First discovery of a meta leader (not recovering from outage) is not an election.
      if (!prev && next && !wasOptimistic && !unreachableForRef.current) {
        prevLeaderRef.current = next;
        return;
      }

      const diff = diffLeaderChange(prev, next, peersNow);
      prevLeaderRef.current = next;
      unreachableForRef.current = undefined;

      if (diff.kind === "change" && diff.next) {
        if (wasOptimistic) {
          optimisticRef.current = false;
          runSteps(planSettleFromCandidate(diff.old || undefined, diff.next), { force: true });
        } else {
          optimisticRef.current = false;
          runSteps(planElectionSequence(peersNow, diff.old || undefined, diff.next), {
            force: true,
          });
        }
      } else if (diff.kind === "lost") {
        unreachableForRef.current = diff.old;
        startOptimisticElection(diff.old);
      } else {
        optimisticRef.current = false;
        stopSequence();
        setOverlay(null);
      }
      return;
    }

    // Same reported meta leader, but peer is missing/offline — open election
    // without naming a winner until jetstreamLeader flips.
    if (next) {
      const leaderPeer = peersNow.find((p) => p.name === next);
      if (!leaderPeer || !leaderPeer.online) {
        if (unreachableForRef.current !== next) {
          unreachableForRef.current = next;
          startOptimisticElection(next);
        }
      } else if (unreachableForRef.current === next) {
        // Clear leaderUnreachable and optimistic wait once the leader is reachable again.
        unreachableForRef.current = undefined;
        optimisticRef.current = false;
        stopSequence();
        setOverlay(null);
      }
    }
  }, [jetstreamLeader, peers, runSteps, startOptimisticElection, stopSequence]);

  const visualRoles = useMemo(
    () => applyVisualRoles(peers, jetstreamLeader, overlay),
    [peers, jetstreamLeader, overlay],
  );

  const phase: ElectionPhase = overlay?.phase ?? "stable";

  return {
    visualRoles,
    phase,
    captionKey: electionCaptionKey(overlay),
    captionParams: {
      from: overlay?.fromLeader,
      to: overlay?.toLeader,
      candidate: overlay?.candidate,
    },
  };
}
