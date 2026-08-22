import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import Alert from "../components/ui/Alert";
import QueryErrorState from "../components/ui/QueryErrorState";
import { useConnectionEvents } from "../hooks/useConnectionEvents";
import { useAccount } from "../lib/account";
import { api, type NATSConnectionStatus } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { SYSTEMS_CONNECTIONS_POLL_MS } from "../lib/constants";
import { formatDateTime } from "../lib/datetime";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";

function SystemCardIcon({ isDefault }: { isDefault: boolean }) {
  return (
    <span className="nc-system-card__icon" aria-hidden="true">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        {isDefault ? (
          <>
            <rect x="3" y="4" width="18" height="6" rx="1.5" />
            <rect x="3" y="14" width="18" height="6" rx="1.5" />
            <circle cx="7" cy="7" r="1" fill="currentColor" stroke="none" />
            <circle cx="7" cy="17" r="1" fill="currentColor" stroke="none" />
          </>
        ) : (
          <>
            <circle cx="12" cy="12" r="3" />
            <path
              d="M12 3v2M12 19v2M3 12h2M19 12h2M5.6 5.6l1.4 1.4M17 17l1.4 1.4M5.6 18.4l1.4-1.4M17 7l1.4-1.4"
              strokeLinecap="round"
            />
          </>
        )}
      </svg>
    </span>
  );
}

export default function SystemsPage() {
  const { t } = useTranslation();
  const { clusters, clusterId, loading, error } = useCluster();
  const { accounts } = useAccount();

  useConnectionEvents(clusterId);

  const connectionQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "connection"),
    queryFn: async () =>
      (await api<NATSConnectionStatus>(`/api/v1/clusters/${encodeURIComponent(clusterId!)}/connection`))
        .data,
    enabled: Boolean(clusterId),
  });

  const connectionsQuery = useQuery({
    queryKey: ["clusters", "connections"],
    queryFn: async () =>
      (await api<NATSConnectionStatus[]>("/api/v1/clusters/connections")).data ?? [],
    refetchInterval: visibilityAwareInterval(SYSTEMS_CONNECTIONS_POLL_MS),
  });

  const statusById = useMemo(() => {
    const map = new Map<string, NATSConnectionStatus>();
    for (const item of connectionsQuery.data ?? []) {
      map.set(item.clusterId, item);
    }
    if (clusterId && connectionQuery.data) {
      map.set(clusterId, connectionQuery.data);
    }
    return map;
  }, [connectionsQuery.data, connectionQuery.data, clusterId]);

  return (
    <div className="nc-systems-page">
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("systems.title")}</h1>
        </div>
      </div>

      {error && <Alert variant="error">{error}</Alert>}
      {connectionQuery.isError && (
        <QueryErrorState
          error={connectionQuery.error}
          title={t("errors.connectionsStatus")}
          onRetry={() => void connectionQuery.refetch()}
        />
      )}
      {connectionsQuery.isError && !connectionQuery.isError && (
        <QueryErrorState
          error={connectionsQuery.error}
          title={t("errors.connectionsStatus")}
          onRetry={() => void connectionsQuery.refetch()}
        />
      )}
      {loading && <p className="text-muted">{t("systems.loading")}</p>}

      <div className="nc-card-grid nc-systems-page__grid">
        {clusters.map((item) => {
          const accountCount = item.id === clusterId ? accounts.length : Math.max(accounts.length, 1);
          const status = statusById.get(item.id);
          const connected = status?.connected === true;
          const known = status !== undefined;
          const jetStreamOk = status?.jetstreamOk === true;
          return (
            <Link key={item.id} className="nc-system-card" to={`/systems/${item.id}`}>
              <div className="nc-system-card__top">
                <SystemCardIcon isDefault={item.isDefault} />
                {connected ? (
                  <span
                    className="nc-conn-live"
                    title={t("systems.availabilityHint")}
                    aria-label={t("account.clusterStatus.available")}
                  >
                    <span className="nc-conn-live__dot" aria-hidden="true" />
                    {t("account.clusterStatus.live")}
                  </span>
                ) : known ? (
                  <span
                    className="nc-conn-live nc-conn-live--down"
                    title={t("systems.unavailabilityHint")}
                    aria-label={t("account.clusterStatus.unavailable")}
                  >
                    <span className="nc-conn-live__dot" aria-hidden="true" />
                    {t("account.clusterStatus.unavailable")}
                  </span>
                ) : (
                  <span className="nc-conn-status" aria-label={t("account.unknownStatus")}>
                    <span className="nc-conn-status__dot" aria-hidden="true" />
                    {t("account.unknownStatus")}
                  </span>
                )}
              </div>
              <div className="nc-system-card__body">
                <div className="nc-system-card__name">{item.name}</div>
                <div className="nc-system-card__meta">
                  <span>{item.isDefault ? t("systems.defaultSystem") : t("systems.natsCluster")}</span>
                  <span aria-hidden="true">·</span>
                  <span>{t("systems.accountsCount", { count: accountCount })}</span>
                </div>
              </div>
              <div
                className="nc-system-card__stats"
                aria-label={t("account.clusterStatus.title")}
              >
                <div className="nc-system-card__stat">
                  <span className="nc-system-card__stat-label">{t("account.clusterStatus.jetStream")}</span>
                  <span
                    className={`nc-system-card__stat-value mono${
                      !known
                        ? ""
                        : jetStreamOk
                          ? " nc-system-card__stat-value--ok"
                          : " nc-system-card__stat-value--danger"
                    }`}
                  >
                    {!known
                      ? t("common.emDash")
                      : jetStreamOk
                        ? t("common.enabled")
                        : t("account.clusterStatus.jetStreamDown")}
                  </span>
                </div>
                <div className="nc-system-card__stat">
                  <span className="nc-system-card__stat-label">{t("account.clusterStatus.server")}</span>
                  <span className="nc-system-card__stat-value mono">
                    {status?.serverName || t("common.emDash")}
                  </span>
                </div>
                <div className="nc-system-card__stat">
                  <span className="nc-system-card__stat-label">{t("account.clusterStatus.lastChecked")}</span>
                  <span className="nc-system-card__stat-value mono">
                    {formatDateTime(status?.lastCheckedAt, t("common.emDash"))}
                  </span>
                </div>
              </div>
              {status?.lastError ? (
                <p className="nc-system-card__error" role="status">
                  <span className="nc-system-card__stat-label">{t("account.clusterStatus.lastError")}</span>
                  {status.lastError}
                </p>
              ) : null}
            </Link>
          );
        })}
      </div>
    </div>
  );
}
