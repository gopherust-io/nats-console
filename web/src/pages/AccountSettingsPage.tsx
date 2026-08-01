import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import Alert from "../components/ui/Alert";
import { AccountInfo, api, clusterPath } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`;
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MiB`;
  return `${(value / (1024 * 1024 * 1024)).toFixed(2)} GiB`;
}

export default function AccountSettingsPage() {
  const { t } = useTranslation();
  const { accountName, clusterId: routeCluster } = useParams();
  const { clusterId } = useCluster();
  const id = routeCluster ?? clusterId;
  const account = accountName ?? "Default";
  const [name, setName] = useState(account);

  const accountQuery = useQuery({
    queryKey: clusterQueryKey(id, "account"),
    queryFn: async () => (await api<AccountInfo>(clusterPath(id!, "/account"))).data,
    enabled: Boolean(id),
  });

  useEffect(() => {
    setName(account);
  }, [account]);

  const info = accountQuery.data;

  return (
    <div>
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("account.settingsTitle")}</h1>
          <p className="nc-page-sub">{t("account.settingsSubtitle")}</p>
        </div>
      </div>

      {accountQuery.error instanceof Error && <Alert variant="error">{accountQuery.error.message}</Alert>}

      <div className="nc-settings-section">
        <h4>{t("account.general")}</h4>
        <p>{t("account.generalHelp")}</p>
        <div className="nc-form-row">
          <label htmlFor="account-name">{t("account.accountName")}</label>
          <input id="account-name" value={name} readOnly disabled />
        </div>
      </div>

      <div className="nc-settings-section">
        <h4>{t("account.limits")}</h4>
        <p>{t("account.limitsHelp")}</p>
        <div className="nc-meta-row">
          <span>{t("account.streams")}</span>
          <span>
            {info?.streams ?? 0} / {info?.limits.maxStreams ?? "∞"}
          </span>
        </div>
        <div className="nc-meta-row">
          <span>{t("account.consumers")}</span>
          <span>
            {info?.consumers ?? 0} / {info?.limits.maxConsumers ?? "∞"}
          </span>
        </div>
        <div className="nc-meta-row">
          <span>{t("account.diskStorage")}</span>
          <span>
            {formatBytes(info?.storage ?? 0)} / {formatBytes(info?.limits.maxStorage ?? 0)}
          </span>
        </div>
        <div className="nc-meta-row">
          <span>{t("systems.memoryStorage")}</span>
          <span>
            {formatBytes(info?.memory ?? 0)} / {formatBytes(info?.limits.maxMemory ?? 0)}
          </span>
        </div>
      </div>

      <div className="nc-settings-section">
        <h4>{t("account.jetStreamSection")}</h4>
        <div className="nc-meta-row">
          <span>{t("account.jetStreamSection")}</span>
          <span>{info ? t("common.enabled") : t("account.unknownStatus")}</span>
        </div>
      </div>
    </div>
  );
}
