export type ConnzConnectionInspector = {
  start?: string;
  tls_version?: string;
  tls_cipher?: string;
  tls_cipher_suite?: string;
  slow_consumer?: boolean;
  is_slow_consumer?: boolean;
  reason?: string;
  stalls?: number;
  pending_bytes?: number;
};

/** Pending bytes above this are treated as a slow/stalling client. */
export const SLOW_PENDING_BYTES_THRESHOLD = 1024 * 1024; // 1 MiB

export function isSlowConsumerConnection(c: ConnzConnectionInspector): boolean {
  if (c.slow_consumer || c.is_slow_consumer) return true;
  if ((c.reason ?? "").toLowerCase().includes("slow")) return true;
  if ((c.stalls ?? 0) > 0) return true;
  if ((c.pending_bytes ?? 0) >= SLOW_PENDING_BYTES_THRESHOLD) return true;
  return false;
}

export function formatConnectedSince(start?: string): string {
  if (!start) return "—";
  const d = new Date(start);
  if (Number.isNaN(d.getTime())) return start;
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

export function connectionTLSCipher(c: ConnzConnectionInspector): string | undefined {
  return c.tls_cipher_suite || c.tls_cipher || undefined;
}

export function formatTLSVersion(c: ConnzConnectionInspector): string {
  const cipher = connectionTLSCipher(c);
  if (!c.tls_version && !cipher) return "—";
  if (c.tls_version && cipher) return `${c.tls_version} (${cipher})`;
  return c.tls_version || cipher || "—";
}

/** Parse a NATS RTT string (e.g. "1.2ms", "500µs", "1s") into milliseconds. */
export function parseRttMs(rtt?: string | null): number | null {
  if (!rtt) return null;
  const match = /^([\d.]+)\s*(ms|µs|us|ns|s)?$/i.exec(rtt.trim());
  if (!match) return null;
  const value = Number(match[1]);
  if (!Number.isFinite(value)) return null;
  const unit = (match[2] ?? "ms").toLowerCase();
  switch (unit) {
    case "ns":
      return value / 1_000_000;
    case "us":
    case "µs":
      return value / 1000;
    case "s":
      return value * 1000;
    default:
      return value;
  }
}

export function formatRttDisplay(rtt?: string): string {
  if (!rtt) return "—";
  const match = /^([\d.]+)\s*(ms|µs|us|s)?$/i.exec(rtt.trim());
  if (!match) return rtt;
  const value = Number(match[1]);
  if (Number.isNaN(value)) return rtt;
  const unit = (match[2] ?? "ms").toLowerCase().replace("us", "µs");
  const rounded = value >= 10 ? value.toFixed(1) : value.toFixed(2);
  return `${rounded}${unit}`;
}

/** Prefer NATS authorized_user; fall back to legacy `user`. */
export function connectionUsername(c: {
  authorized_user?: string;
  user?: string;
}): string | undefined {
  return c.authorized_user || c.user || undefined;
}
