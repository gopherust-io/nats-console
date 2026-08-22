import { useMemo, useState, useDeferredValue } from "react";
import { useTranslation } from "react-i18next";
import { useQueries } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router";
import VirtualTable, { type VirtualTableColumn } from "../components/VirtualTable";
import { useCluster } from "../lib/cluster";
import { MONITORING_POLL_MS } from "../lib/constants";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";
import { fetchAllStreams } from "../lib/streams";


type UnifiedRow = {
  key: string;
  clusterId: string;
  clusterName: string;
  streamName: string;
  messages: number;
  bytes: number;
  consumers: number;
  storage: string;
  error?: string;
};

export default function AllStreamsPage() {
  const { t } = useTranslation();
  const { clusters, clusterId: selectedClusterId, setClusterId, loading: clustersLoading } = useCluster();
  const navigate = useNavigate();
  const [search, setSearch] = useState("");
  const deferredSearch = useDeferredValue(search);

  const streamQueries = useQueries({
    queries: clusters.map((cluster) => {
      const focused = cluster.id === selectedClusterId;
      return {
        queryKey: clusterQueryKey(cluster.id, "streams"),
        queryFn: async () => fetchAllStreams(cluster.id),
        // Only the selected cluster keeps a live poll; others fetch once.
        refetchInterval: focused ? visibilityAwareInterval(MONITORING_POLL_MS) : false,
        staleTime: focused ? MONITORING_POLL_MS : 5 * 60_000,
        refetchOnWindowFocus: focused,
      };
    }),
  });

  const rows = useMemo(() => {
    const out: UnifiedRow[] = [];
    clusters.forEach((cluster, index) => {
      const q = streamQueries[index];
      if (q?.isError) {
        out.push({
          key: `${cluster.id}:error`,
          clusterId: cluster.id,
          clusterName: cluster.name,
          streamName: "—",
          messages: 0,
          bytes: 0,
          consumers: 0,
          storage: "—",
          error: t("allStreams.clusterError", { name: cluster.name }),
        });
        return;
      }
      for (const stream of q?.data ?? []) {
        out.push({
          key: `${cluster.id}:${stream.config.name}`,
          clusterId: cluster.id,
          clusterName: cluster.name,
          streamName: stream.config.name,
          messages: stream.state.messages,
          bytes: stream.state.bytes,
          consumers: stream.state.consumerCount,
          storage: stream.config.storage,
        });
      }
    });
    const q = deferredSearch.trim().toLowerCase();
    if (!q) return out;
    return out.filter(
      (row) =>
        row.clusterName.toLowerCase().includes(q) ||
        row.streamName.toLowerCase().includes(q) ||
        (row.error ?? "").toLowerCase().includes(q),
    );
  }, [clusters, streamQueries, deferredSearch, t]);

  const loading = clustersLoading || streamQueries.some((q) => q.isLoading || q.isFetching);

  const columns = useMemo<VirtualTableColumn[]>(
    () => [
      { id: "cluster", header: t("allStreams.cluster"), width: "minmax(120px, 1fr)" },
      { id: "stream", header: t("allStreams.stream"), width: "minmax(140px, 1.2fr)" },
      { id: "messages", header: t("allStreams.messages"), width: "100px", align: "right" },
      { id: "bytes", header: t("allStreams.bytes"), width: "100px", align: "right" },
      { id: "consumers", header: t("allStreams.consumers"), width: "100px", align: "right" },
      { id: "storage", header: t("allStreams.storage"), width: "96px" },
      { id: "actions", header: "", width: "100px", align: "right" },
    ],
    [t],
  );

  function openStream(row: UnifiedRow) {
    if (row.error) return;
    setClusterId(row.clusterId);
    navigate(
      `/systems/${row.clusterId}/accounts/Default/jetstream/streams/${encodeURIComponent(row.streamName)}`,
    );
  }

  return (
    <div>
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("allStreams.title")}</h1>
          <p className="nc-page-sub">{t("allStreams.subtitle")}</p>
        </div>
        <Link className="btn secondary" to="/systems">
          {t("nav.systems")}
        </Link>
      </div>

      <div className="nc-toolbar">
        <input
          className="nc-search"
          placeholder={t("common.searchPlaceholder")}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {loading && rows.length === 0 && <p className="text-muted">{t("allStreams.loading")}</p>}

      {rows.length === 0 && !loading ? (
        <div className="nc-empty">{t("allStreams.empty")}</div>
      ) : (
        <div className="table-wrap">
          <VirtualTable
            columns={columns}
            items={rows}
            empty={t("allStreams.empty")}
            getKey={(row) => row.key}
            renderCell={(row, columnId) => {
              if (row.error) {
                if (columnId === "cluster") return row.clusterName;
                if (columnId === "stream") return <span className="text-muted">{row.error}</span>;
                return "—";
              }
              switch (columnId) {
                case "cluster":
                  return row.clusterName;
                case "stream":
                  return (
                    <button type="button" className="linkish" onClick={() => openStream(row)}>
                      {row.streamName}
                    </button>
                  );
                case "messages":
                  return row.messages;
                case "bytes":
                  return row.bytes;
                case "consumers":
                  return row.consumers;
                case "storage":
                  return row.storage;
                case "actions":
                  return (
                    <button type="button" className="btn secondary btn--small" onClick={() => openStream(row)}>
                      {t("allStreams.open")}
                    </button>
                  );
                default:
                  return null;
              }
            }}
          />
        </div>
      )}
    </div>
  );
}
