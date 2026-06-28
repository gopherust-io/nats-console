import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, Cluster, ClusterListResponse, getSelectedClusterId, setSelectedClusterId } from "../lib/api";
import { clusterQueryKey } from "./query";

type ClusterContextValue = {
  clusters: Cluster[];
  clusterId: string | null;
  cluster: Cluster | null;
  setClusterId: (id: string) => void;
  reload: () => Promise<void>;
  loading: boolean;
  error: string;
};

const ClusterContext = createContext<ClusterContextValue | null>(null);

export function ClusterProvider({ children }: { children: ReactNode }) {
  const [clusterId, setClusterIdState] = useState<string | null>(getSelectedClusterId());

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: clusterQueryKey(null, "list"),
    queryFn: () => api<ClusterListResponse>("/api/v1/clusters"),
    staleTime: 60_000,
  });

  const clusters = useMemo(() => data?.clusters ?? [], [data?.clusters]);

  useEffect(() => {
    if (clusters.length === 0) return;
    const stored = getSelectedClusterId();
    const exists = clusters.find((c) => c.id === stored);
    if (exists) {
      setClusterIdState(stored);
      return;
    }
    const fallback = clusters.find((c) => c.isDefault) ?? clusters[0];
    if (fallback) {
      setSelectedClusterId(fallback.id);
      setClusterIdState(fallback.id);
    }
  }, [clusters]);

  const setClusterId = useCallback((id: string) => {
    setSelectedClusterId(id);
    setClusterIdState(id);
  }, []);

  const reload = useCallback(async () => {
    await refetch();
  }, [refetch]);

  const cluster = clusters.find((c) => c.id === clusterId) ?? null;

  const value = useMemo(
    () => ({
      clusters,
      clusterId,
      cluster,
      setClusterId,
      reload,
      loading: isLoading,
      error: error instanceof Error ? error.message : "",
    }),
    [clusters, clusterId, cluster, setClusterId, reload, isLoading, error],
  );

  return <ClusterContext.Provider value={value}>{children}</ClusterContext.Provider>;
}

export function useCluster() {
  const ctx = useContext(ClusterContext);
  if (!ctx) {
    throw new Error("useCluster must be used within ClusterProvider");
  }
  return ctx;
}
