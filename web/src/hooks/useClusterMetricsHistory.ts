import { useQuery } from "@tanstack/react-query";
import { api, clusterPath } from "../lib/api";
import { clusterQueryKey } from "../lib/query";
import { MetricsHistoryResponse, MetricsRangePreset, rangeToQuery } from "../lib/metricsHistory";

export function useClusterMetricsHistory(clusterId: string | null, range: MetricsRangePreset, metrics: string) {
  const params = rangeToQuery(range);
  const queryString = new URLSearchParams({
    from: params.from,
    to: params.to,
    step: params.step,
    metrics,
  }).toString();

  return useQuery({
    queryKey: clusterQueryKey(clusterId, `metrics-history:${range}:${metrics}`),
    queryFn: () => api<MetricsHistoryResponse>(`${clusterPath(clusterId!, "/metrics/history")}?${queryString}`),
    enabled: Boolean(clusterId),
    refetchInterval: 60_000,
  });
}
