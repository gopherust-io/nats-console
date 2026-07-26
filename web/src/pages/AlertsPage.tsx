import { FormEvent, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "../components/ui/Alert";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import { acknowledgeAlert, fetchAlerts, type AlertStatus } from "../lib/alerts";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";

function formatWhen(iso: string) {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

export default function AlertsPage() {
  const { t } = useTranslation();
  const { canManageAlertRules } = useAuth();
  const { clusterId } = useCluster();
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<AlertStatus>("open");
  const [filterCluster, setFilterCluster] = useState("");

  const alertsQuery = useQuery({
    queryKey: ["alerts", status, filterCluster],
    queryFn: () => fetchAlerts({ status, clusterId: filterCluster || undefined, limit: 100 }),
  });

  const ackMutation = useMutation({
    mutationFn: acknowledgeAlert,
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["alerts"] }),
        queryClient.invalidateQueries({ queryKey: ["alerts", "open-summary"] }),
      ]);
    },
  });

  const alerts = alertsQuery.data?.alerts ?? [];
  const total = alertsQuery.data?.total ?? 0;
  const error =
    (alertsQuery.error instanceof Error ? alertsQuery.error.message : "") ||
    (ackMutation.error instanceof Error ? ackMutation.error.message : "");

  function onFilter(event: FormEvent) {
    event.preventDefault();
  }

  return (
    <div className="page">
      <PageHeader
        eyebrow={t("alerts.eyebrow")}
        title={t("alerts.title")}
        subtitle={t("alerts.subtitle")}
        badge={<span className="badge">{t("alerts.count", { count: total })}</span>}
        actions={
          <>
            {canManageAlertRules && (
              <Link className="btn btn--secondary" to="/admin/alert-rules">
                {t("alerts.manageRules")}
              </Link>
            )}
            <button className="btn btn--secondary" type="button" onClick={() => alertsQuery.refetch()} disabled={alertsQuery.isFetching}>
              {t("common.refresh")}
            </button>
          </>
        }
      />

      <Alert variant="error">{error}</Alert>

      <div className="nc-tabs" style={{ marginBottom: 16 }}>
        <button type="button" className={`nc-tab${status === "open" ? " active" : ""}`} onClick={() => setStatus("open")}>
          {t("alerts.open")}
        </button>
        <button type="button" className={`nc-tab${status === "closed" ? " active" : ""}`} onClick={() => setStatus("closed")}>
          {t("alerts.closed")}
        </button>
      </div>

      <form className="audit-toolbar panel" onSubmit={onFilter}>
        <label className="audit-toolbar__field">
          <span className="audit-toolbar__label">{t("alerts.clusterFilter")}</span>
          <input
            value={filterCluster}
            onChange={(e) => setFilterCluster(e.target.value.trim())}
            placeholder={clusterId ?? t("alerts.clusterPlaceholder")}
          />
        </label>
        <button
          className="btn btn--secondary"
          type="button"
          onClick={() => {
            if (clusterId) setFilterCluster(clusterId);
          }}
        >
          {t("alerts.useActiveCluster")}
        </button>
      </form>

      {alertsQuery.isLoading && <div className="skeleton skeleton--table" />}

      {!alertsQuery.isLoading && !alertsQuery.isError && alerts.length === 0 && (
        <EmptyState title={t("alerts.emptyTitle")} description={t("alerts.emptyDescription")} />
      )}

      {!alertsQuery.isLoading && alerts.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("alerts.severity")}</th>
                <th>{t("alerts.message")}</th>
                <th>{t("alerts.metric")}</th>
                <th>{t("alerts.value")}</th>
                <th>{t("alerts.lastSeen")}</th>
                <th>{t("alerts.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {alerts.map((alert) => (
                <tr key={alert.id}>
                  <td>
                    <span className={`nc-severity nc-severity--${alert.severity}`}>{alert.severity}</span>
                  </td>
                  <td>
                    <div>{alert.message || alert.ruleName}</div>
                    <div className="text-muted" style={{ fontSize: "0.8rem" }}>
                      {alert.ruleName}
                      {alert.acknowledgedAt ? ` · ${t("alerts.acknowledged")}` : ""}
                    </div>
                  </td>
                  <td>
                    <code>{alert.metric}</code>
                  </td>
                  <td>
                    {alert.firingValue} / {alert.threshold}
                  </td>
                  <td>{formatWhen(alert.lastSeenAt)}</td>
                  <td>
                    {alert.status === "open" && !alert.acknowledgedAt && (
                      <button
                        type="button"
                        className="btn btn--secondary btn--small"
                        disabled={ackMutation.isPending}
                        onClick={() => ackMutation.mutate(alert.id)}
                      >
                        {t("alerts.acknowledge")}
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
