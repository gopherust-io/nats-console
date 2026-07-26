import { FormEvent, useState } from "react";
import { useTranslation } from "react-i18next";
import Alert from "../components/ui/Alert";
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

type FormState = {
  name: string;
  natsUrl: string;
  monitoringUrl: string;
  token: string;
  credsFilePath: string;
  isDefault: boolean;
};

const emptyForm = (): FormState => ({
  name: "",
  natsUrl: "",
  monitoringUrl: "",
  token: "",
  credsFilePath: "",
  isDefault: false,
});

export default function ClustersPage() {
  const { t } = useTranslation();
  const { canWrite, canDeleteClusters } = useAuth();
  const { clusters, reload, setClusterId, clusterId } = useCluster();
  const [availability, setAvailability] = useState<Record<string, AvailabilityState>>({});
  const [showForm, setShowForm] = useState(false);
  const [editItem, setEditItem] = useState<Cluster | null>(null);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  function resetForm() {
    setShowForm(false);
    setEditItem(null);
    setForm(emptyForm());
  }

  function openCreate() {
    setEditItem(null);
    setForm(emptyForm());
    setShowForm(true);
    setError("");
  }

  function openEdit(cluster: Cluster) {
    setEditItem(cluster);
    setForm({
      name: cluster.name,
      natsUrl: cluster.natsUrl,
      monitoringUrl: cluster.monitoringUrl || "",
      token: "",
      credsFilePath: "",
      isDefault: cluster.isDefault,
    });
    setShowForm(true);
    setError("");
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!form.name.trim() || !form.natsUrl.trim()) {
      setError(t("clusters.nameRequired"));
      return;
    }
    setSaving(true);
    setError("");
    try {
      if (editItem) {
        const body: Record<string, unknown> = {
          name: form.name.trim(),
          natsUrl: form.natsUrl.trim(),
          monitoringUrl: form.monitoringUrl.trim(),
          isDefault: form.isDefault,
        };
        if (form.token.trim()) body.token = form.token.trim();
        if (form.credsFilePath.trim()) body.credsFilePath = form.credsFilePath.trim();
        await api(`/api/v1/clusters/${editItem.id}`, {
          method: "PUT",
          body: JSON.stringify(body),
        });
      } else {
        await api("/api/v1/clusters", {
          method: "POST",
          body: JSON.stringify({
            name: form.name.trim(),
            natsUrl: form.natsUrl.trim(),
            monitoringUrl: form.monitoringUrl.trim(),
            token: form.token.trim(),
            credsFilePath: form.credsFilePath.trim(),
            isDefault: form.isDefault,
          }),
        });
      }
      resetForm();
      await reload();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : editItem
            ? t("clusters.updateFailed")
            : t("clusters.createFailed"),
      );
    } finally {
      setSaving(false);
    }
  }

  async function deleteCluster(cluster: Cluster) {
    if (!window.confirm(t("clusters.confirmDelete", { name: cluster.name }))) return;
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
  }

  async function testCluster(cluster: Cluster) {
    setAvailability((prev) => ({
      ...prev,
      [cluster.id]: {
        status: "checking",
        nonce: (prev[cluster.id]?.nonce ?? 0) + 1,
      },
    }));

    try {
      const result = await api<TestResponse>(`/api/v1/clusters/${cluster.id}/test`, { method: "POST" });
      setAvailability((prev) => ({
        ...prev,
        [cluster.id]: {
          status: "done",
          result,
          nonce: (prev[cluster.id]?.nonce ?? 0) + 1,
        },
      }));
    } catch (err) {
      setAvailability((prev) => ({
        ...prev,
        [cluster.id]: {
          status: "done",
          result: {
            ok: false,
            message: err instanceof Error ? err.message : t("clusters.checkFailed"),
          },
          nonce: (prev[cluster.id]?.nonce ?? 0) + 1,
        },
      }));
    }
  }

  return (
    <div>
      <div className="page-header">
        <h1>{t("clusters.title")}</h1>
        {canWrite && (
          <button className="btn btn--primary" type="button" onClick={openCreate}>
            {t("clusters.create")}
          </button>
        )}
      </div>
      <p className="text-muted mb-24">{t("clusters.help")}</p>
      {error && <Alert variant="error">{error}</Alert>}

      {showForm && canWrite && (
        <form className="card mb-24" onSubmit={onSubmit}>
          <h2>{editItem ? t("clusters.edit") : t("clusters.create")}</h2>
          <label>
            {t("common.name")}
            <input
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              required
            />
          </label>
          <label>
            {t("clusters.natsUrl")}
            <input
              className="mono"
              value={form.natsUrl}
              onChange={(e) => setForm((f) => ({ ...f, natsUrl: e.target.value }))}
              placeholder="nats://localhost:4222"
              required
            />
          </label>
          <label>
            {t("clusters.monitoringUrl")}
            <input
              className="mono"
              value={form.monitoringUrl}
              onChange={(e) => setForm((f) => ({ ...f, monitoringUrl: e.target.value }))}
              placeholder="http://localhost:8222"
            />
          </label>
          <label>
            {t("clusters.token")}
            <input
              type="password"
              value={form.token}
              onChange={(e) => setForm((f) => ({ ...f, token: e.target.value }))}
              placeholder={editItem?.hasToken ? "••••••••" : undefined}
              autoComplete="off"
            />
          </label>
          <label>
            {t("clusters.credsPath")}
            <input
              className="mono"
              value={form.credsFilePath}
              onChange={(e) => setForm((f) => ({ ...f, credsFilePath: e.target.value }))}
              placeholder={editItem?.hasCreds ? "(leave blank to keep)" : undefined}
            />
          </label>
          <label className="role-chip">
            <input
              type="checkbox"
              checked={form.isDefault}
              onChange={(e) => setForm((f) => ({ ...f, isDefault: e.target.checked }))}
            />
            {t("clusters.isDefault")}
          </label>
          <div className="actions">
            <button className="btn btn--primary" type="submit" disabled={saving}>
              {saving ? t("common.saving") : editItem ? t("common.save") : t("clusters.create")}
            </button>
            <button className="btn secondary" type="button" onClick={resetForm}>
              {t("common.cancel")}
            </button>
          </div>
        </form>
      )}

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>{t("common.name")}</th>
              <th>{t("clusters.natsUrl")}</th>
              <th>{t("clusters.monitoring")}</th>
              <th>{t("clusters.default")}</th>
              <th>{t("clusters.availability")}</th>
              {(canWrite || canDeleteClusters) && <th />}
            </tr>
          </thead>
          <tbody>
            {clusters.map((cluster) => {
              const state = availability[cluster.id];
              return (
                <tr key={cluster.id}>
                  <td>{cluster.name}</td>
                  <td className="mono">{cluster.natsUrl}</td>
                  <td className="mono">{cluster.monitoringUrl || "-"}</td>
                  <td>{cluster.isDefault ? t("common.yes") : t("common.no")}</td>
                  <td>
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
                  {(canWrite || canDeleteClusters) && (
                    <td>
                      <div className="actions">
                        {canWrite && (
                          <button className="btn secondary btn--small" type="button" onClick={() => openEdit(cluster)}>
                            {t("common.edit")}
                          </button>
                        )}
                        {canDeleteClusters && !cluster.isDefault && (
                          <button
                            className="btn btn--ghost btn--small"
                            type="button"
                            onClick={() => deleteCluster(cluster)}
                          >
                            {t("common.delete")}
                          </button>
                        )}
                      </div>
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
