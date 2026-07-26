import { lazy, Suspense, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import Alert from "../components/ui/Alert";
import TimeRangeSelector from "../components/metrics/TimeRangeSelector";
import { useClusterMetricsHistory } from "../hooks/useClusterMetricsHistory";
import { AccountInfo, api, clusterPath } from "../lib/api";
import { MetricsRangePreset } from "../lib/metricsHistory";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

const MetricsTimeSeriesChart = lazy(() => import("../components/metrics/MetricsTimeSeriesChart"));

// Account-scoped gauges from JetStream AccountInfo (connected account), not cluster /varz counters.
const metrics =
  "jetstream.streams,jetstream.consumers,jetstream.storage_bytes,jetstream.memory_bytes";

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

function formatCount(value: number) {
  return Math.round(value).toLocaleString();
}

export default function AccountOverviewPage() {
  const { t } = useTranslation();
  const { accountName } = useParams();
  const { clusterId } = useCluster();
  const [range, setRange] = useState<MetricsRangePreset>("1h");

  const accountQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "account"),
    queryFn: () => api<AccountInfo>(clusterPath(clusterId!, "/account")),
    enabled: Boolean(clusterId),
    refetchInterval: 30_000,
  });

  const historyQuery = useClusterMetricsHistory(clusterId, range, metrics);

  const streams = seriesPoints(historyQuery.data, "jetstream.streams");
  const consumers = seriesPoints(historyQuery.data, "jetstream.consumers");
  const storage = seriesPoints(historyQuery.data, "jetstream.storage_bytes");
  const memory = seriesPoints(historyQuery.data, "jetstream.memory_bytes");
  const account = accountQuery.data;

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

      {accountQuery.error instanceof Error && <Alert variant="error">{accountQuery.error.message}</Alert>}

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

      <div className="nc-meta-card">
        <h4>{accountName ?? t("account.accountFallback")}</h4>
        <div className="nc-meta-row">
          <span>{t("account.jetStreamEnabled")}</span>
          <span>{account ? t("common.yes") : t("account.unknownStatus")}</span>
        </div>
        <div className="nc-meta-row">
          <span>{t("account.streams")}</span>
          <span>
            {account?.streams ?? 0}
            {account?.limits?.maxStreams ? ` / ${account.limits.maxStreams}` : ""}
          </span>
        </div>
        <div className="nc-meta-row">
          <span>{t("account.consumers")}</span>
          <span>
            {account?.consumers ?? 0}
            {account?.limits?.maxConsumers ? ` / ${account.limits.maxConsumers}` : ""}
          </span>
        </div>
        <div className="nc-meta-row">
          <span>{t("account.storage")}</span>
          <span>{formatBytes(account?.storage ?? 0)}</span>
        </div>
        <div className="nc-meta-row">
          <span>{t("account.memory")}</span>
          <span>{formatBytes(account?.memory ?? 0)}</span>
        </div>
      </div>
    </div>
  );
}
