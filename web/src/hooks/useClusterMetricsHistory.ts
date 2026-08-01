import { useQuery } from "@tanstack/react-query";
import { api, clusterPath } from "../lib/api";
import { METRICS_HISTORY_POLL_MS } from "../lib/constants";
import { clusterQueryKey } from "../lib/query";
import { MetricsHistoryResponse, MetricsRangePreset, rangeToQuery } from "../lib/metricsHistory";

export function useClusterMetricsHistory(
  clusterId: string | null,
  range: MetricsRangePreset,
  metrics: string,
  options?: { enabled?: boolean },
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
    refetchInterval: METRICS_HISTORY_POLL_MS,
    refetchOnWindowFocus: false,
    refetchIntervalInBackground: false,
  });
}
