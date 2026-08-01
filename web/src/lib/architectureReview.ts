import { api, clusterPath } from "./api";

export type ArchSeverity = "info" | "warn" | "critical";
export type ArchVerdict = "healthy" | "needs_attention" | "at_risk";

export type ArchitectureFinding = {
  kind: string;
  severity: ArchSeverity | string;
  title: string;
  evidence: string[];
  suggestion: string;
  stream?: string;
  subject?: string;
  consumer?: string;
};

export type ArchitectureReviewTotals = {
  problems: number;
  critical: number;
  warn: number;
  info: number;
  byKind?: Record<string, number>;
};

export type ArchitectureReviewSnapshot = {
  capturedAt?: string;
  verdict: ArchVerdict | string;
  problems: ArchitectureFinding[];
  suggestions: string[];
  totals: ArchitectureReviewTotals;
  demo?: boolean;
};

export type ArchitectureReviewAskResult = {
  reply: string;
  snapshot: ArchitectureReviewSnapshot;
};

const EMPTY_TOTALS: ArchitectureReviewTotals = {
  problems: 0,
  critical: 0,
  warn: 0,
  info: 0,
};

function normalizeSnapshot(data?: Partial<ArchitectureReviewSnapshot> | null): ArchitectureReviewSnapshot {
  return {
    capturedAt: data?.capturedAt,
    verdict: data?.verdict ?? "healthy",
    problems: Array.isArray(data?.problems) ? data.problems : [],
    suggestions: Array.isArray(data?.suggestions) ? data.suggestions : [],
    totals: data?.totals ?? EMPTY_TOTALS,
    demo: Boolean(data?.demo),
  };
}

export async function fetchArchitectureReview(
  clusterId: string,
  options?: { fresh?: boolean },
): Promise<ArchitectureReviewSnapshot> {
  const fresh = options?.fresh ? "?fresh=1" : "";
  const snap = await api<ArchitectureReviewSnapshot>(
    clusterPath(clusterId, `/architecture-review${fresh}`),
  );
  return normalizeSnapshot(snap.data);
}

export async function askArchitectureReview(
  clusterId: string,
  message?: string,
  options?: { fresh?: boolean },
): Promise<ArchitectureReviewAskResult> {
  const fresh = options?.fresh ? "?fresh=1" : "";
  const snap = await api<ArchitectureReviewAskResult>(
    clusterPath(clusterId, `/architecture-review/ask${fresh}`),
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ message: message || "Is this event architecture good?" }),
    },
  );
  return {
    reply: snap.data?.reply ?? "",
    snapshot: normalizeSnapshot(snap.data?.snapshot),
  };
}

/** Canned Docs showcase when cluster inventory is empty. */
export function demoArchitectureReview(): ArchitectureReviewSnapshot {
  return {
    verdict: "at_risk",
    demo: true,
    problems: [
      {
        kind: "too_many_consumers",
        severity: "warn",
        title: "Subject orders.created has 9 consumers",
        evidence: ["ORDERS/billing", "ORDERS/shipping", "ORDERS/analytics"],
        suggestion:
          "Reduce fan-out on orders.created — prefer fewer consumers, shared queue groups, or split into narrower subjects.",
        subject: "orders.created",
      },
      {
        kind: "circular_dependency",
        severity: "critical",
        title: "Circular dependency between ORDERS and BILLING",
        evidence: ["ORDERS subjects consumed by BILLING", "BILLING subjects consumed by ORDERS"],
        suggestion:
          "Break the cycle — pick a single owner stream for shared subjects, or introduce a dedicated bridge stream with one-way flow.",
        stream: "ORDERS",
      },
      {
        kind: "naming_inconsistent",
        severity: "info",
        title: "Semantic duplicate subject: orders.new",
        evidence: ["genome=order.created", "orders.created", "orders.new"],
        suggestion: "Converge synonyms onto order.created",
        subject: "orders.new",
      },
      {
        kind: "payload_too_large",
        severity: "warn",
        title: "Stream ORDERS average message size is 400.0 KiB",
        evidence: ["bytes=409600000 messages=1000 avg=409600"],
        suggestion:
          "Shrink payloads on ORDERS — store blobs in Object Store/KV and publish references, or compress large fields.",
        stream: "ORDERS",
      },
    ],
    suggestions: [
      "Reduce fan-out on orders.created — prefer fewer consumers, shared queue groups, or split into narrower subjects.",
      "Break the cycle — pick a single owner stream for shared subjects, or introduce a dedicated bridge stream with one-way flow.",
      "Converge synonyms onto order.created",
      "Shrink payloads on ORDERS — store blobs in Object Store/KV and publish references, or compress large fields.",
    ],
    totals: { problems: 4, critical: 1, warn: 2, info: 1 },
  };
}

export const ARCHITECTURE_REVIEW_HREF = "/docs/architecture-review";
