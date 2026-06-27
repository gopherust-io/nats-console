import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { api, Cluster, ClusterListResponse, getSelectedClusterId, setSelectedClusterId } from "../lib/api";

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
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [clusterId, setClusterIdState] = useState<string | null>(getSelectedClusterId());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  async function reload() {
    try {
      const data = await api<ClusterListResponse>("/api/v1/clusters");
      setClusters(data.clusters);
      setError("");

      const stored = getSelectedClusterId();
      const exists = data.clusters.find((c) => c.id === stored);
      if (exists) {
        setClusterIdState(stored);
      } else {
        const fallback = data.clusters.find((c) => c.is_default) ?? data.clusters[0];
        if (fallback) {
          setSelectedClusterId(fallback.id);
          setClusterIdState(fallback.id);
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load clusters");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    reload();
  }, []);

  function setClusterId(id: string) {
    setSelectedClusterId(id);
    setClusterIdState(id);
  }

  const cluster = clusters.find((c) => c.id === clusterId) ?? null;

  return (
    <ClusterContext.Provider value={{ clusters, clusterId, cluster, setClusterId, reload, loading, error }}>
      {children}
    </ClusterContext.Provider>
  );
}

export function useCluster() {
  const ctx = useContext(ClusterContext);
  if (!ctx) {
    throw new Error("useCluster must be used within ClusterProvider");
  }
  return ctx;
}
