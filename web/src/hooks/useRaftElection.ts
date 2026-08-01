import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useReducedMotion } from "motion/react";
import {
  applyVisualRoles,
  diffLeaderChange,
  electionCaptionKey,
  pickSimulateTarget,
  planElectionSequence,
  type ElectionOverlay,
  type ElectionPhase,
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
  const provisionalForRef = useRef<string | undefined>(undefined);
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

  const runSequence = useCallback(
    (fromLeader: string | undefined, toLeader: string, opts?: { force?: boolean }) => {
      if (busyRef.current && !opts?.force) return;
      const steps = planElectionSequence(peersRef.current, fromLeader, toLeader);

      stopSequence();
      busyRef.current = true;

      if (reduceMotion) {
        setOverlay({
          phase: "settled",
          fromLeader,
          toLeader,
          candidate: toLeader,
        });
        const id = window.setTimeout(() => {
          setOverlay(null);
          busyRef.current = false;
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
          });
          if (step.phase === "settled") {
            const clearId = window.setTimeout(() => {
              setOverlay(null);
              busyRef.current = false;
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

  useEffect(() => () => clearTimers(), [clearTimers]);

  // Reset election priming when switching clusters (same route stays mounted).
  useEffect(() => {
    primedRef.current = false;
    prevLeaderRef.current = undefined;
    provisionalForRef.current = undefined;
    stopSequence();
    setOverlay(null);
  }, [clusterId, stopSequence]);

  useEffect(() => {
    const next = (jetstreamLeader ?? "").trim() || undefined;
    const peersNow = peersRef.current;

    if (!primedRef.current) {
      primedRef.current = true;
      prevLeaderRef.current = next;
      return;
    }

    const prev = prevLeaderRef.current;

    if (prev !== next) {
      const diff = diffLeaderChange(prev, next, peersNow);
      prevLeaderRef.current = next;
      provisionalForRef.current = undefined;
      if (diff.kind === "change" && diff.next) {
        // Interrupt provisional overlay so the real meta leader always wins.
        runSequence(diff.old || undefined, diff.next, { force: true });
      } else {
        stopSequence();
        setOverlay(null);
      }
      return;
    }

    // Same reported leader, but peer is missing/offline — hold a candidate
    // overlay (no fake settled leader) until JetStream publishes a new leader.
    if (next) {
      const leaderPeer = peersNow.find((p) => p.name === next);
      if (!leaderPeer || !leaderPeer.online) {
        if (provisionalForRef.current !== next) {
          const pick = pickSimulateTarget(peersNow, next);
          if (pick?.to) {
            provisionalForRef.current = next;
            setOverlay({
              phase: "candidate",
              fromLeader: next,
              toLeader: pick.to,
              candidate: pick.to,
            });
            busyRef.current = false;
            clearTimers();
          }
        }
      } else if (provisionalForRef.current === next) {
        provisionalForRef.current = undefined;
        setOverlay(null);
      }
    }
  }, [jetstreamLeader, peers, runSequence, stopSequence, clearTimers]);

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
