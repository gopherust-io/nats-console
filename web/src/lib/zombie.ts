import { api, clusterPath, jetStreamUIBase } from "./api";

export type ZombieKind =
  | "empty_stream"
  | "idle_consumer"
  | "unconsumed_subject"
  | "unpublished_subject"
  | "unbound_consumer";

export type ZombieFinding = {
  kind: ZombieKind | string;
  stream?: string;
  consumer?: string;
  subject?: string;
  reasons: string[];
};

export type ZombieTotals = {
  emptyStreams: number;
  idleConsumers: number;
  unconsumedSubjects: number;
  unpublishedSubjects: number;
  unboundConsumers: number;
  total: number;
};

export type ZombieSnapshot = {
  capturedAt?: string;
  findings: ZombieFinding[];
  totals: ZombieTotals;
};

const KIND_ORDER: ZombieKind[] = [
  "empty_stream",
  "idle_consumer",
  "unconsumed_subject",
  "unpublished_subject",
  "unbound_consumer",
];

export async function fetchZombies(
  clusterId: string,
  options?: { fresh?: boolean },
): Promise<ZombieSnapshot> {
  const fresh = options?.fresh ? "?fresh=1" : "";
  const snap = await api<ZombieSnapshot>(clusterPath(clusterId, `/zombies${fresh}`));
  const data = snap.data;
  return {
    capturedAt: data?.capturedAt,
    findings: Array.isArray(data?.findings) ? data.findings : [],
    totals: data?.totals ?? {
      emptyStreams: 0,
      idleConsumers: 0,
      unconsumedSubjects: 0,
      unpublishedSubjects: 0,
      unboundConsumers: 0,
      total: 0,
    },
  };
}

export function sortZombieFindings(findings: ZombieFinding[]): ZombieFinding[] {
  return [...findings].sort((a, b) => {
    const ai = KIND_ORDER.indexOf(a.kind as ZombieKind);
    const bi = KIND_ORDER.indexOf(b.kind as ZombieKind);
    const ak = ai === -1 ? KIND_ORDER.length : ai;
    const bk = bi === -1 ? KIND_ORDER.length : bi;
    if (ak !== bk) return ak - bk;
    return (a.stream ?? "").localeCompare(b.stream ?? "") ||
      (a.consumer ?? "").localeCompare(b.consumer ?? "") ||
      (a.subject ?? "").localeCompare(b.subject ?? "");
  });
}

export function groupZombiesByKind(findings: ZombieFinding[]): Map<string, ZombieFinding[]> {
  const sorted = sortZombieFindings(findings);
  const groups = new Map<string, ZombieFinding[]>();
  for (const f of sorted) {
    const list = groups.get(f.kind) ?? [];
    list.push(f);
    groups.set(f.kind, list);
  }
  return groups;
}

/** Location state marker so stream/consumer pages can return to zombie detection. */
export const ZOMBIES_LOCATION_STATE = { from: "zombies" } as const;

export type ZombiesLocationState = { from?: string };

/** Topology URL that opens the zombie detection tab. */
export const ZOMBIES_TOPOLOGY_HREF = "/admin/topology?view=zombies";

export function isFromZombies(state: unknown): boolean {
  return Boolean(state && typeof state === "object" && (state as ZombiesLocationState).from === "zombies");
}

export function zombieFindingHref(
  finding: ZombieFinding,
  clusterId: string,
  accountName = "Default",
): string | null {
  if (!finding.stream) return null;
  const base = jetStreamUIBase(clusterId, accountName);
  if (finding.consumer) {
    return `${base}/streams/${encodeURIComponent(finding.stream)}/consumers/${encodeURIComponent(finding.consumer)}`;
  }
  return `${base}/streams/${encodeURIComponent(finding.stream)}`;
}

export function zombieFindingLabel(finding: ZombieFinding): string {
  const parts = [finding.stream, finding.consumer, finding.subject].filter(Boolean);
  return parts.join(" · ");
}
