import { api, ApiError, clusterPath } from "./api";

export type ArchitectureScoreFactor = {
  id: string;
  label: string;
  delta: number;
  sign: "plus" | "minus" | string;
};

export type ArchitectureScoreTrendPoint = {
  period: string;
  kind: "day" | "month" | string;
  score: number;
};

export type ArchitectureScoreSnapshot = {
  capturedAt?: string;
  score: number;
  maxScore: number;
  verdict: string;
  factors: ArchitectureScoreFactor[];
  trend: ArchitectureScoreTrendPoint[];
  avgLag?: number;
  demo?: boolean;
};

export type ArchitectureScoreAskResult = {
  reply: string;
  snapshot: ArchitectureScoreSnapshot;
};

const ROUTE_MISSING_HINT =
  "Architecture score API not found (404). Rebuild/restart the Consol API (e.g. make reload-api) so architecture-score routes are loaded.";

function normalizeSnapshot(data?: Partial<ArchitectureScoreSnapshot> | null): ArchitectureScoreSnapshot {
  return {
    capturedAt: data?.capturedAt,
    score: typeof data?.score === "number" ? data.score : 0,
    maxScore: typeof data?.maxScore === "number" ? data.maxScore : 100,
    verdict: data?.verdict ?? "",
    factors: Array.isArray(data?.factors) ? data!.factors! : [],
    trend: Array.isArray(data?.trend) ? data!.trend! : [],
    avgLag: data?.avgLag,
    demo: Boolean(data?.demo),
  };
}

async function getScore(path: string): Promise<ArchitectureScoreSnapshot> {
  try {
    const snap = await api<ArchitectureScoreSnapshot>(path);
    return normalizeSnapshot(snap.data);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      throw new ApiError(ROUTE_MISSING_HINT, { status: 404, code: "not_found" });
    }
    throw err;
  }
}

export async function fetchArchitectureScore(
  clusterId: string | null | undefined,
  options?: { fresh?: boolean; demo?: boolean },
): Promise<ArchitectureScoreSnapshot> {
  if (!clusterId || options?.demo) {
    if (clusterId && options?.demo) {
      return getScore(clusterPath(clusterId, `/architecture-score?demo=1`));
    }
    return getScore("/api/v1/architecture-score/demo");
  }
  const q = options?.fresh ? "?fresh=1" : "";
  return getScore(clusterPath(clusterId, `/architecture-score${q}`));
}

export async function askArchitectureScore(
  clusterId: string,
  message?: string,
  options?: { fresh?: boolean },
): Promise<ArchitectureScoreAskResult> {
  const q = options?.fresh ? "?fresh=1" : "";
  try {
    const snap = await api<ArchitectureScoreAskResult>(
      clusterPath(clusterId, `/architecture-score/ask${q}`),
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ message: message || "How is this architecture score doing?" }),
      },
    );
    return {
      reply: snap.data?.reply ?? "",
      snapshot: normalizeSnapshot(snap.data?.snapshot),
    };
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      throw new ApiError(ROUTE_MISSING_HINT, { status: 404, code: "not_found" });
    }
    throw err;
  }
}

export function demoArchitectureScore(): ArchitectureScoreSnapshot {
  return {
    score: 92,
    maxScore: 100,
    verdict: "Architecture score 92/100 — naming and latency improved; watch consumer fan-out and payloads",
    demo: true,
    factors: [
      { id: "naming", label: "Better naming", delta: 3, sign: "plus" },
      { id: "latency", label: "Better latency", delta: 4, sign: "plus" },
      { id: "consumer_explosion", label: "Consumer explosion", delta: -6, sign: "minus" },
      { id: "duplicate_events", label: "Duplicate events", delta: -2, sign: "minus" },
      { id: "payload_size", label: "Growing payload sizes", delta: -6, sign: "minus" },
    ],
    trend: [
      { period: "2026-03", kind: "month", score: 84 },
      { period: "2026-04", kind: "month", score: 86 },
      { period: "2026-05", kind: "month", score: 88 },
      { period: "2026-06", kind: "month", score: 90 },
      { period: "2026-07", kind: "month", score: 91 },
      { period: "2026-08", kind: "month", score: 92 },
    ],
  };
}

export const ARCHITECTURE_SCORE_HREF = "/docs/architecture-score";
