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

export function fetchAlertOpenSummary() {
  return api<AlertOpenSummary>("/api/v1/alerts/open-summary");
}

export function fetchAlerts(params: { status?: AlertStatus; clusterId?: string; limit?: number }) {
  const q = new URLSearchParams();
  if (params.status) q.set("status", params.status);
  if (params.clusterId) q.set("clusterId", params.clusterId);
  if (params.limit) q.set("limit", String(params.limit));
  const suffix = q.toString() ? `?${q}` : "";
  return api<{ alerts: Alert[]; total: number }>(`/api/v1/alerts${suffix}`);
}

export function acknowledgeAlert(id: string) {
  return api<Alert>(`/api/v1/alerts/${encodeURIComponent(id)}/acknowledge`, { method: "POST" });
}

export function fetchAlertRules(clusterId?: string) {
  const q = clusterId ? `?clusterId=${encodeURIComponent(clusterId)}` : "";
  return api<{ rules: AlertRule[]; total: number }>(`/api/v1/alert-rules${q}`);
}

export function fetchAlertRuleMetrics() {
  return api<AlertRuleMetrics>("/api/v1/alert-rules/metrics");
}

export function createAlertRule(body: {
  name: string;
  message?: string;
  severity: AlertSeverity;
  metric: string;
  comparator: AlertComparator;
  threshold: number;
  enabled?: boolean;
  clusterId?: string;
}) {
  return api<AlertRule>("/api/v1/alert-rules", { method: "POST", body: JSON.stringify(body) });
}

export function updateAlertRule(
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
  return api<AlertRule>(`/api/v1/alert-rules/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}

export function deleteAlertRule(id: string) {
  return api<void>(`/api/v1/alert-rules/${encodeURIComponent(id)}`, { method: "DELETE" });
}
