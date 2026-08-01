import { api, clusterPath, jetStreamUIBase } from "./api";
import { EVENT_CATALOG_HREF } from "./eventCatalog";

export type EventGenomeFinding = {
  subject: string;
  suggested: string;
  genome: string;
  cluster: string[];
  stream?: string;
  consumer?: string;
  reasons: string[];
};

export type EventGenomeTotals = {
  clusters: number;
  duplicates: number;
  total: number;
};

export type EventGenomeSnapshot = {
  capturedAt?: string;
  findings: EventGenomeFinding[];
  totals: EventGenomeTotals;
};

const EMPTY_TOTALS: EventGenomeTotals = {
  clusters: 0,
  duplicates: 0,
  total: 0,
};

export async function fetchEventGenome(
  clusterId: string,
  options?: { fresh?: boolean },
): Promise<EventGenomeSnapshot> {
  const fresh = options?.fresh ? "?fresh=1" : "";
  const snap = await api<EventGenomeSnapshot>(clusterPath(clusterId, `/event-genome${fresh}`));
  const data = snap.data;
  return {
    capturedAt: data?.capturedAt,
    findings: Array.isArray(data?.findings) ? data.findings : [],
    totals: data?.totals ?? EMPTY_TOTALS,
  };
}

/** Sort by cluster size (desc), then genome, then subject. */
export function sortEventGenomeFindings(findings: EventGenomeFinding[]): EventGenomeFinding[] {
  return [...findings].sort((a, b) => {
    const as = a.cluster?.length ?? 0;
    const bs = b.cluster?.length ?? 0;
    if (as !== bs) return bs - as;
    return (
      a.genome.localeCompare(b.genome) ||
      (a.stream ?? "").localeCompare(b.stream ?? "") ||
      (a.consumer ?? "").localeCompare(b.consumer ?? "") ||
      a.subject.localeCompare(b.subject)
    );
  });
}

/** Location state marker so stream/consumer pages can return to event genome. */
export const GENOME_LOCATION_STATE = { from: "genome" } as const;

export type GenomeLocationState = { from?: string };

/** Topology URL that opens the event genome tab. */
export const GENOME_TOPOLOGY_HREF = "/admin/topology?view=genome";

export function isFromGenome(state: unknown): boolean {
  return Boolean(state && typeof state === "object" && (state as GenomeLocationState).from === "genome");
}

export function eventGenomeFindingHref(
  finding: EventGenomeFinding,
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

export function eventGenomeCatalogHref(subject: string): string {
  return `${EVENT_CATALOG_HREF}?q=${encodeURIComponent(subject)}`;
}

export function eventGenomeFindingLabel(finding: EventGenomeFinding): string {
  const parts = [finding.stream, finding.consumer, finding.subject].filter(Boolean);
  return parts.join(" · ");
}
