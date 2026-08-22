import { lazy, Suspense, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import TimeRangeSelector from "../components/metrics/TimeRangeSelector";
import Alert from "../components/ui/Alert";
import ClockNumber from "../components/ui/ClockNumber";
import PageLoader from "../components/ui/PageLoader";
import QueryErrorState from "../components/ui/QueryErrorState";
import { useAccountOverviewEvents } from "../hooks/useAccountOverviewEvents";
import { useClusterMetricsHistory } from "../hooks/useClusterMetricsHistory";
import { AccountInfo, api, clusterPath } from "../lib/api";
import { METRICS_HISTORY_POLL_MS } from "../lib/constants";
import { MetricsHistoryPoint, MetricsRangePreset } from "../lib/metricsHistory";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";
import { formatLatencyMs, type RequestReplySnapshot } from "../lib/requestReplyInspector";
import "../styles/replicas.css";
import "../styles/account.css";

const MetricsTimeSeriesChart = lazy(() => import("../components/metrics/MetricsTimeSeriesChart"));

// Account-scoped gauges from JetStream AccountInfo (connected account), not cluster /varz counters.
const metrics =
  "jetstream.streams,jetstream.consumers,jetstream.storage_bytes,jetstream.memory_bytes,jetstream.consumer_max_lag,server.cpu_percent,server.mem_bytes";

function seriesPoints(
  history: ReturnType<typeof useClusterMetricsHistory>["data"],
  metric: string,
) {
  return history?.series.find((item) => item.metric === metric)?.points ?? [];
}

function withLiveTail(points: MetricsHistoryPoint[], live: number | undefined): MetricsHistoryPoint[] {
  if (live == null || Number.isNaN(live)) return points;
  const last = points.length ? points[points.length - 1] : null;
  if (last && last.v === live) return points;
  return [...points, { t: new Date().toISOString(), v: live }];
}

function formatBytes(value: number) {
  if (value < 1024) return `${Math.round(value).toLocaleString()}B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)}KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)}MB`;
  return `${(value / (1024 * 1024 * 1024)).toFixed(2)}GB`;
}

function formatLimitCount(used = 0, max = 0) {
  const limit = max > 0 ? max.toLocaleString() : "∞";
  return `${Math.round(used).toLocaleString()} / ${limit}`;
}

function formatLimitBytes(used = 0, max = 0) {
  const limit = max > 0 ? formatBytes(max) : "∞";
  return `${formatBytes(used)} / ${limit}`;
}

function usagePct(used: number, max: number) {
  if (max <= 0) return 0;
  return Math.min(100, Math.round((used / max) * 100));
}

function usageTone(pct: number, hasLimit: boolean): "ok" | "warn" | "danger" {
  if (!hasLimit) return "ok";
  if (pct >= 90) return "danger";
  if (pct >= 70) return "warn";
  return "ok";
}

export default function AccountOverviewPage() {
  const { t } = useTranslation();
  const { accountName, clusterId: routeCluster } = useParams();
  const { clusterId: contextClusterId } = useCluster();
  const clusterId = routeCluster ?? contextClusterId;
  const [range, setRange] = useState<MetricsRangePreset>("1h");

  const { live } = useAccountOverviewEvents(clusterId);

  const accountQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "account"),
    queryFn: async () => (await api<AccountInfo>(clusterPath(clusterId!, "/account"))).data,
    enabled: Boolean(clusterId),
    staleTime: 5_000,
    refetchOnWindowFocus: !live,
    refetchInterval: live ? false : visibilityAwareInterval(5_000),
  });

  const historyQuery = useClusterMetricsHistory(clusterId, range, metrics, {
    // SSE already pushes live gauges; history only needs a slow backfill when live.
    refetchInterval: live ? false : METRICS_HISTORY_POLL_MS,
  });

  const requestReplyQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "request-reply"),
    queryFn: async () => (await api<RequestReplySnapshot>(clusterPath(clusterId!, "/request-reply"))).data,
    enabled: Boolean(clusterId),
    staleTime: 5_000,
    refetchOnWindowFocus: !live,
    refetchInterval: live ? false : visibilityAwareInterval(5_000),
  });

  const account = accountQuery.data;
  const streamPoints = withLiveTail(seriesPoints(historyQuery.data, "jetstream.streams"), account?.streams);
  const consumerPoints = withLiveTail(
    seriesPoints(historyQuery.data, "jetstream.consumers"),
    account?.consumers,
  );
  const storagePoints = withLiveTail(
    seriesPoints(historyQuery.data, "jetstream.storage_bytes"),
    account?.storage,
  );
  const memoryPoints = withLiveTail(
    seriesPoints(historyQuery.data, "jetstream.memory_bytes"),
    account?.memory,
  );

  // Tiles always prefer live AccountInfo (SSE/REST), never history last-point.
  const streams = account?.streams ?? 0;
  const consumers = account?.consumers ?? 0;
  const storage = account?.storage ?? 0;
  const memory = account?.memory ?? 0;

  const rr = requestReplyQuery.data;
  const hasRrParticipants = (rr?.requesters ?? 0) > 0 || (rr?.responders ?? 0) > 0;
  const emDash = t("common.emDash");

  const chartPairs = useMemo(
    () => [
      {
        title: t("account.streams"),
        key: "streams",
        points: streamPoints,
      },
      {
        title: t("account.consumers"),
        key: "consumers",
        points: consumerPoints,
      },
      {
        title: t("account.storage"),
        key: "storage",
        points: storagePoints,
      },
      {
        title: t("account.memory"),
        key: "memory",
        points: memoryPoints,
      },
    ],
    [streamPoints, consumerPoints, storagePoints, memoryPoints, t],
  );

  const loading = accountQuery.isLoading && !account;

  return (
    <div className="replicas-page account-page">
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("account.overviewTitle")}</h1>
          <p className="nc-page-sub">{t("account.overviewSubtitle")}</p>
        </div>
        <TimeRangeSelector value={range} onChange={setRange} />
      </div>

      {loading && <PageLoader />}

      {accountQuery.isError && (
        <QueryErrorState error={accountQuery.error} onRetry={() => void accountQuery.refetch()} />
      )}
      {requestReplyQuery.isError && (
        <QueryErrorState error={requestReplyQuery.error} onRetry={() => void requestReplyQuery.refetch()} />
      )}

      {!loading && account && !live && (
        <Alert variant="info">{t("account.staleSnapshot")}</Alert>
      )}

      {!loading && (
        <>
          <div className="replicas-summary">
            <div className="replicas-card replicas-stat-card">
              <span className="replicas-card__badge">{t("account.streams")}</span>
              <div className="replicas-card__body">
                <div className="replicas-stat-card__value mono">
                  <ClockNumber value={Math.round(streams)} />
                </div>
              </div>
            </div>
            <div className="replicas-card replicas-stat-card">
              <span className="replicas-card__badge">{t("account.consumers")}</span>
              <div className="replicas-card__body">
                <div className="replicas-stat-card__value mono">
                  <ClockNumber value={Math.round(consumers)} />
                </div>
              </div>
            </div>
            <div className="replicas-card replicas-stat-card">
              <span className="replicas-card__badge">{t("account.storage")}</span>
              <div className="replicas-card__body">
                <div className="replicas-stat-card__value mono">
                  <ClockNumber value={Math.round(storage)} format={formatBytes} />
                </div>
              </div>
            </div>
            <div className="replicas-card replicas-stat-card">
              <span className="replicas-card__badge">{t("account.memory")}</span>
              <div className="replicas-card__body">
                <div className="replicas-stat-card__value mono">
                  <ClockNumber value={Math.round(memory)} format={formatBytes} />
                </div>
              </div>
            </div>
          </div>

          <div className="account-charts-grid">
            {chartPairs.map((card) => (
              <div className="replicas-card account-chart-card" key={card.key}>
                <span className="replicas-card__badge">{card.title}</span>
                <div className="replicas-card__body account-chart-card__body">
                  <Suspense fallback={null}>
                    <MetricsTimeSeriesChart
                      title={card.title}
                      embedded
                      series={[
                        { key: card.key, label: card.title, color: "var(--accent)", points: card.points },
                      ]}
                    />
                  </Suspense>
                </div>
              </div>
            ))}
          </div>

          <div className="replicas-panel">
            <div className="replicas-panel__head">
              <h2 className="replicas-panel__title">{t("account.requestReply.title")}</h2>
            </div>
            <p className="raft-election__caption">{t("account.requestReply.subtitle")}</p>
            <div className="account-rr-grid">
              <div className="replicas-card replicas-stat-card">
                <span className="replicas-card__badge">{t("account.requestReply.requesters")}</span>
                <div className="replicas-card__body">
                  <div className="replicas-stat-card__value mono">
                    {hasRrParticipants ? (
                      <ClockNumber value={Math.round(rr?.requesters ?? 0)} />
                    ) : (
                      emDash
                    )}
                  </div>
                </div>
              </div>
              <div className="replicas-card replicas-stat-card">
                <span className="replicas-card__badge">{t("account.requestReply.responders")}</span>
                <div className="replicas-card__body">
                  <div className="replicas-stat-card__value mono">
                    {hasRrParticipants ? (
                      <ClockNumber value={Math.round(rr?.responders ?? 0)} />
                    ) : (
                      emDash
                    )}
                  </div>
                </div>
              </div>
              <div className="replicas-card replicas-stat-card">
                <span className="replicas-card__badge">{t("account.requestReply.medianRtt")}</span>
                <div className="replicas-card__body">
                  <div className="replicas-stat-card__value mono">
                    {hasRrParticipants ? formatLatencyMs(rr?.medianRttMs ?? null) : emDash}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div className="replicas-panel account-identity-panel">
            <div className="replicas-panel__head account-identity-panel__head">
              <div className="account-identity-panel__title-row">
                <h2 className="replicas-panel__title">{accountName ?? t("account.accountFallback")}</h2>
                {accountName === "Default" ? (
                  <span className="account-identity-chip account-identity-chip--default">
                    {t("systems.defaultAccount")}
                  </span>
                ) : (
                  <span className="account-identity-chip">{t("systems.natsAccount")}</span>
                )}
                {account ? (
                  <span className="account-identity-chip account-identity-chip--ok">
                    {t("account.jetStreamOn")}
                  </span>
                ) : null}
                {live ? (
                  <span className="nc-conn-live" title={t("account.clusterStatus.liveHint")}>
                    <span className="nc-conn-live__dot" aria-hidden="true" />
                    {t("account.clusterStatus.live")}
                  </span>
                ) : account ? (
                  <span className="nc-conn-status nc-conn-status--warn" title={t("account.staleSnapshot")}>
                    <span className="nc-conn-status__dot" aria-hidden="true" />
                    {t("account.reconnecting")}
                  </span>
                ) : null}
              </div>
            </div>
            <p className="raft-election__caption">{t("account.limitsHelp")}</p>

            <div className="replicas-summary account-limit-cards" aria-label={t("account.limits")}>
              {[
                {
                  key: "streams",
                  label: t("account.streams"),
                  value: <ClockNumber value={Math.round(streams)} />,
                  hint: formatLimitCount(streams, account?.limits?.maxStreams),
                  used: streams,
                  max: account?.limits?.maxStreams ?? 0,
                },
                {
                  key: "consumers",
                  label: t("account.consumers"),
                  value: <ClockNumber value={Math.round(consumers)} />,
                  hint: formatLimitCount(consumers, account?.limits?.maxConsumers),
                  used: consumers,
                  max: account?.limits?.maxConsumers ?? 0,
                },
                {
                  key: "disk",
                  label: t("account.diskStorage"),
                  value: <ClockNumber value={Math.round(storage)} format={formatBytes} />,
                  hint: formatLimitBytes(storage, account?.limits?.maxStorage),
                  used: storage,
                  max: account?.limits?.maxStorage ?? 0,
                },
                {
                  key: "memory",
                  label: t("systems.memoryStorage"),
                  value: <ClockNumber value={Math.round(memory)} format={formatBytes} />,
                  hint: formatLimitBytes(memory, account?.limits?.maxMemory),
                  used: memory,
                  max: account?.limits?.maxMemory ?? 0,
                },
              ].map((item) => {
                const capped = item.max > 0;
                const pct = usagePct(item.used, item.max);
                const tone = usageTone(pct, capped);
                return (
                  <div className="replicas-card replicas-stat-card" key={item.key}>
                    <span className="replicas-card__badge">{item.label}</span>
                    <div className="replicas-card__body">
                      <div className="replicas-stat-card__value mono">{item.value}</div>
                      <p className="account-limit-card__hint mono">{item.hint}</p>
                      {capped ? (
                        <div
                          className={`account-limit-bar account-limit-bar--${tone}`}
                          role="meter"
                          aria-valuemin={0}
                          aria-valuemax={100}
                          aria-valuenow={pct}
                          aria-label={item.label}
                        >
                          <span className="account-limit-bar__fill" style={{ width: `${pct}%` }} />
                        </div>
                      ) : (
                        <span className="account-identity-chip account-identity-chip--unlimited">
                          {t("account.unlimited")}
                        </span>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </>
      )}
    </div>
  );
}
