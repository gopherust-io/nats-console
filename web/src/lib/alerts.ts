import { api } from "./api";

export type AlertSeverity = "info" | "warning" | "critical";
export type AlertStatus = "open" | "closed";
export type AlertComparator = "gt" | "gte" | "lt" | "lte";

export type Alert = {
  id: string;
  ruleId: string;
  clusterId: string;
  accountName?: string;
  status: AlertStatus;
  severity: AlertSeverity;
  metric: string;
  message: string;
  firingValue: number;
  threshold: number;
  firstSeenAt: string;
  lastSeenAt: string;
  closedAt?: string;
  acknowledgedAt?: string;
  acknowledgedBy?: string;
  ruleName?: string;
};

export type AlertRule = {
  id: string;
  clusterId?: string;
  accountName?: string;
  name: string;
  message: string;
  severity: AlertSeverity;
  metric: string;
  comparator: AlertComparator;
  threshold: number;
  enabled: boolean;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type AlertOpenSummary = {
  count: number;
  alerts: Alert[];
};

export type AlertRuleMetrics = {
  metrics: string[];
  comparators: AlertComparator[];
  severities: AlertSeverity[];
};

export async function fetchAlertOpenSummary() {
  return (await api<AlertOpenSummary>("/api/v1/alerts/open-summary")).data;
}

export async function fetchAlerts(params: { status?: AlertStatus; clusterId?: string; limit?: number }) {
  const q = new URLSearchParams();
  if (params.status) q.set("status", params.status);
  if (params.clusterId) q.set("clusterId", params.clusterId);
  if (params.limit) q.set("limit", String(params.limit));
  const suffix = q.toString() ? `?${q}` : "";
  const r = await api<Alert[]>(`/api/v1/alerts${suffix}`);
  return { alerts: r.data ?? [], total: r.meta?.total ?? 0 };
}

export async function acknowledgeAlert(id: string) {
  return (await api<Alert>(`/api/v1/alerts/${encodeURIComponent(id)}/acknowledge`, { method: "POST" })).data;
}

export async function fetchAlertRules(clusterId?: string) {
  const q = clusterId ? `?clusterId=${encodeURIComponent(clusterId)}` : "";
  const r = await api<AlertRule[]>(`/api/v1/alert-rules${q}`);
  return { rules: r.data ?? [], total: r.meta?.total ?? 0 };
}

export async function fetchAlertRuleMetrics() {
  return (await api<AlertRuleMetrics>("/api/v1/alert-rules/metrics")).data;
}

export async function createAlertRule(body: {
  name: string;
  message?: string;
  severity: AlertSeverity;
  metric: string;
  comparator: AlertComparator;
  threshold: number;
  enabled?: boolean;
  clusterId?: string;
}) {
  return (await api<AlertRule>("/api/v1/alert-rules", { method: "POST", body: JSON.stringify(body) })).data;
}

export async function updateAlertRule(
  id: string,
  body: Partial<{
    name: string;
    message: string;
    severity: AlertSeverity;
    metric: string;
    comparator: AlertComparator;
    threshold: number;
    enabled: boolean;
    clusterId: string;
    clearCluster: boolean;
  }>,
) {
  return (
    await api<AlertRule>(`/api/v1/alert-rules/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    })
  ).data;
}

export async function deleteAlertRule(id: string) {
  await api<void>(`/api/v1/alert-rules/${encodeURIComponent(id)}`, { method: "DELETE" });
}
