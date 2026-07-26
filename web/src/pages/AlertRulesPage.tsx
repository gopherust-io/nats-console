import { FormEvent, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "../components/ui/Alert";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import {
  createAlertRule,
  deleteAlertRule,
  fetchAlertRuleMetrics,
  fetchAlertRules,
  updateAlertRule,
  type AlertComparator,
  type AlertRule,
  type AlertSeverity,
} from "../lib/alerts";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";

type FormState = {
  name: string;
  message: string;
  severity: AlertSeverity;
  metric: string;
  comparator: AlertComparator;
  threshold: string;
  enabled: boolean;
  scopeAll: boolean;
};

const emptyForm = (): FormState => ({
  name: "",
  message: "",
  severity: "warning",
  metric: "server.cpu_percent",
  comparator: "gte",
  threshold: "90",
  enabled: true,
  scopeAll: true,
});

function formFromRule(rule: AlertRule): FormState {
  return {
    name: rule.name,
    message: rule.message,
    severity: rule.severity,
    metric: rule.metric,
    comparator: rule.comparator,
    threshold: String(rule.threshold),
    enabled: rule.enabled,
    scopeAll: !rule.clusterId,
  };
}

export default function AlertRulesPage() {
  const { t } = useTranslation();
  const { canManageAlertRules } = useAuth();
  const { clusterId, clusters } = useCluster();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<FormState>(emptyForm);
  const [editing, setEditing] = useState<AlertRule | null>(null);
  const [error, setError] = useState("");

  const rulesQuery = useQuery({
    queryKey: ["alert-rules"],
    queryFn: () => fetchAlertRules(),
    enabled: canManageAlertRules,
  });

  const metricsQuery = useQuery({
    queryKey: ["alert-rules", "metrics"],
    queryFn: fetchAlertRuleMetrics,
    enabled: canManageAlertRules,
  });

  const createMutation = useMutation({
    mutationFn: createAlertRule,
    onSuccess: async () => {
      setForm(emptyForm());
      setEditing(null);
      setError("");
      await queryClient.invalidateQueries({ queryKey: ["alert-rules"] });
    },
    onError: (err: Error) => setError(err.message),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, body }: { id: string; body: Parameters<typeof updateAlertRule>[1] }) =>
      updateAlertRule(id, body),
    onSuccess: async () => {
      setForm(emptyForm());
      setEditing(null);
      setError("");
      await queryClient.invalidateQueries({ queryKey: ["alert-rules"] });
    },
    onError: (err: Error) => setError(err.message),
  });

  const deleteMutation = useMutation({
    mutationFn: deleteAlertRule,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["alert-rules"] });
    },
    onError: (err: Error) => setError(err.message),
  });

  if (!canManageAlertRules) {
    return (
      <div className="page">
        <EmptyState title={t("alerts.noPermissionTitle")} description={t("alerts.noPermissionDescription")} />
      </div>
    );
  }

  const rules = rulesQuery.data?.rules ?? [];
  const metrics = metricsQuery.data?.metrics ?? [];
  const comparators = metricsQuery.data?.comparators ?? ["gt", "gte", "lt", "lte"];
  const severities = metricsQuery.data?.severities ?? ["info", "warning", "critical"];

  function startEdit(rule: AlertRule) {
    setEditing(rule);
    setForm(formFromRule(rule));
    setError("");
  }

  function cancelEdit() {
    setEditing(null);
    setForm(emptyForm());
  }

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    const threshold = Number(form.threshold);
    if (!form.name.trim() || Number.isNaN(threshold)) {
      setError(t("alerts.invalidForm"));
      return;
    }
    if (editing) {
      updateMutation.mutate({
        id: editing.id,
        body: {
          name: form.name.trim(),
          message: form.message.trim() || form.name.trim(),
          severity: form.severity,
          metric: form.metric,
          comparator: form.comparator,
          threshold,
          enabled: form.enabled,
          clearCluster: form.scopeAll,
          clusterId: form.scopeAll ? undefined : clusterId || undefined,
        },
      });
      return;
    }
    createMutation.mutate({
      name: form.name.trim(),
      message: form.message.trim() || form.name.trim(),
      severity: form.severity,
      metric: form.metric,
      comparator: form.comparator,
      threshold,
      enabled: form.enabled,
      clusterId: form.scopeAll ? undefined : clusterId || undefined,
    });
  }

  const busy = createMutation.isPending || updateMutation.isPending;

  return (
    <div className="page">
      <PageHeader
        eyebrow={t("alerts.eyebrow")}
        title={t("alerts.rulesTitle")}
        subtitle={t("alerts.rulesSubtitle")}
        actions={
          <Link className="btn btn--secondary" to="/admin/alerts">
            {t("alerts.viewFeed")}
          </Link>
        }
      />

      <Alert variant="error">{error || (rulesQuery.error instanceof Error ? rulesQuery.error.message : "")}</Alert>

      <section className="panel" style={{ marginBottom: 20 }}>
        <h2 className="panel__title">{editing ? t("alerts.editRule") : t("alerts.createRule")}</h2>
        <form className="form-grid" onSubmit={onSubmit}>
          <label>
            {t("alerts.ruleName")}
            <input value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} required />
          </label>
          <label>
            {t("alerts.message")}
            <input value={form.message} onChange={(e) => setForm((f) => ({ ...f, message: e.target.value }))} />
          </label>
          <label>
            {t("alerts.metric")}
            <select value={form.metric} onChange={(e) => setForm((f) => ({ ...f, metric: e.target.value }))}>
              {(metrics.length ? metrics : [form.metric]).map((m) => (
                <option key={m} value={m}>
                  {m}
                </option>
              ))}
            </select>
          </label>
          <label>
            {t("alerts.comparator")}
            <select
              value={form.comparator}
              onChange={(e) => setForm((f) => ({ ...f, comparator: e.target.value as AlertComparator }))}
            >
              {comparators.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
          </label>
          <label>
            {t("alerts.threshold")}
            <input
              type="number"
              step="any"
              value={form.threshold}
              onChange={(e) => setForm((f) => ({ ...f, threshold: e.target.value }))}
              required
            />
          </label>
          <label>
            {t("alerts.severity")}
            <select
              value={form.severity}
              onChange={(e) => setForm((f) => ({ ...f, severity: e.target.value as AlertSeverity }))}
            >
              {severities.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </label>
          <label className="role-chip">
            <input
              type="checkbox"
              checked={form.scopeAll}
              onChange={(e) => setForm((f) => ({ ...f, scopeAll: e.target.checked }))}
            />
            {t("alerts.allClusters")}
          </label>
          {!form.scopeAll && (
            <p className="text-muted">
              {t("alerts.scopedTo", { name: clusters.find((c) => c.id === clusterId)?.name ?? clusterId ?? "—" })}
            </p>
          )}
          <label className="role-chip">
            <input
              type="checkbox"
              checked={form.enabled}
              onChange={(e) => setForm((f) => ({ ...f, enabled: e.target.checked }))}
            />
            {t("alerts.enabled")}
          </label>
          <div className="actions">
            <button className="btn" type="submit" disabled={busy}>
              {busy
                ? t("common.loading")
                : editing
                  ? t("common.save")
                  : t("alerts.createRule")}
            </button>
            {editing && (
              <button className="btn secondary" type="button" onClick={cancelEdit}>
                {t("common.cancel")}
              </button>
            )}
          </div>
        </form>
      </section>

      {rulesQuery.isLoading && <div className="skeleton skeleton--table" />}

      {!rulesQuery.isLoading && rules.length === 0 && (
        <EmptyState title={t("alerts.noRulesTitle")} description={t("alerts.noRulesDescription")} />
      )}

      {rules.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("alerts.ruleName")}</th>
                <th>{t("alerts.metric")}</th>
                <th>{t("alerts.threshold")}</th>
                <th>{t("alerts.severity")}</th>
                <th>{t("alerts.scope")}</th>
                <th>{t("alerts.enabled")}</th>
                <th>{t("alerts.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {rules.map((rule) => (
                <tr key={rule.id}>
                  <td>
                    <div>{rule.name}</div>
                    <div className="text-muted" style={{ fontSize: "0.8rem" }}>
                      {rule.message}
                    </div>
                  </td>
                  <td>
                    <code>{rule.metric}</code>
                  </td>
                  <td>
                    {rule.comparator} {rule.threshold}
                  </td>
                  <td>
                    <span className={`nc-severity nc-severity--${rule.severity}`}>{rule.severity}</span>
                  </td>
                  <td>{rule.clusterId || t("alerts.allClusters")}</td>
                  <td>
                    <input
                      type="checkbox"
                      checked={rule.enabled}
                      disabled={updateMutation.isPending}
                      onChange={(e) => updateMutation.mutate({ id: rule.id, body: { enabled: e.target.checked } })}
                    />
                  </td>
                  <td>
                    <div className="actions">
                      <button
                        type="button"
                        className="btn btn--secondary btn--small"
                        onClick={() => startEdit(rule)}
                      >
                        {t("common.edit")}
                      </button>
                      <button
                        type="button"
                        className="btn btn--secondary btn--small"
                        disabled={deleteMutation.isPending}
                        onClick={() => {
                          if (window.confirm(t("alerts.confirmDelete", { name: rule.name }))) {
                            deleteMutation.mutate(rule.id);
                          }
                        }}
                      >
                        {t("common.delete")}
                      </button>
                    </div>
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
