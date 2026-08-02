import { useState } from "react";
import { useTranslation } from "react-i18next";
import Alert from "../components/ui/Alert";
import { useConfirmDialog } from "../hooks/useConfirmDialog";
import { api, Cluster } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";

type TestResponse = {
  ok: boolean;
  message: string;
  serverName?: string;
  jetstream?: boolean;
};

type AvailabilityState = {
  status: "checking" | "done";
  result?: TestResponse;
  /** Bumps on every click so the status message remounts and is visible again. */
  nonce: number;
};

/** Comma-separated NATS / monitoring URLs → one line each (avoids blowing out the table). */
function endpointLines(value: string): string[] {
  return value
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
}

function EndpointCell({ value, empty = "-" }: { value: string; empty?: string }) {
  const lines = endpointLines(value);
  if (lines.length === 0) return <td className="mono">{empty}</td>;
  return (
    <td className="mono">
      <div className="clusters-endpoints">
        {lines.map((line, i) => (
          <div key={`${i}-${line}`} className="clusters-endpoints__item">
            {line}
          </div>
        ))}
      </div>
    </td>
  );
}

export default function ClustersPage() {
  const { t } = useTranslation();
  const { askConfirm, confirmDialog } = useConfirmDialog();
  const { canDeleteClusters } = useAuth();
  const { clusters, reload, setClusterId, clusterId } = useCluster();
  const [availability, setAvailability] = useState<Record<string, AvailabilityState>>({});
  const [error, setError] = useState("");
  const showDelete = canDeleteClusters && clusters.some((c) => !c.isDefault);

  function deleteCluster(cluster: Cluster) {
    askConfirm({
      title: t("clusters.confirmDeleteTitle"),
      description: t("clusters.confirmDelete", { name: cluster.name }),
      action: async () => {
        setError("");
        try {
          await api(`/api/v1/clusters/${cluster.id}`, { method: "DELETE" });
          if (clusterId === cluster.id) {
            const next = clusters.find((c) => c.id !== cluster.id);
            if (next) setClusterId(next.id);
          }
          await reload();
        } catch (err) {
          setError(err instanceof Error ? err.message : t("clusters.deleteFailed"));
        }
      },
    });
  }

  async function testCluster(cluster: Cluster) {
    const nonce = (availability[cluster.id]?.nonce ?? 0) + 1;
    setAvailability((prev) => ({
      ...prev,
      [cluster.id]: { status: "checking", nonce },
    }));

    try {
      const result = (await api<TestResponse>(`/api/v1/clusters/${cluster.id}/test`, { method: "POST" })).data;
      setAvailability((prev) => {
        if (prev[cluster.id]?.nonce !== nonce) return prev;
        return { ...prev, [cluster.id]: { status: "done", result, nonce } };
      });
    } catch (err) {
      setAvailability((prev) => {
        if (prev[cluster.id]?.nonce !== nonce) return prev;
        return {
          ...prev,
          [cluster.id]: {
            status: "done",
            result: {
              ok: false,
              message: err instanceof Error ? err.message : t("clusters.checkFailed"),
            },
            nonce,
          },
        };
      });
    }
  }

  return (
    <div className="clusters-page">
      {confirmDialog}
      <div className="page-header">
        <h1>{t("clusters.title")}</h1>
      </div>
      <p className="text-muted mb-24">{t("clusters.help")}</p>
      {error && <Alert variant="error">{error}</Alert>}

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>{t("common.name")}</th>
              <th>{t("clusters.natsUrl")}</th>
              <th>{t("clusters.monitoring")}</th>
              <th>{t("clusters.default")}</th>
              <th>{t("clusters.availability")}</th>
              {showDelete && <th />}
            </tr>
          </thead>
          <tbody>
            {clusters.map((cluster) => {
              const state = availability[cluster.id];
              return (
                <tr key={cluster.id}>
                  <td>{cluster.name}</td>
                  <EndpointCell value={cluster.natsUrl} />
                  <EndpointCell value={cluster.monitoringUrl || ""} />
                  <td>{cluster.isDefault ? t("common.yes") : t("common.no")}</td>
                  <td className="clusters-availability">
                    <button
                      className="btn secondary"
                      onClick={() => testCluster(cluster)}
                      disabled={state?.status === "checking"}
                    >
                      {state?.status === "checking" ? t("clusters.checking") : t("clusters.checkAvailability")}
                    </button>
                    {state?.status === "checking" && (
                      <span
                        key={`checking-${state.nonce}`}
                        className="cluster-availability-msg cluster-availability-msg--checking nc-animate-in nc-fade-in nc-slide-from-bottom-2"
                      >
                        <svg className="cluster-availability-heartbeat" viewBox="0 0 16 16" aria-hidden>
                          <path
                            fill="currentColor"
                            d="M8 14.25S1.5 10.1 1.5 5.85C1.5 3.6 3.15 2 5.15 2c1.15 0 2.15.55 2.85 1.4C8.7 2.55 9.7 2 10.85 2 12.85 2 14.5 3.6 14.5 5.85 14.5 10.1 8 14.25 8 14.25z"
                          />
                        </svg>
                        {t("clusters.checking")}
                      </span>
                    )}
                    {state?.status === "done" && state.result && (
                      <span
                        key={`result-${state.nonce}`}
                        className={`cluster-availability-msg nc-animate-in nc-fade-in nc-slide-from-bottom-2 ${
                          state.result.ok
                            ? "text-success cluster-availability-msg--ok cluster-availability-msg--beat"
                            : "text-error cluster-availability-msg--err"
                        }`}
                      >
                        <svg
                          className={`cluster-availability-heartbeat${state.result.ok ? "" : " cluster-availability-heartbeat--still"}`}
                          viewBox="0 0 16 16"
                          aria-hidden
                        >
                          <path
                            fill="currentColor"
                            d="M8 14.25S1.5 10.1 1.5 5.85C1.5 3.6 3.15 2 5.15 2c1.15 0 2.15.55 2.85 1.4C8.7 2.55 9.7 2 10.85 2 12.85 2 14.5 3.6 14.5 5.85 14.5 10.1 8 14.25 8 14.25z"
                          />
                        </svg>
                        {state.result.ok ? t("clusters.available") : state.result.message}
                      </span>
                    )}
                  </td>
                  {showDelete && (
                    <td>
                      {!cluster.isDefault && (
                        <button
                          className="btn btn--ghost btn--small"
                          type="button"
                          onClick={() => deleteCluster(cluster)}
                        >
                          {t("common.delete")}
                        </button>
                      )}
                    </td>
                  )}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
