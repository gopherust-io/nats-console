import { lazy, Suspense, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import TimeRangeSelector from "../components/metrics/TimeRangeSelector";
import QueryErrorState from "../components/ui/QueryErrorState";
import { useClusterMetricsHistory } from "../hooks/useClusterMetricsHistory";
import { AccountInfo, api, clusterPath } from "../lib/api";
import { MetricsRangePreset } from "../lib/metricsHistory";
import { useCluster } from "../lib/cluster";
import { MONITORING_POLL_MS, SYSTEMS_CONNECTIONS_POLL_MS } from "../lib/constants";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";
import { formatLatencyMs, type RequestReplySnapshot } from "../lib/requestReplyInspector";
import { fetchTopology, type TopologyNode } from "../lib/topology";

const MetricsTimeSeriesChart = lazy(() => import("../components/metrics/MetricsTimeSeriesChart"));

// Account-scoped gauges from JetStream AccountInfo (connected account), not cluster /varz counters.
const metrics =
  "jetstream.streams,jetstream.consumers,jetstream.storage_bytes,jetstream.memory_bytes,jetstream.consumer_max_lag,server.cpu_percent,server.mem_bytes";

type TopologyHealth = "healthy" | "warning" | "unhealthy" | "unknown";

type TopologyKpis = {
  leaders: number;
  replicas: number;
  health: TopologyHealth;
  connected: boolean | null;
  slowConsumers: number;
  issues: string[];
};

function seriesPoints(
  history: ReturnType<typeof useClusterMetricsHistory>["data"],
  metric: string,
) {
  return history?.series.find((item) => item.metric === metric)?.points ?? [];
}

function lastValue(points: { v: number }[]) {
  return points.length ? points[points.length - 1].v : 0;
}

function formatBytes(value: number) {
  if (value < 1024) return `${Math.round(value).toLocaleString()} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  return `${(value / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function formatLimitCount(used = 0, max = 0) {
  const limit = max > 0 ? max.toLocaleString() : "∞";
  return `${Math.round(used).toLocaleString()} / ${limit}`;
}

function formatLimitBytes(used = 0, max = 0) {
  const limit = max > 0 ? formatBytes(max) : "∞";
  return `${formatBytes(used)} / ${limit}`;
}

function formatCount(value: number) {
  return Math.round(value).toLocaleString();
}

function formatPercent(value: number) {
  if (!Number.isFinite(value)) return "—";
  return `${value.toFixed(1)}%`;
}

function walkTopology(node: TopologyNode, visit: (item: TopologyNode) => void) {
  visit(node);
  for (const child of node.children ?? []) {
    walkTopology(child, visit);
  }
}

function deriveClusterHealth(root: TopologyNode | null, connected: boolean | null): TopologyKpis {
  const issues: string[] = [];
  let leaders = 0;
  let replicas = 0;
  let slowConsumers = 0;

  if (root) {
    walkTopology(root, (node) => {
      if (node.role === "leader") leaders += 1;
      if (node.role === "replica") replicas += 1;
      if (node.kind === "consumer" && node.status === "warning") {
        slowConsumers += 1;
        return;
      }
      if (node.kind !== "cluster" && node.kind !== "stream") return;

      if (node.status === "unhealthy") {
        issues.push(`${node.kind}:${node.name}`);
      }
      for (const peer of node.raft?.peers ?? []) {
        if (peer.offline) {
          issues.push(`${node.name}:${peer.name}`);
        }
      }
      if ((node.raft?.clusterSize ?? 0) > 1 && !node.raft?.leader) {
        issues.push(`${node.name}:no-leader`);
      }
    });
  }

  if (connected === false) {
    return {
      leaders,
      replicas,
      health: "unhealthy",
      connected,
      slowConsumers,
      issues: ["connection"],
    };
  }
  if (!root) {
    return {
      leaders,
      replicas,
      health: "unknown",
      connected,
      slowConsumers,
      issues,
    };
  }
  if (issues.length > 0) {
    return {
      leaders,
      replicas,
      health: "unhealthy",
      connected,
      slowConsumers,
      issues,
    };
  }
  if (connected === true) {
    return {
      leaders,
      replicas,
      health: "healthy",
      connected,
      slowConsumers,
      issues,
    };
  }
  return {
    leaders,
    replicas,
    health: "unknown",
    connected,
    slowConsumers,
    issues,
  };
}

export default function AccountOverviewPage() {
  const { t } = useTranslation();
  const { accountName, clusterId: routeCluster } = useParams();
  const { clusterId: contextClusterId, cluster } = useCluster();
  const clusterId = routeCluster ?? contextClusterId;
  const [range, setRange] = useState<MetricsRangePreset>("1h");

  const accountQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "account"),
    queryFn: async () => (await api<AccountInfo>(clusterPath(clusterId!, "/account"))).data,
    enabled: Boolean(clusterId),
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS),
  });

  const connectionsQuery = useQuery({
    queryKey: ["clusters", "connections"],
    queryFn: async () =>
      (await api<{ clusterId: string; connected: boolean }[]>("/api/v1/clusters/connections")).data ?? [],
    refetchInterval: visibilityAwareInterval(SYSTEMS_CONNECTIONS_POLL_MS),
  });

  const historyQuery = useClusterMetricsHistory(clusterId, range, metrics);
  const topologyQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "topology-overview"),
    queryFn: () => fetchTopology(clusterId!, cluster?.name ?? "Cluster", { fresh: true }),
    enabled: Boolean(clusterId),
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS),
  });

  const requestReplyQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "request-reply"),
    queryFn: async () => (await api<RequestReplySnapshot>(clusterPath(clusterId!, "/request-reply"))).data,
    enabled: Boolean(clusterId),
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS),
  });

  const streams = seriesPoints(historyQuery.data, "jetstream.streams");
  const consumers = seriesPoints(historyQuery.data, "jetstream.consumers");
  const storage = seriesPoints(historyQuery.data, "jetstream.storage_bytes");
  const memory = seriesPoints(historyQuery.data, "jetstream.memory_bytes");
  const maxLag = seriesPoints(historyQuery.data, "jetstream.consumer_max_lag");
  const cpu = seriesPoints(historyQuery.data, "server.cpu_percent");
  const serverMemory = seriesPoints(historyQuery.data, "server.mem_bytes");
  const account = accountQuery.data;
  const clusterConnected = useMemo(() => {
    if (!clusterId) return null;
    const match = connectionsQuery.data?.find((item) => item.clusterId === clusterId);
    return match?.connected ?? null;
  }, [connectionsQuery.data, clusterId]);
  const topologyKpis = useMemo(
    () => deriveClusterHealth(topologyQuery.data ?? null, clusterConnected),
    [topologyQuery.data, clusterConnected],
  );
  const topologyHealth = t(`account.clusterTopology.health.${topologyKpis.health}`);
  const connectionLabel =
    topologyKpis.connected === true
      ? t("account.clusterTopology.connectionConnected")
      : topologyKpis.connected === false
        ? t("account.clusterTopology.connectionDisconnected")
        : t("account.unknownStatus");

  const rr = requestReplyQuery.data;
  const hasRrParticipants = (rr?.requesters ?? 0) > 0 || (rr?.responders ?? 0) > 0;

  const chartPairs = useMemo(
    () => [
      { title: t("account.streams"), value: formatCount(lastValue(streams)), key: "streams", points: streams },
      { title: t("account.consumers"), value: formatCount(lastValue(consumers)), key: "consumers", points: consumers },
      { title: t("account.storage"), value: formatBytes(lastValue(storage)), key: "storage", points: storage },
      { title: t("account.memory"), value: formatBytes(lastValue(memory)), key: "memory", points: memory },
    ],
    [streams, consumers, storage, memory, t],
  );

  return (
    <div>
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("account.overviewTitle")}</h1>
          <p className="nc-page-sub">{t("account.overviewSubtitle")}</p>
        </div>
        <TimeRangeSelector value={range} onChange={setRange} />
      </div>

      {accountQuery.isError && (
        <QueryErrorState error={accountQuery.error} onRetry={() => void accountQuery.refetch()} />
      )}
      {topologyQuery.isError && (
        <QueryErrorState error={topologyQuery.error} onRetry={() => void topologyQuery.refetch()} />
      )}
      {requestReplyQuery.isError && (
        <QueryErrorState error={requestReplyQuery.error} onRetry={() => void requestReplyQuery.refetch()} />
      )}

      <div className="nc-metrics-grid">
        {chartPairs.map((card) => (
          <div className="nc-metric-card" key={card.key}>
            <h3>{card.title}</h3>
            <p className="nc-metric-card__value">{card.value}</p>
            <Suspense fallback={null}>
              <MetricsTimeSeriesChart
                title={card.title}
                series={[{ key: card.key, label: card.title, color: "var(--accent)", points: card.points }]}
              />
            </Suspense>
          </div>
        ))}
      </div>

      <div className="nc-overview-sections">
      <section className="nc-overview-section nc-meta-card">
        <h4>{t("account.clusterTopology.title")}</h4>
        <p className="nc-page-sub">{t("account.clusterTopology.subtitle")}</p>
        <div className="nc-metrics-grid nc-metrics-grid--embedded">
          <div className="nc-metric-card nc-metric-card--compact">
            <h3>{t("account.clusterTopology.leaders")}</h3>
            <p className="nc-metric-card__value">{formatCount(topologyKpis.leaders)}</p>
          </div>
          <div className="nc-metric-card nc-metric-card--compact">
            <h3>{t("account.clusterTopology.replicas")}</h3>
            <p className="nc-metric-card__value">{formatCount(topologyKpis.replicas)}</p>
          </div>
          <div className="nc-metric-card nc-metric-card--compact">
            <h3>{t("account.clusterTopology.healthLabel")}</h3>
            <p className={`nc-metric-card__value nc-metric-card__value--status nc-health-${topologyKpis.health}`}>
              {topologyHealth}
            </p>
            <p className="nc-metric-card__hint">{connectionLabel}</p>
            {topologyKpis.slowConsumers > 0 && (
              <p className="nc-metric-card__hint">
                {t("account.clusterTopology.slowConsumers", { count: topologyKpis.slowConsumers })}
              </p>
            )}
          </div>
          <div className="nc-metric-card nc-metric-card--compact">
            <h3>{t("account.clusterTopology.latency")}</h3>
            <p className="nc-metric-card__value">{formatCount(lastValue(maxLag))}</p>
          </div>
          <div className="nc-metric-card nc-metric-card--compact">
            <h3>{t("account.clusterTopology.cpu")}</h3>
            <p className="nc-metric-card__value">{formatPercent(lastValue(cpu))}</p>
          </div>
          <div className="nc-metric-card nc-metric-card--compact">
            <h3>{t("account.clusterTopology.memory")}</h3>
            <p className="nc-metric-card__value">{formatBytes(lastValue(serverMemory))}</p>
          </div>
        </div>
      </section>

      <section className="nc-overview-section nc-meta-card">
        <h4>{t("account.requestReply.title")}</h4>
        <p className="nc-page-sub">{t("account.requestReply.subtitle")}</p>
        <div className="nc-metrics-grid nc-metrics-grid--embedded">
          <div className="nc-metric-card nc-metric-card--compact">
            <h3>{t("account.requestReply.requesters")}</h3>
            <p className="nc-metric-card__value">
              {hasRrParticipants ? formatCount(rr?.requesters ?? 0) : t("common.emDash")}
            </p>
          </div>
          <div className="nc-metric-card nc-metric-card--compact">
            <h3>{t("account.requestReply.responders")}</h3>
            <p className="nc-metric-card__value">
              {hasRrParticipants ? formatCount(rr?.responders ?? 0) : t("common.emDash")}
            </p>
          </div>
          <div className="nc-metric-card nc-metric-card--compact">
            <h3>{t("account.requestReply.medianRtt")}</h3>
            <p className="nc-metric-card__value">
              {hasRrParticipants ? formatLatencyMs(rr?.medianRttMs ?? null) : t("common.emDash")}
            </p>
          </div>
        </div>
      </section>

      <section className="nc-overview-section nc-meta-card">
        <h4>{accountName ?? t("account.accountFallback")}</h4>
        <p className="nc-page-sub">{t("account.settingsSubtitle")}</p>

        <div className="nc-overview-subsection">
          <h5>{t("account.general")}</h5>
          <p>{t("account.generalHelp")}</p>
          <div className="nc-form-row">
            <label htmlFor="overview-account-name">{t("account.accountName")}</label>
            <input id="overview-account-name" value={accountName ?? t("account.accountFallback")} readOnly disabled />
          </div>
        </div>

        <div className="nc-overview-subsection">
          <h5>{t("account.limits")}</h5>
          <p>{t("account.limitsHelp")}</p>
          <div className="nc-meta-row">
            <span>{t("account.streams")}</span>
            <span>{formatLimitCount(account?.streams, account?.limits?.maxStreams)}</span>
          </div>
          <div className="nc-meta-row">
            <span>{t("account.consumers")}</span>
            <span>{formatLimitCount(account?.consumers, account?.limits?.maxConsumers)}</span>
          </div>
          <div className="nc-meta-row">
            <span>{t("account.diskStorage")}</span>
            <span>{formatLimitBytes(account?.storage, account?.limits?.maxStorage)}</span>
          </div>
          <div className="nc-meta-row">
            <span>{t("systems.memoryStorage")}</span>
            <span>{formatLimitBytes(account?.memory, account?.limits?.maxMemory)}</span>
          </div>
        </div>

        <div className="nc-overview-subsection">
          <h5>{t("account.jetStreamSection")}</h5>
          <div className="nc-meta-row">
            <span>{t("account.jetStreamEnabled")}</span>
            <span>{account ? t("common.enabled") : t("account.unknownStatus")}</span>
          </div>
        </div>
      </section>
      </div>
    </div>
  );
}
