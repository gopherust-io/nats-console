import { api, clusterPath, jetStreamUIBase } from "./api";

export type SubjectNamingKind =
  | "wrong_case"
  | "missing_dots"
  | "non_dot_separator"
  | "shallow_hierarchy"
  | "inconsistent_variant";

export type SubjectNamingFinding = {
  kind: SubjectNamingKind | string;
  stream?: string;
  consumer?: string;
  subject: string;
  suggested: string;
  reasons: string[];
  cluster?: string[];
};

export type SubjectNamingTotals = {
  wrongCase: number;
  missingDots: number;
  nonDotSeparator: number;
  shallowHierarchy: number;
  inconsistentVariants: number;
  total: number;
};

export type SubjectNamingSnapshot = {
  capturedAt?: string;
  findings: SubjectNamingFinding[];
  totals: SubjectNamingTotals;
};

const KIND_ORDER: SubjectNamingKind[] = [
  "inconsistent_variant",
  "wrong_case",
  "missing_dots",
  "non_dot_separator",
  "shallow_hierarchy",
];

const EMPTY_TOTALS: SubjectNamingTotals = {
  wrongCase: 0,
  missingDots: 0,
  nonDotSeparator: 0,
  shallowHierarchy: 0,
  inconsistentVariants: 0,
  total: 0,
};

export async function fetchSubjectNaming(
  clusterId: string,
  options?: { fresh?: boolean },
): Promise<SubjectNamingSnapshot> {
  const fresh = options?.fresh ? "?fresh=1" : "";
  const snap = await api<SubjectNamingSnapshot>(clusterPath(clusterId, `/subject-naming${fresh}`));
  const data = snap.data;
  return {
    capturedAt: data?.capturedAt,
    findings: Array.isArray(data?.findings) ? data.findings : [],
    totals: data?.totals ?? EMPTY_TOTALS,
  };
}

export function sortSubjectNamingFindings(findings: SubjectNamingFinding[]): SubjectNamingFinding[] {
  return [...findings].sort((a, b) => {
    const ai = KIND_ORDER.indexOf(a.kind as SubjectNamingKind);
    const bi = KIND_ORDER.indexOf(b.kind as SubjectNamingKind);
    const ak = ai === -1 ? KIND_ORDER.length : ai;
    const bk = bi === -1 ? KIND_ORDER.length : bi;
    if (ak !== bk) return ak - bk;
    return (
      (a.stream ?? "").localeCompare(b.stream ?? "") ||
      (a.consumer ?? "").localeCompare(b.consumer ?? "") ||
      a.subject.localeCompare(b.subject)
    );
  });
}

/** Location state marker so stream/consumer pages can return to subject naming. */
export const NAMING_LOCATION_STATE = { from: "naming" } as const;

export type NamingLocationState = { from?: string };

/** Topology URL that opens the subject naming tab. */
export const NAMING_TOPOLOGY_HREF = "/admin/topology?view=naming";

export function isFromNaming(state: unknown): boolean {
  return Boolean(state && typeof state === "object" && (state as NamingLocationState).from === "naming");
}

export function subjectNamingFindingHref(
  finding: SubjectNamingFinding,
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

export function subjectNamingFindingLabel(finding: SubjectNamingFinding): string {
  const parts = [finding.stream, finding.consumer, finding.subject].filter(Boolean);
  return parts.join(" · ");
}
