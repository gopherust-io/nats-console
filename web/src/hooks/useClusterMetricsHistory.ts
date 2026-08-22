import { useQuery } from "@tanstack/react-query";
import { api, clusterPath } from "../lib/api";
import { METRICS_HISTORY_POLL_MS } from "../lib/constants";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";
import { MetricsHistoryResponse, MetricsRangePreset, rangeToQuery } from "../lib/metricsHistory";

function resolveInterval(value: number | false | undefined) {
  if (value === false) return false;
  if (typeof value === "number") return visibilityAwareInterval(value);
  return visibilityAwareInterval(METRICS_HISTORY_POLL_MS);
}

export function useClusterMetricsHistory(
  clusterId: string | null,
  range: MetricsRangePreset,
  metrics: string,
  options?: { enabled?: boolean; refetchInterval?: number | false },
) {
  const params = rangeToQuery(range);
  const queryString = new URLSearchParams({
    from: params.from,
    to: params.to,
    step: params.step,
    metrics,
  }).toString();

  return useQuery({
    queryKey: clusterQueryKey(clusterId, `metrics-history:${range}:${metrics}`),
    queryFn: async () =>
      (await api<MetricsHistoryResponse>(`${clusterPath(clusterId!, "/metrics/history")}?${queryString}`)).data,
    enabled: Boolean(clusterId) && metrics.length > 0 && (options?.enabled ?? true),
    refetchInterval: resolveInterval(options?.refetchInterval),
    refetchOnWindowFocus: false,
    refetchIntervalInBackground: false,
  });
}
