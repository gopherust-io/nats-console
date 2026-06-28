import { api, getAuthHeader } from "./api";

export type PprofConfig = {
  enabled: boolean;
  authRequired?: boolean;
  continuousEnabled?: boolean;
  continuousIntervalSecs?: number;
  continuousCpuSliceSecs?: number;
  profiles?: string[];
  maxCpuSeconds?: number;
};

export type PprofRuntimeStats = {
  fetchedAt: string;
  goroutines: number;
  memory: {
    alloc: number;
    totalAlloc: number;
    sys: number;
    heapAlloc: number;
    heapInuse: number;
    heapObjects: number;
    numGc: number;
  };
};

export type PprofProfileEntry = {
  name: string;
  flat: number;
  flatPercent: number;
  cum: number;
  cumPercent: number;
};

export type PprofProfileSummary = {
  fetchedAt: string;
  profileType: string;
  totalSamples?: number;
  durationSecs?: number;
  entries: PprofProfileEntry[];
};

export type PprofContinuousSnapshot = {
  fetchedAt: string;
  intervalSecs: number;
  cpuSliceSecs: number;
  profiles: Record<string, PprofProfileSummary>;
  runtimeHistory: PprofRuntimeStats[];
};

export function fetchPprofConfig() {
  return api<PprofConfig>("/api/v1/pprof/config");
}

export function fetchPprofRuntime() {
  return api<PprofRuntimeStats>("/api/v1/pprof/runtime");
}

export function fetchPprofContinuous() {
  return api<PprofContinuousSnapshot>("/api/v1/pprof/continuous");
}

export function fetchPprofProfileSummary(profile: string, seconds?: number, continuous = false) {
  const params = new URLSearchParams();
  if (profile === "cpu" && seconds) {
    params.set("seconds", String(seconds));
  }
  if (continuous) {
    params.set("source", "continuous");
  }
  const query = params.toString();
  return api<PprofProfileSummary>(`/api/v1/pprof/profile/${profile}${query ? `?${query}` : ""}`);
}

export function pprofProfileDownloadUrl(profile: string, seconds?: number) {
  const params = new URLSearchParams();
  if (profile === "cpu" && seconds) {
    params.set("seconds", String(seconds));
  }
  const query = params.toString();
  return `/api/v1/pprof/profile/${profile}/download${query ? `?${query}` : ""}`;
}

export function pprofDebugIndexUrl() {
  return "/api/v1/pprof/config";
}

export function formatPprofBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  return `${(value / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

export async function downloadPprofProfile(profile: string, seconds?: number) {
  const url = pprofProfileDownloadUrl(profile, seconds);
  const headers = new Headers();
  const auth = getAuthHeader();
  if (auth) {
    headers.set("Authorization", auth);
  }
  const response = await fetch(url, { headers, credentials: "include" });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error ?? `Download failed (${response.status})`);
  }
  const blob = await response.blob();
  const filename =
    response.headers.get("Content-Disposition")?.match(/filename="?([^";]+)"?/)?.[1] ??
    (profile === "cpu" ? "cpu.pprof" : `${profile}.pb.gz`);
  const objectUrl = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = objectUrl;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(objectUrl);
}
