import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { QUERY_GC_TIME_MS, QUERY_STALE_TIME_MS } from "./constants";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: QUERY_STALE_TIME_MS,
      gcTime: QUERY_GC_TIME_MS,
      refetchOnWindowFocus: true,
      refetchIntervalInBackground: false,
      retry: 1,
    },
  },
});

/** Pause interval refetches while the document is hidden. */
export function visibilityAwareInterval(ms: number): number | false | (() => number | false) {
  return () => {
    if (typeof document !== "undefined" && document.visibilityState === "hidden") {
      return false;
    }
    return ms;
  };
}

export function QueryProvider({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

export function clusterQueryKey(clusterId: string | null, suffix: string) {
  return ["cluster", clusterId, suffix] as const;
}

/** Mark streams + topology stale after JetStream shape changes (create/update/delete). */
export async function invalidateJetStreamTopology(clusterId: string | null | undefined) {
  if (!clusterId) return;
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "streams") }),
    queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "topology") }),
  ]);
}
