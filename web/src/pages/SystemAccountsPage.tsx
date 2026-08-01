import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { useAccount } from "../lib/account";
import { AccountInfo, api, clusterPath } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { MONITORING_POLL_MS } from "../lib/constants";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";

export default function SystemAccountsPage() {
  const { t } = useTranslation();
  const { clusterId } = useParams();
  const { cluster } = useCluster();
  const { accounts } = useAccount();

  const accountQuery = useQuery({
    queryKey: clusterQueryKey(clusterId ?? null, "account"),
    queryFn: async () => (await api<AccountInfo>(clusterPath(clusterId!, "/account"))).data,
    enabled: Boolean(clusterId),
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS),
  });

  const varzQuery = useQuery({
    queryKey: clusterQueryKey(clusterId ?? null, "varz-lite"),
    queryFn: async () => (await api<Record<string, unknown>>(clusterPath(clusterId!, "/monitoring/varz"))).data,
    enabled: Boolean(clusterId),
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS),
  });

  const connections = Number(varzQuery.data?.connections ?? 0);
  const inMsgs = Number(varzQuery.data?.in_msgs ?? 0);
  const inBytes = Number(varzQuery.data?.in_bytes ?? 0);

  return (
    <div>
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("systems.accountsTitle")}</h1>
          <p className="nc-page-sub">
            {t("systems.accountsSubtitle", { name: cluster?.name ?? t("systems.thisSystem") })}
          </p>
        </div>
      </div>

      <div className="nc-card-grid">
        {accounts.map((account) => (
          <Link
            key={account.name}
            className="nc-account-card"
            to={`/systems/${clusterId}/accounts/${encodeURIComponent(account.name)}`}
          >
            <div className="nc-ring">
              <div className="nc-ring__stats">
                <div>{t("systems.conns", { count: account.name === "Default" ? connections : 0 })}</div>
                <div>{t("systems.msgs", { count: account.name === "Default" ? inMsgs : 0 })}</div>
                <div>{t("systems.bytes", { count: account.name === "Default" ? inBytes : 0 })}</div>
              </div>
            </div>
            <div className="nc-account-card__name">{account.name}</div>
          </Link>
        ))}
      </div>

      {accountQuery.data && (
        <p className="text-muted mt-16">
          {t("systems.activeSees", {
            streams: accountQuery.data.streams,
            consumers: accountQuery.data.consumers,
          })}
        </p>
      )}
    </div>
  );
}
