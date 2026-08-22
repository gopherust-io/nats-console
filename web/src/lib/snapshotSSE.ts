const SNAPSHOT_SUFFIXES = new Set([
  "topology",
  "account",
  "request-reply",
  "streams",
  "kv",
  "objects",
  "object-buckets",
  "varz-lite",
  "zombies",
  "subject-naming",
  "event-genome",
  "event-catalog",
  "event-wikipedia",
  "architecture-score",
  "architecture-review",
  "architecture-refactor",
  "chaos-story-seed",
  "replicas",
]);

let snapshotSSELive = false;

export function setSnapshotSSELive(live: boolean) {
  snapshotSSELive = live;
}

export function isSnapshotSSELive() {
  return snapshotSSELive;
}

export function isSnapshotBackedQueryKey(queryKey: readonly unknown[]): boolean {
  if (queryKey[0] === "clusters" && queryKey[1] === "connections") return true;
  if (queryKey[0] !== "cluster") return false;
  const suffix = String(queryKey[2] ?? "");
  if (suffix === "connz" || suffix === "connz-subs") return false;
  if (SNAPSHOT_SUFFIXES.has(suffix)) return true;
  if (suffix.startsWith("metrics-history:")) return true;
  return false;
}
