/** Compact count for impact display (e.g. 10M, 1.5K). */
export function formatCompactCount(n: number): string {
  if (!Number.isFinite(n) || n < 0) return "0";
  if (n >= 1_000_000_000) return `${trimFixed(n / 1_000_000_000)}B`;
  if (n >= 1_000_000) return `${trimFixed(n / 1_000_000)}M`;
  if (n >= 1_000) return `${trimFixed(n / 1_000)}K`;
  return String(Math.round(n));
}

/** Humanize millisecond durations (e.g. 3h, 45m, 12s). */
export function formatDurationMs(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return "0s";
  const sec = Math.round(ms / 1000);
  if (sec < 60) return `${sec}s`;
  const min = Math.floor(sec / 60);
  if (min < 60) {
    const rem = sec % 60;
    return rem ? `${min}m ${rem}s` : `${min}m`;
  }
  const hours = Math.floor(min / 60);
  const remMin = min % 60;
  return remMin ? `${hours}h ${remMin}m` : `${hours}h`;
}

function trimFixed(n: number): string {
  return n.toFixed(1).replace(/\.0$/, "");
}
