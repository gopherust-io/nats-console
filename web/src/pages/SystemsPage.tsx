import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import Alert from "../components/ui/Alert";
import QueryErrorState from "../components/ui/QueryErrorState";
import { api, clusterPath } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { SYSTEMS_CONNECTIONS_POLL_MS, MONITORING_POLL_MS } from "../lib/constants";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";

type ConnStatus = {
  clusterId: string;
  connected: boolean;
};

export default function SystemsPage() {
  const { t } = useTranslation();
  const { clusters, loading, error } = useCluster();

  const connectionsQuery = useQuery({
    queryKey: ["clusters", "connections"],
    queryFn: async () => (await api<ConnStatus[]>("/api/v1/clusters/connections")).data ?? [],
    refetchInterval: visibilityAwareInterval(SYSTEMS_CONNECTIONS_POLL_MS),
  });

  const statusById = useMemo(() => {
    const map = new Map<string, ConnStatus>();
    for (const c of connectionsQuery.data ?? []) {
      map.set(c.clusterId, c);
    }
    return map;
  }, [connectionsQuery.data]);

  return (
    <div>
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("systems.title")}</h1>
          <p className="nc-page-sub">{t("systems.subtitle")}</p>
        </div>
      </div>

      {error && <Alert variant="error">{error}</Alert>}
      {connectionsQuery.isError && (
        <QueryErrorState
          error={connectionsQuery.error}
          title={t("errors.connectionsStatus")}
          onRetry={() => void connectionsQuery.refetch()}
        />
      )}
      {loading && <p className="text-muted">{t("systems.loading")}</p>}

      <div className="nc-card-grid">
        <Link className="nc-system-card" to="/systems/clusters">
          <div className="nc-system-card__body">
            <div>
              <div className="nc-system-card__name">{t("nav.clusters")}</div>
              <div className="nc-system-card__meta">
                <span>{t("systems.clustersCardDesc")}</span>
              </div>
            </div>
          </div>
        </Link>
        {clusters.map((cluster) => {
          const connected = statusById.get(cluster.id)?.connected === true;
          return (
            <Link key={cluster.id} className="nc-system-card" to={`/systems/${cluster.id}`}>
              <div className="nc-system-card__body">
                <div>
                  <div className="nc-system-card__name">{cluster.name}</div>
                  <div className="nc-system-card__meta">
                    <span>{cluster.isDefault ? t("systems.defaultSystem") : t("systems.natsCluster")}</span>
                  </div>
                </div>
                <span className={connected ? "nc-connected" : "nc-disconnected"}>
                  {connected ? t("systems.connected") : t("systems.disconnected")}
                </span>
              </div>
            </Link>
          );
        })}
      </div>
    </div>
  );
}

export function SystemUsagePage() {
  const { t } = useTranslation();
  const { clusterId } = useCluster();
  const accountQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "account"),
    queryFn: async () => (await api(clusterPath(clusterId!, "/account"))).data,
    enabled: Boolean(clusterId),
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS),
  });

  const account = accountQuery.data as {
    streams?: number;
    consumers?: number;
    storage?: number;
    memory?: number;
    limits?: { maxStreams?: number; maxConsumers?: number; maxStorage?: number; maxMemory?: number };
  } | null;

  function pct(used = 0, max = 0) {
    if (!max || max < 0) return t("common.emDash");
    return `${Math.min(100, (used / max) * 100).toFixed(1)}%`;
  }

  return (
    <div>
      <h1 className="nc-page-title">{t("systems.usageTitle")}</h1>
      <p className="nc-page-sub">{t("systems.usageSubtitle")}</p>
      {accountQuery.isError && (
        <QueryErrorState error={accountQuery.error} onRetry={() => void accountQuery.refetch()} />
      )}
      <div className="nc-usage-grid">
        <div className="nc-usage-card">
          <div className="nc-usage-card__label">{t("systems.streams")}</div>
          <div className="nc-usage-card__ring">{pct(account?.streams, account?.limits?.maxStreams)}</div>
          <div className="nc-usage-card__tier">
            {account?.streams ?? 0} / {account?.limits?.maxStreams ?? "∞"}
          </div>
        </div>
        <div className="nc-usage-card">
          <div className="nc-usage-card__label">{t("systems.consumers")}</div>
          <div className="nc-usage-card__ring">{pct(account?.consumers, account?.limits?.maxConsumers)}</div>
          <div className="nc-usage-card__tier">
            {account?.consumers ?? 0} / {account?.limits?.maxConsumers ?? "∞"}
          </div>
        </div>
        <div className="nc-usage-card">
          <div className="nc-usage-card__label">{t("systems.fileStorage")}</div>
          <div className="nc-usage-card__ring">{pct(account?.storage, account?.limits?.maxStorage)}</div>
        </div>
        <div className="nc-usage-card">
          <div className="nc-usage-card__label">{t("systems.memoryStorage")}</div>
          <div className="nc-usage-card__ring">{pct(account?.memory, account?.limits?.maxMemory)}</div>
        </div>
      </div>
    </div>
  );
}
