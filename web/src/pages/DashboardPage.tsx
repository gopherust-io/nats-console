import { useQuery } from "@tanstack/react-query";
import Alert from "../components/ui/Alert";
import PageHeader from "../components/ui/PageHeader";
import StatCard from "../components/ui/StatCard";
import { AccountInfo, api, clusterPath } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

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

export default function DashboardPage() {
  const { clusterId, cluster } = useCluster();
  const loading = !clusterId;

  const accountQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "account"),
    queryFn: () => api<AccountInfo>(clusterPath(clusterId!, "/account")),
    enabled: Boolean(clusterId),
    refetchInterval: 30_000,
  });

  const varzQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "varz"),
    queryFn: () => api<Record<string, unknown>>(clusterPath(clusterId!, "/monitoring/varz")),
    enabled: Boolean(clusterId),
    refetchInterval: 30_000,
  });

  const jszQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "jsz"),
    queryFn: () => api<JSZResponse>(clusterPath(clusterId!, "/monitoring/jsz?streams=1&consumers=1")),
    enabled: Boolean(clusterId),
    refetchInterval: 30_000,
  });

  const error =
    (accountQuery.error instanceof Error && accountQuery.error.message) ||
    (varzQuery.error instanceof Error && varzQuery.error.message) ||
    (jszQuery.error instanceof Error && jszQuery.error.message) ||
    "";

  const account = accountQuery.data ?? null;
  const varz = varzQuery.data ?? null;
  const jsz = jszQuery.data ?? null;
  const isFetching = accountQuery.isFetching || varzQuery.isFetching;

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
