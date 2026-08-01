import { api, ApiError, clusterPath } from "./api";

export type BottleneckSeverity = "info" | "warn" | "critical";
export type BottleneckVerdict = "healthy" | "needs_attention" | "at_risk";

export type HiddenBottleneckFinding = {
  kind: string;
  severity: BottleneckSeverity | string;
  title: string;
  evidence: string[];
  suggestion: string;
  stream?: string;
  consumer?: string;
  schedule?: string;
  weekday?: number;
  hourUtc?: number;
};

export type HiddenBottleneckTotals = {
  problems: number;
  critical: number;
  warn: number;
  info: number;
  byKind?: Record<string, number>;
};

export type HiddenBottleneckSnapshot = {
  capturedAt?: string;
  from?: string;
  to?: string;
  verdict: BottleneckVerdict | string;
  findings: HiddenBottleneckFinding[];
  suggestions: string[];
  totals: HiddenBottleneckTotals;
  demo?: boolean;
};

export type HiddenBottlenecksAskResult = {
  reply: string;
  snapshot: HiddenBottleneckSnapshot;
};

const EMPTY_TOTALS: HiddenBottleneckTotals = {
  problems: 0,
  critical: 0,
  warn: 0,
  info: 0,
};

function normalizeSnapshot(data?: Partial<HiddenBottleneckSnapshot> | null): HiddenBottleneckSnapshot {
  return {
    capturedAt: data?.capturedAt,
    from: data?.from,
    to: data?.to,
    verdict: data?.verdict ?? "healthy",
    findings: Array.isArray(data?.findings) ? data.findings : [],
    suggestions: Array.isArray(data?.suggestions) ? data.suggestions : [],
    totals: data?.totals ?? EMPTY_TOTALS,
    demo: Boolean(data?.demo),
  };
}

export async function fetchHiddenBottlenecks(
  clusterId: string,
  options?: { demo?: boolean },
): Promise<HiddenBottleneckSnapshot> {
  const demo = options?.demo ? "?demo=1" : "";
  try {
    const snap = await api<HiddenBottleneckSnapshot>(
      clusterPath(clusterId, `/hidden-bottlenecks${demo}`),
    );
    return normalizeSnapshot(snap.data);
  } catch (err) {
    if (err instanceof ApiError && err.code === "not_found") {
      throw new ApiError(
        "Hidden bottlenecks API not found (404). Rebuild/restart the Consol API (e.g. make reload-api) so the route is loaded.",
        {
          status: err.status,
          code: err.code,
          retryable: err.retryable,
          retryAfterSeconds: err.retryAfterSeconds,
        },
      );
    }
    throw err;
  }
}

export async function askHiddenBottlenecks(
  clusterId: string,
  message?: string,
  options?: { demo?: boolean },
): Promise<HiddenBottlenecksAskResult> {
  const demo = options?.demo ? "?demo=1" : "";
  const snap = await api<HiddenBottlenecksAskResult>(
    clusterPath(clusterId, `/hidden-bottlenecks/ask${demo}`),
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        message: message || "What hidden bottlenecks should we care about?",
      }),
    },
  );
  return {
    reply: snap.data?.reply ?? "",
    snapshot: normalizeSnapshot(snap.data?.snapshot),
  };
}

/** Canned Docs showcase — Friday 18:00 payload doubles. */
export function demoHiddenBottlenecks(): HiddenBottleneckSnapshot {
  return {
    verdict: "needs_attention",
    demo: true,
    findings: [
      {
        kind: "correlated_payload_lag",
        severity: "warn",
        title: "Friday 18:00 UTC billing-worker slows when ORDERS payload grows",
        evidence: [
          "schedule=Friday 18:00 UTC",
          "consumer=billing-worker",
          "stream=ORDERS",
          "lag=420 (baseline 42)",
          "avgPayload=8192B (baseline 4096B)",
          "weeks=4",
        ],
        suggestion:
          "Inspect producers that enlarge payloads on this schedule; consider compression, schema trimming, or scaling the consumer before the window.",
        stream: "ORDERS",
        consumer: "billing-worker",
        schedule: "Friday 18:00 UTC",
        weekday: 5,
        hourUtc: 18,
      },
    ],
    suggestions: [
      "Inspect producers that enlarge payloads on this schedule; consider compression, schema trimming, or scaling the consumer before the window.",
    ],
    totals: { problems: 1, critical: 0, warn: 1, info: 0, byKind: { correlated_payload_lag: 1 } },
  };
}

export const HIDDEN_BOTTLENECKS_HREF = "/docs/hidden-bottlenecks";

export function findingMatchesConsumer(
  findings: HiddenBottleneckFinding[],
  stream: string,
  consumer: string,
): boolean {
  return findings.some((f) => f.stream === stream && f.consumer === consumer);
}
