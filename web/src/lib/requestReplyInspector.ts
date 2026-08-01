export type RequestReplySnapshot = {
  capturedAt?: string;
  patterns?: unknown[];
  connections?: unknown[];
  requesters: number;
  responders: number;
  medianRttMs?: number | null;
  maxProbeMs?: number | null;
};

export function formatLatencyMs(ms: number | null | undefined): string {
  if (ms == null || !Number.isFinite(ms)) return "—";
  if (ms < 1) return `${ms.toFixed(2)} ms`;
  if (ms < 1000) return `${ms.toFixed(1)} ms`;
  return `${(ms / 1000).toFixed(2)} s`;
}

export function formatCount(value: number): string {
  return Math.round(value).toLocaleString();
}
