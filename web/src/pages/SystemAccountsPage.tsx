import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { formatAccountBytes, formatAccountCount } from "../components/AccountMetricStats";
import { useAccountOverviewEvents } from "../hooks/useAccountOverviewEvents";
import { useAccount } from "../lib/account";
import { api, clusterPath, type AccountInfo } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

type VarzLite = {
  connections?: number;
  in_msgs?: number;
  in_bytes?: number;
};

function AccountCardIcon() {
  return (
    <span className="nc-system-card__icon" aria-hidden="true">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        <circle cx="12" cy="8" r="3.25" />
        <path d="M5.5 19.5c1.6-3.2 4-4.75 6.5-4.75s4.9 1.55 6.5 4.75" strokeLinecap="round" />
      </svg>
    </span>
  );
}

function jetStreamValue(
  known: boolean,
  value: number | undefined,
  empty: string,
  format: (n: number) => string,
) {
  if (!known || value == null) return empty;
  return format(value);
}

export default function SystemAccountsPage() {
  const { t } = useTranslation();
  const { clusterId } = useParams();
  const { cluster } = useCluster();
  const { accounts } = useAccount();

  useAccountOverviewEvents(clusterId ?? null);

  const varzQuery = useQuery({
    queryKey: clusterQueryKey(clusterId ?? null, "varz-lite"),
    queryFn: async () => (await api<VarzLite>(clusterPath(clusterId!, "/monitoring/varz"))).data,
    enabled: Boolean(clusterId),
    staleTime: 5_000,
    refetchOnWindowFocus: false,
    refetchInterval: false,
  });

  const accountQuery = useQuery({
    queryKey: clusterQueryKey(clusterId ?? null, "account"),
    queryFn: async () => (await api<AccountInfo>(clusterPath(clusterId!, "/account"))).data,
    enabled: Boolean(clusterId),
    staleTime: 5_000,
    refetchOnWindowFocus: false,
    refetchInterval: false,
  });

  const connections = Number(varzQuery.data?.connections ?? 0);
  const inMsgs = Number(varzQuery.data?.in_msgs ?? 0);
  const inBytes = Number(varzQuery.data?.in_bytes ?? 0);
  const accountInfo = accountQuery.data;
  const empty = t("common.emDash");
  const live = Boolean(varzQuery.data || accountInfo);

  return (
    <div className="nc-systems-page">
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("systems.accountsTitle")}</h1>
          <p className="nc-page-sub">
            {t("systems.accountsSubtitle", { name: cluster?.name ?? t("systems.thisSystem") })}
          </p>
        </div>
      </div>

      <div className="nc-card-grid nc-systems-page__grid">
        {accounts.map((account) => {
          const isDefault = account.name === "Default";
          const jetStreamKnown = isDefault && Boolean(accountInfo);
          const showLive = isDefault && live;
          return (
            <Link
              key={account.name}
              className="nc-system-card"
              to={`/systems/${clusterId}/accounts/${encodeURIComponent(account.name)}`}
            >
              <div className="nc-system-card__top">
                <AccountCardIcon />
                {showLive ? (
                  <span
                    className="nc-conn-live"
                    title={t("account.clusterStatus.liveHint")}
                    aria-label={t("account.clusterStatus.available")}
                  >
                    <span className="nc-conn-live__dot" aria-hidden="true" />
                    {t("account.clusterStatus.live")}
                  </span>
                ) : (
                  <span className="nc-conn-status" aria-label={t("account.unknownStatus")}>
                    <span className="nc-conn-status__dot" aria-hidden="true" />
                    {t("account.unknownStatus")}
                  </span>
                )}
              </div>
              <div className="nc-system-card__body">
                <div className="nc-system-card__name">{account.name}</div>
                <div className="nc-system-card__meta">
                  <span>{isDefault ? t("systems.defaultAccount") : t("systems.natsAccount")}</span>
                  {isDefault ? (
                    <>
                      <span aria-hidden="true">·</span>
                      <span>{t("systems.conns", { count: formatAccountCount(connections) })}</span>
                      <span aria-hidden="true">·</span>
                      <span>{t("systems.msgs", { count: formatAccountCount(inMsgs) })}</span>
                      <span aria-hidden="true">·</span>
                      <span>{formatAccountBytes(inBytes)}</span>
                    </>
                  ) : null}
                </div>
              </div>
              <div className="nc-system-card__stats" aria-label={t("account.jetStreamSection")}>
                <div className="nc-system-card__stat">
                  <span className="nc-system-card__stat-label">{t("account.streams")}</span>
                  <span className="nc-system-card__stat-value mono">
                    {jetStreamValue(jetStreamKnown, accountInfo?.streams, empty, formatAccountCount)}
                  </span>
                </div>
                <div className="nc-system-card__stat">
                  <span className="nc-system-card__stat-label">{t("account.consumers")}</span>
                  <span className="nc-system-card__stat-value mono">
                    {jetStreamValue(jetStreamKnown, accountInfo?.consumers, empty, formatAccountCount)}
                  </span>
                </div>
                <div className="nc-system-card__stat">
                  <span className="nc-system-card__stat-label">{t("account.storage")}</span>
                  <span className="nc-system-card__stat-value mono">
                    {jetStreamValue(jetStreamKnown, accountInfo?.storage, empty, formatAccountBytes)}
                  </span>
                </div>
              </div>
            </Link>
          );
        })}
      </div>
    </div>
  );
}
