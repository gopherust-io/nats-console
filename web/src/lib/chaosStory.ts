import { api, clusterPath } from "./api";

export type ChaosSeverity = "info" | "warn" | "critical";

export type ChaosStoryAct = {
  title: string;
  description: string;
  kind: string;
  targets?: string[];
  durationSec: number;
};

export type ChaosStory = {
  title: string;
  setting: string;
  severity: ChaosSeverity | string;
  summary: string;
  acts: ChaosStoryAct[];
  blastRadius?: string[];
  recoveryHints?: string[];
  source: string;
  demo?: boolean;
};

export type ChaosStorySeed = {
  streams: string[];
  consumers: string[];
  subjects: string[];
};

export type ChaosStoryEnvelope = {
  story?: ChaosStory | null;
  seed: ChaosStorySeed;
};

export type ChaosStoryGenerateResult = {
  story: ChaosStory;
  seed: ChaosStorySeed;
};

const EMPTY_SEED: ChaosStorySeed = { streams: [], consumers: [], subjects: [] };

function normalizeAct(a: Partial<ChaosStoryAct> | null | undefined): ChaosStoryAct {
  return {
    title: a?.title ?? "",
    description: a?.description ?? "",
    kind: a?.kind ?? "traffic_spike",
    targets: Array.isArray(a?.targets) ? a.targets : [],
    durationSec: typeof a?.durationSec === "number" && a.durationSec > 0 ? a.durationSec : 5,
  };
}

export function normalizeChaosStory(data?: Partial<ChaosStory> | null): ChaosStory {
  return {
    title: data?.title ?? "Chaos story",
    setting: data?.setting ?? "",
    severity: data?.severity ?? "warn",
    summary: data?.summary ?? "",
    acts: Array.isArray(data?.acts) ? data.acts.map(normalizeAct) : [],
    blastRadius: Array.isArray(data?.blastRadius) ? data.blastRadius : [],
    recoveryHints: Array.isArray(data?.recoveryHints) ? data.recoveryHints : [],
    source: data?.source ?? "demo",
    demo: Boolean(data?.demo),
  };
}

function normalizeSeed(data?: Partial<ChaosStorySeed> | null): ChaosStorySeed {
  return {
    streams: Array.isArray(data?.streams) ? data.streams : [],
    consumers: Array.isArray(data?.consumers) ? data.consumers : [],
    subjects: Array.isArray(data?.subjects) ? data.subjects : [],
  };
}

/** Advance simulate playbook; mirrors domain.NextChaosActIndex. */
export function nextChaosActIndex(current: number, actCount: number): { next: number; done: boolean } {
  if (actCount <= 0) return { next: 0, done: true };
  const next = current + 1;
  if (next >= actCount) return { next: actCount - 1, done: true };
  return { next, done: false };
}

export function actDurationMs(act: ChaosStoryAct | undefined): number {
  const sec = act?.durationSec && act.durationSec > 0 ? act.durationSec : 5;
  return Math.min(sec, 30) * 1000;
}

/** Canned Docs showcase (Black Friday multi-failure). */
export function demoChaosStory(): ChaosStory {
  return normalizeChaosStory({
    title: "Black Friday payment meltdown",
    setting: "Black Friday peak traffic",
    severity: "critical",
    summary:
      "Payment cluster drops during Black Friday while one JetStream node loses quorum and a consumer deploy introduces a schema mismatch.",
    acts: [
      {
        title: "Traffic surge hits payments",
        description: "Checkout volume spikes; PAYMENTS stream lag climbs as shoppers flood the funnel.",
        kind: "traffic_spike",
        targets: ["PAYMENTS", "payments.authorized"],
        durationSec: 5,
      },
      {
        title: "Payment cluster down",
        description: "The payment processing cluster becomes unreachable; publishers to PAYMENTS start timing out.",
        kind: "cluster_down",
        targets: ["PAYMENTS"],
        durationSec: 6,
      },
      {
        title: "JetStream quorum loss",
        description: "One JetStream replica drops out of R3; RAFT elections stall stream acknowledges on ORDERS.",
        kind: "quorum_loss",
        targets: ["ORDERS"],
        durationSec: 6,
      },
      {
        title: "Bad consumer deploy",
        description:
          "A rush deploy of ORDERS/billing expects a new required field; older messages fail validation and DLQ fills.",
        kind: "schema_mismatch",
        targets: ["ORDERS", "billing", "orders.created"],
        durationSec: 7,
      },
      {
        title: "Stabilization",
        description:
          "Rollback the consumer, restore the JetStream replica, and shed non-critical traffic until payments recover.",
        kind: "recovery",
        targets: ["PAYMENTS", "ORDERS", "billing"],
        durationSec: 5,
      },
    ],
    blastRadius: [
      "Checkout failures and abandoned carts",
      "ORDERS consumers stalled or poisoning on schema",
      "Downstream fulfillment delayed",
    ],
    recoveryHints: [
      "Roll back the billing consumer before fixing schema",
      "Restore JetStream replica / wait for quorum",
      "Pause non-critical publishers until PAYMENTS is healthy",
    ],
    source: "demo",
    demo: true,
  });
}

export function demoChaosStorySeed(): ChaosStorySeed {
  return {
    streams: ["PAYMENTS", "ORDERS", "BILLING"],
    consumers: ["billing", "shipping", "analytics"],
    subjects: ["payments.authorized", "orders.created", "orders.shipped"],
  };
}

export async function fetchChaosStoryDemo(): Promise<ChaosStoryEnvelope> {
  const res = await api<ChaosStoryEnvelope>("/api/v1/chaos-story/demo");
  return {
    story: res.data?.story ? normalizeChaosStory(res.data.story) : demoChaosStory(),
    seed: normalizeSeed(res.data?.seed) ?? demoChaosStorySeed(),
  };
}

export async function fetchChaosStorySeed(
  clusterId: string,
  options?: { fresh?: boolean },
): Promise<ChaosStoryEnvelope> {
  const fresh = options?.fresh ? "?fresh=1" : "";
  const res = await api<ChaosStoryEnvelope>(clusterPath(clusterId, `/chaos-story${fresh}`));
  return {
    story: res.data?.story ? normalizeChaosStory(res.data.story) : null,
    seed: normalizeSeed(res.data?.seed ?? EMPTY_SEED),
  };
}

export async function generateChaosStory(
  clusterId: string,
  hint?: string,
  options?: { fresh?: boolean },
): Promise<ChaosStoryGenerateResult> {
  const fresh = options?.fresh ? "?fresh=1" : "";
  const res = await api<ChaosStoryGenerateResult>(
    clusterPath(clusterId, `/chaos-story/generate${fresh}`),
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ hint: hint || "Invent a realistic multi-failure chaos story for peak traffic." }),
    },
  );
  return {
    story: normalizeChaosStory(res.data?.story),
    seed: normalizeSeed(res.data?.seed),
  };
}

export const CHAOS_STORY_HREF = "/docs/chaos-story";
