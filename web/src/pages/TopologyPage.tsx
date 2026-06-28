import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import TopologyStreamDetail from "../components/TopologyStreamDetail";
import TopologyStreamOverview from "../components/TopologyStreamOverview";
import TopologyTree from "../components/TopologyTree";
import TopologyFlowDiagram from "../components/TopologyFlowDiagram";
import Alert from "../components/ui/Alert";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import StatCard from "../components/ui/StatCard";
import {
  countTopology,
  fetchTopology,
  filterTopology,
  findStreamById,
  getStreamNodes,
  sortStreamNodes,
  type StreamOverviewSort,
} from "../lib/topology";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

type TopologyView = "overview" | "explorer";

const FLOW_AUTO_MAX = 3;
const FILTER_DEBOUNCE_MS = 200;

export default function TopologyPage() {
  const { clusterId, cluster } = useCluster();
  const [filterInput, setFilterInput] = useState("");
  const [filterQuery, setFilterQuery] = useState("");
  const [treeExpanded, setTreeExpanded] = useState(false);
  const [selectedStreamId, setSelectedStreamId] = useState<string | null>(null);
  const [sortBy, setSortBy] = useState<StreamOverviewSort>("name");
  const [view, setView] = useState<TopologyView>("overview");
  const [showTree, setShowTree] = useState(true);

  const topologyQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "topology"),
    queryFn: () => fetchTopology(clusterId!, cluster?.name ?? "Cluster"),
    enabled: Boolean(clusterId),
    refetchInterval: 30_000,
  });

  useEffect(() => {
    const timer = window.setTimeout(() => setFilterQuery(filterInput.trim()), FILTER_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [filterInput]);

  const error = topologyQuery.error instanceof Error ? topologyQuery.error.message : "";
  const root = topologyQuery.data ?? null;

  const filteredRoot = useMemo(() => {
    if (!root) return null;
    return filterTopology(root, filterQuery) ?? null;
  }, [root, filterQuery]);

  const counts = useMemo(() => (root ? countTopology(root) : null), [root]);
  const streams = useMemo(
    () => (filteredRoot ? sortStreamNodes(getStreamNodes(filteredRoot), sortBy) : []),
    [filteredRoot, sortBy],
  );

  const selectedStream = useMemo(() => {
    if (!filteredRoot || !selectedStreamId) return null;
    return findStreamById(filteredRoot, selectedStreamId);
  }, [filteredRoot, selectedStreamId]);

  useEffect(() => {
    if (!filteredRoot) {
      setSelectedStreamId(null);
      return;
    }
    const available = getStreamNodes(filteredRoot);
    if (available.length === 0) {
      setSelectedStreamId(null);
      return;
    }
    if (selectedStreamId && !available.some((stream) => stream.id === selectedStreamId)) {
      setSelectedStreamId(available.length === 1 ? available[0].id : null);
    }
    if (filterQuery && available.length === 1 && !selectedStreamId) {
      setSelectedStreamId(available[0].id);
    }
  }, [filteredRoot, filterQuery, selectedStreamId]);

  useEffect(() => {
    if (!counts) return;
    setTreeExpanded(counts.streams <= 4);
    setShowTree(counts.streams <= 12);
    if (counts.streams > 12) {
      setView("overview");
    }
  }, [counts]);

  const showGlobalFlow = streams.length > 0 && streams.length <= FLOW_AUTO_MAX && !selectedStream;

  return (
    <div className="page">
      <PageHeader
        eyebrow="JetStream"
        title="Topology"
        subtitle="Explore how subjects, streams, and consumers relate."
        badge={
          cluster ? (
            <span className="badge badge--live">
              {cluster.name}
              {topologyQuery.isFetching && <span className="badge__pulse" aria-label="Refreshing" />}
            </span>
          ) : undefined
        }
        actions={
          <div className="topology-toolbar">
            <input
              className="topology-toolbar__search"
              type="search"
              placeholder="Filter streams, subjects, consumers…"
              value={filterInput}
              onChange={(event) => setFilterInput(event.target.value)}
              aria-label="Filter topology"
            />
            <div className="topology-toolbar__views" role="tablist" aria-label="Topology view">
              <button
                type="button"
                role="tab"
                aria-selected={view === "overview"}
                className={`topology-toolbar__tab${view === "overview" ? " topology-toolbar__tab--active" : ""}`}
                onClick={() => setView("overview")}
              >
                Overview
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={view === "explorer"}
                className={`topology-toolbar__tab${view === "explorer" ? " topology-toolbar__tab--active" : ""}`}
                onClick={() => setView("explorer")}
              >
                Explorer
              </button>
            </div>
            <button className="btn btn--secondary" type="button" onClick={() => setTreeExpanded(true)}>
              Expand all
            </button>
            <button className="btn btn--secondary" type="button" onClick={() => setTreeExpanded(false)}>
              Collapse all
            </button>
            <button
              className="btn btn--secondary"
              type="button"
              onClick={() => topologyQuery.refetch()}
              disabled={topologyQuery.isFetching}
            >
              Refresh
            </button>
          </div>
        }
      />

      <Alert variant="error">{error}</Alert>

      {topologyQuery.isLoading && <div className="skeleton skeleton--panel" />}

      {counts && (
        <div className="stat-grid stat-grid--3 mb-24">
          <StatCard label="Streams" value={counts.streams} accent="sky" icon="▤" />
          <StatCard label="Subjects" value={counts.subjects} accent="emerald" icon="◎" />
          <StatCard label="Consumers" value={counts.consumers} accent="violet" icon="◉" />
        </div>
      )}

      {filteredRoot && filteredRoot.children.length === 0 && !filterQuery && (
        <EmptyState
          title="No JetStream topology yet"
          description='Run `make seed-demo` to load sample streams and consumers, or create a stream from the Streams page.'
        />
      )}

      {filteredRoot && streams.length > 0 && (
        <TopologyStreamOverview
          streams={streams}
          selectedStreamId={selectedStreamId}
          sortBy={sortBy}
          onSortChange={setSortBy}
          onSelectStream={setSelectedStreamId}
        />
      )}

      {filteredRoot && streams.length > 0 && selectedStream && (
        <TopologyStreamDetail stream={selectedStream} onClose={() => setSelectedStreamId(null)} />
      )}

      {filteredRoot && streams.length > 0 && showGlobalFlow && (
        <TopologyFlowDiagram streams={streams} maxStreams={FLOW_AUTO_MAX} />
      )}

      {filteredRoot && streams.length > 0 && (view === "explorer" || showTree) && (
        <TopologyTree
          root={filteredRoot}
          defaultExpanded={treeExpanded}
          expandAll={treeExpanded}
          selectedStreamId={selectedStreamId}
        />
      )}

      {filteredRoot && filteredRoot.children.length === 0 && filterQuery && (
        <EmptyState title="No matches" description={`Nothing matched "${filterQuery}". Try another filter.`} />
      )}

      {root && filterQuery && !filteredRoot && (
        <EmptyState title="No matches" description={`Nothing matched "${filterQuery}". Try another filter.`} />
      )}
    </div>
  );
}
