import { useMutation, useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import Alert from "../components/ui/Alert";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import StatCard from "../components/ui/StatCard";
import {
  downloadPprofProfile,
  fetchPprofConfig,
  fetchPprofContinuous,
  fetchPprofProfileSummary,
  fetchPprofRuntime,
  formatPprofBytes,
  pprofDebugIndexUrl,
  type PprofProfileEntry,
  type PprofRuntimeStats,
} from "../lib/pprof";

const PROFILE_LABELS: Record<string, string> = {
  heap: "Heap allocations",
  goroutine: "Goroutines",
  allocs: "All allocations",
  block: "Blocking",
  mutex: "Mutex contention",
  threadcreate: "Thread creation",
  cpu: "CPU",
};

function ProfileBar({ entry }: { entry: PprofProfileEntry }) {
  const width = Math.max(entry.flatPercent, 0.5);
  return (
    <div className="pprof-bar">
      <div className="pprof-bar__label" title={entry.name}>
        {entry.name}
      </div>
      <div className="pprof-bar__track" aria-hidden>
        <div className="pprof-bar__fill" style={{ width: `${Math.min(width, 100)}%` }} />
      </div>
      <div className="pprof-bar__value">{entry.flatPercent.toFixed(1)}%</div>
    </div>
  );
}

function RuntimeSparkline({
  label,
  history,
  value,
}: {
  label: string;
  history: PprofRuntimeStats[];
  value: (point: PprofRuntimeStats) => number;
}) {
  const points = history.map(value);
  const max = Math.max(...points, 1);
  return (
    <div className="pprof-sparkline">
      <div className="pprof-sparkline__label">{label}</div>
      <div className="pprof-sparkline__track" aria-hidden>
        {points.map((point, index) => (
          <span
            key={index}
            className="pprof-sparkline__bar"
            style={{ height: `${Math.max((point / max) * 100, 4)}%` }}
          />
        ))}
      </div>
    </div>
  );
}

export default function ProfilingPage() {
  const [selectedProfile, setSelectedProfile] = useState("heap");
  const [cpuSeconds, setCpuSeconds] = useState(30);
  const [continuousMode, setContinuousMode] = useState(true);
  const [downloadError, setDownloadError] = useState("");

  const configQuery = useQuery({
    queryKey: ["pprof", "config"],
    queryFn: fetchPprofConfig,
  });

  const continuousEnabled = configQuery.data?.continuousEnabled === true && continuousMode;

  const continuousQuery = useQuery({
    queryKey: ["pprof", "continuous"],
    queryFn: fetchPprofContinuous,
    enabled: configQuery.data?.enabled === true && continuousEnabled,
    refetchInterval: (configQuery.data?.continuousIntervalSecs ?? 15) * 1000,
  });

  const runtimeQuery = useQuery({
    queryKey: ["pprof", "runtime"],
    queryFn: fetchPprofRuntime,
    enabled: configQuery.data?.enabled === true && !continuousEnabled,
    refetchInterval: 10_000,
  });

  const profileMutation = useMutation({
    mutationFn: () =>
      fetchPprofProfileSummary(selectedProfile, selectedProfile === "cpu" ? cpuSeconds : undefined, false),
  });

  const config = configQuery.data;
  const continuous = continuousQuery.data;
  const runtime = continuousEnabled
    ? continuous?.runtimeHistory.at(-1) ?? null
    : runtimeQuery.data ?? null;
  const runtimeHistory = continuous?.runtimeHistory ?? [];
  const profiles = config?.profiles ?? [];
  const maxCpu = config?.maxCpuSeconds ?? 120;

  const summary = useMemo(() => {
    if (continuousEnabled && continuous?.profiles[selectedProfile]) {
      return continuous.profiles[selectedProfile];
    }
    return profileMutation.data ?? null;
  }, [continuousEnabled, continuous, selectedProfile, profileMutation.data]);

  const error =
    (configQuery.error instanceof Error ? configQuery.error.message : "") ||
    (continuousQuery.error instanceof Error ? continuousQuery.error.message : "") ||
    (runtimeQuery.error instanceof Error ? runtimeQuery.error.message : "") ||
    (profileMutation.error instanceof Error ? profileMutation.error.message : "") ||
    downloadError;

  async function handleDownload() {
    setDownloadError("");
    try {
      await downloadPprofProfile(selectedProfile, selectedProfile === "cpu" ? cpuSeconds : undefined);
    } catch (err) {
      setDownloadError(err instanceof Error ? err.message : "Download failed");
    }
  }

  if (configQuery.isLoading) {
    return (
      <div className="page">
        <div className="skeleton skeleton--panel" />
      </div>
    );
  }

  if (configQuery.isError) {
    return (
      <div className="page">
        <PageHeader
          eyebrow="Administration"
          title="Profiling"
          subtitle="Runtime profiling and performance visualization for the console server."
        />
        <Alert variant="error">
          {configQuery.error instanceof Error ? configQuery.error.message : "Failed to load profiling config"}
        </Alert>
        <button className="btn btn--secondary" type="button" onClick={() => configQuery.refetch()}>
          Retry
        </button>
      </div>
    );
  }

  if (configQuery.isSuccess && !config?.enabled) {
    return (
      <div className="page">
        <PageHeader
          eyebrow="Administration"
          title="Profiling"
          subtitle="Runtime profiling and performance visualization for the console server."
        />
        <EmptyState
          title="Profiling disabled"
          description="Set PPROF_ENABLED=true on the server to expose Go pprof endpoints and this dashboard."
        />
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader
        eyebrow="Administration"
        title="Profiling"
        subtitle="Continuous background sampling plus on-demand profiles for the console server process."
        actions={
          <div className="pprof-actions">
            <label className="pprof-toggle">
              <input
                type="checkbox"
                checked={continuousMode}
                onChange={(event) => setContinuousMode(event.target.checked)}
              />
              Continuous
            </label>
            <a className="btn btn--secondary" href={pprofDebugIndexUrl()} target="_blank" rel="noreferrer">
              Open pprof API
            </a>
          </div>
        }
      />

      <Alert variant="error">{error}</Alert>

      {continuousEnabled && (
        <p className="panel__desc pprof-live-note">
          Live sampling every {config?.continuousIntervalSecs ?? 15}s
          {config?.continuousCpuSliceSecs ? ` · CPU window ${config.continuousCpuSliceSecs}s` : ""}
          {continuousQuery.isFetching ? " · refreshing…" : ""}
        </p>
      )}

      {runtime && (
        <div className="stat-grid">
          <StatCard label="Goroutines" value={runtime.goroutines} accent="sky" />
          <StatCard label="Heap in use" value={formatPprofBytes(runtime.memory.heapInuse)} accent="violet" />
          <StatCard label="Heap alloc" value={formatPprofBytes(runtime.memory.heapAlloc)} accent="emerald" />
          <StatCard label="GC cycles" value={runtime.memory.numGc} accent="amber" />
        </div>
      )}

      {continuousEnabled && runtimeHistory.length > 1 && (
        <section className="panel pprof-panel">
          <h2 className="panel__title">Runtime history</h2>
          <div className="pprof-sparklines">
            <RuntimeSparkline label="Goroutines" history={runtimeHistory} value={(point) => point.goroutines} />
            <RuntimeSparkline label="Heap in use" history={runtimeHistory} value={(point) => point.memory.heapInuse} />
            <RuntimeSparkline label="Heap alloc" history={runtimeHistory} value={(point) => point.memory.heapAlloc} />
          </div>
        </section>
      )}

      <section className="panel pprof-panel">
        <h2 className="panel__title">Profile visualization</h2>
        <p className="panel__desc">
          {continuousEnabled
            ? "Showing the latest background sample. Turn off Continuous or click Collect now for a fresh on-demand snapshot."
            : "Collect a profile snapshot and view the top contributors. CPU profiles block for the selected duration."}
        </p>

        <div className="pprof-controls">
          <label className="pprof-controls__field">
            Profile type
            <select value={selectedProfile} onChange={(event) => setSelectedProfile(event.target.value)}>
              {profiles.map((profile) => (
                <option key={profile} value={profile}>
                  {PROFILE_LABELS[profile] ?? profile}
                </option>
              ))}
            </select>
          </label>

          {!continuousEnabled && selectedProfile === "cpu" && (
            <label className="pprof-controls__field">
              Duration (seconds)
              <input
                type="number"
                min={5}
                max={maxCpu}
                value={cpuSeconds}
                onChange={(event) => setCpuSeconds(Number(event.target.value))}
              />
            </label>
          )}

          {!continuousEnabled && (
            <button
              className="btn btn--primary"
              type="button"
              onClick={() => profileMutation.mutate()}
              disabled={profileMutation.isPending}
            >
              {profileMutation.isPending
                ? selectedProfile === "cpu"
                  ? `Sampling CPU (${cpuSeconds}s)…`
                  : "Collecting…"
                : "Collect profile"}
            </button>
          )}

          <button className="btn btn--secondary" type="button" onClick={handleDownload}>
            Download raw profile
          </button>
        </div>

        {!continuousEnabled && profileMutation.isPending && <div className="skeleton skeleton--panel" />}

        {continuousEnabled && continuousQuery.isLoading && !summary && <div className="skeleton skeleton--panel" />}

        {summary && (continuousEnabled || !profileMutation.isPending) && (
          <>
            <div className="pprof-summary-meta">
              <span>Type: {summary.profileType}</span>
              {summary.durationSecs ? <span>Duration: {summary.durationSecs}s</span> : null}
              {summary.totalSamples ? <span>Samples: {summary.totalSamples.toLocaleString()}</span> : null}
              {summary.fetchedAt ? <span>Sampled: {new Date(summary.fetchedAt).toLocaleTimeString()}</span> : null}
            </div>

            {(summary.entries ?? []).length === 0 ? (
              <EmptyState
                title="Waiting for samples"
                description={
                  continuousEnabled
                    ? "Background sampling is running. Check back in a few seconds."
                    : "The profile did not contain any measurable samples."
                }
              />
            ) : (
              <div className="pprof-bars">
                {(summary.entries ?? []).map((entry) => (
                  <ProfileBar key={entry.name} entry={entry} />
                ))}
              </div>
            )}
          </>
        )}
      </section>
    </div>
  );
}
