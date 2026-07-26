/** Format an ISO timestamp for display; returns fallback on invalid/missing input. */
export function formatDateTime(iso?: string | null, fallback = ""): string {
  if (!iso) return fallback;
  try {
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) return iso;
    return date.toLocaleString();
  } catch {
    return iso;
  }
}
