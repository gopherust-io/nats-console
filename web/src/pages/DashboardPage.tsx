import { lazy, Suspense, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Alert from "../components/ui/Alert";
import PageHeader from "../components/ui/PageHeader";
import StatCard from "../components/ui/StatCard";
import TimeRangeSelector from "../components/metrics/TimeRangeSelector";
import { useClusterMetricsHistory } from "../hooks/useClusterMetricsHistory";
import { AccountInfo, api, clusterPath } from "../lib/api";
import { MetricsRangePreset } from "../lib/metricsHistory";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

const MetricsTimeSeriesChart = lazy(() => import("../components/metrics/MetricsTimeSeriesChart"));

const dashboardPollOptions = {
  refetchInterval: 30_000,
  refetchOnWindowFocus: false,
  refetchIntervalInBackground: false,
} as const;

const dashboardHistoryMetrics =
  "jetstream.storage_bytes,jetstream.memory_bytes,jsz.messages,server.connections,server.in_msgs_total,server.out_msgs_total";

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  return `${(value / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

type JSZResponse = {
  total?: {
    streams?: number;
    consumers?: number;
    messages?: number;
  };
};

function seriesPoints(
  history: ReturnType<typeof useClusterMetricsHistory>["data"],
  metric: string,
) {
  return history?.series.find((item) => item.metric === metric)?.points ?? [];
}

export default function DashboardPage() {
  const { clusterId, cluster } = useCluster();
  const [range, setRange] = useState<MetricsRangePreset>("24h");
  const loading = !clusterId;

  const accountQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "account"),
    queryFn: () => api<AccountInfo>(clusterPath(clusterId!, "/account")),
    enabled: Boolean(clusterId),
    ...dashboardPollOptions,
  });

  const varzQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "varz"),
    queryFn: () => api<Record<string, unknown>>(clusterPath(clusterId!, "/monitoring/varz")),
    enabled: Boolean(clusterId),
    ...dashboardPollOptions,
  });

  const jszQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "jsz"),
    queryFn: () => api<JSZResponse>(clusterPath(clusterId!, "/monitoring/jsz?streams=1&consumers=1")),
    enabled: Boolean(clusterId),
    ...dashboardPollOptions,
  });

  const historyQuery = useClusterMetricsHistory(clusterId, range, dashboardHistoryMetrics);

  const error =
    (accountQuery.error instanceof Error && accountQuery.error.message) ||
    (varzQuery.error instanceof Error && varzQuery.error.message) ||
    (jszQuery.error instanceof Error && jszQuery.error.message) ||
    "";

  const account = accountQuery.data ?? null;
  const varz = varzQuery.data ?? null;
  const jsz = jszQuery.data ?? null;
  const isFetching = accountQuery.isFetching || varzQuery.isFetching;

  const capacitySeries = useMemo(
    () => [
      {
        key: "jetstream.storage_bytes",
        label: "Storage",
        color: "var(--accent)",
        points: seriesPoints(historyQuery.data, "jetstream.storage_bytes"),
        formatValue: formatBytes,
      },
      {
        key: "jetstream.memory_bytes",
        label: "Memory",
        color: "#f59e0b",
        points: seriesPoints(historyQuery.data, "jetstream.memory_bytes"),
        formatValue: formatBytes,
      },
    ],
    [historyQuery.data],
  );

  const messagesSeries = useMemo(
    () => [
      {
        key: "jsz.messages",
        label: "Messages",
        color: "#10b981",
        points: seriesPoints(historyQuery.data, "jsz.messages"),
      },
    ],
    [historyQuery.data],
  );

  const trafficSeries = useMemo(
    () => [
      {
        key: "server.connections",
        label: "Connections",
        color: "var(--accent)",
        points: seriesPoints(historyQuery.data, "server.connections"),
      },
      {
        key: "server.in_msgs_total",
        label: "In msgs / step",
        color: "#8b5cf6",
        points: seriesPoints(historyQuery.data, "server.in_msgs_total"),
      },
      {
        key: "server.out_msgs_total",
        label: "Out msgs / step",
        color: "#06b6d4",
        points: seriesPoints(historyQuery.data, "server.out_msgs_total"),
      },
    ],
    [historyQuery.data],
  );

  return (
    <div className="page">
      <PageHeader
        eyebrow="Overview"
        title="Dashboard"
        subtitle="Live JetStream health and capacity for your active cluster."
        badge={
          cluster ? (
            <span className="badge badge--live">
              {cluster.name}
              {isFetching && <span className="badge__pulse" aria-label="Refreshing" />}
            </span>
          ) : undefined
        }
      />

      <Alert variant="error">{error}</Alert>

      {loading && <div className="skeleton skeleton--table" />}

      {account && (
        <section className="section">
          <h2 className="section__title">Account usage</h2>
          <div className="stat-grid">
            <StatCard label="Streams" value={account.streams} accent="sky" icon="◎" />
            <StatCard label="Consumers" value={account.consumers} accent="violet" icon="◉" />
            <StatCard
              label="Storage"
              value={formatBytes(account.storage)}
              hint={`Limit ${formatBytes(account.limits.maxStorage)}`}
              accent="emerald"
              icon="▣"
            />
            <StatCard
              label="Memory"
              value={formatBytes(account.memory)}
              hint={`Limit ${formatBytes(account.limits.maxMemory)}`}
              accent="amber"
              icon="△"
            />
          </div>
        </section>
      )}

      {jsz?.total && (
        <section className="section">
          <h2 className="section__title">JetStream totals</h2>
          <div className="stat-grid stat-grid--3">
            <StatCard label="JSZ Streams" value={jsz.total.streams ?? 0} accent="sky" />
            <StatCard label="JSZ Consumers" value={jsz.total.consumers ?? 0} accent="violet" />
            <StatCard label="Messages" value={jsz.total.messages ?? 0} accent="emerald" />
          </div>
        </section>
      )}

      {clusterId && (
        <section className="section">
          <div className="section__header">
            <h2 className="section__title">Trends</h2>
            <TimeRangeSelector value={range} onChange={setRange} />
          </div>
          <Suspense fallback={<div className="skeleton skeleton--chart" />}>
            <div className="metrics-chart-grid">
              <MetricsTimeSeriesChart title="Storage & memory" series={capacitySeries} variant="area" />
              <MetricsTimeSeriesChart title="Messages" series={messagesSeries} />
              <MetricsTimeSeriesChart title="Connections & message rate" series={trafficSeries} />
            </div>
          </Suspense>
        </section>
      )}

      {varz && (
        <section className="section">
          <div className="panel">
            <div className="panel__header">
              <h2 className="panel__title">Server</h2>
              <span className="chip chip--success">Online</span>
            </div>
            <p className="panel__lead">{String(varz.server_name ?? "unknown")}</p>
            <p className="text-muted">
              Version {String(varz.version ?? "—")} · Uptime {String(varz.uptime ?? "—")}
            </p>
          </div>
        </section>
      )}
    </div>
  );
}
