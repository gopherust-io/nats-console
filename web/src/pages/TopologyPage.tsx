import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { motion } from "motion/react";
import TopologyInspector from "../components/TopologyInspector";
import TopologyTree from "../components/TopologyTree";
import Alert from "../components/ui/Alert";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import StatCard from "../components/ui/StatCard";
import {
  countTopology,
  fetchTopology,
  filterTopology,
  findNodeById,
  findParentStream,
  getStreamNodes,
  withJetStreamHrefs,
  type TopologyNode,
} from "../lib/topology";
import { useCluster } from "../lib/cluster";
import { useAccount } from "../lib/account";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";
import { useTopologyMotion } from "../lib/topologyMotion";

const FILTER_DEBOUNCE_MS = 200;

export default function TopologyPage() {
  const { t } = useTranslation();
  const { clusterId, cluster } = useCluster();
  const { accountName } = useAccount();
  const {
    explorerInitial,
    explorerAnimate,
    transition,
    softSpring,
    statsVariants,
    statItemVariants,
  } = useTopologyMotion();
  const [filterInput, setFilterInput] = useState("");
  const [filterQuery, setFilterQuery] = useState("");
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);

  const topologyQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "topology"),
    queryFn: () => fetchTopology(clusterId!, cluster?.name ?? "Cluster", { fresh: true }),
    enabled: Boolean(clusterId),
    refetchInterval: visibilityAwareInterval(30_000),
  });

  useEffect(() => {
    const timer = window.setTimeout(() => setFilterQuery(filterInput.trim()), FILTER_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [filterInput]);

  const error = topologyQuery.error instanceof Error ? topologyQuery.error.message : "";
  const root = useMemo(() => {
    if (!topologyQuery.data || !clusterId) return null;
    return withJetStreamHrefs(topologyQuery.data, clusterId, accountName || "Default");
  }, [topologyQuery.data, clusterId, accountName]);

  const filteredRoot = useMemo(() => {
    if (!root) return null;
    return filterTopology(root, filterQuery) ?? null;
  }, [root, filterQuery]);

  const counts = useMemo(() => (root ? countTopology(root) : null), [root]);
  const streamList = useMemo(() => (filteredRoot ? getStreamNodes(filteredRoot) : []), [filteredRoot]);

  const selectedNode = useMemo(() => {
    if (!filteredRoot || !selectedNodeId) return null;
    return findNodeById(filteredRoot, selectedNodeId);
  }, [filteredRoot, selectedNodeId]);

  const parentStream = useMemo(() => {
    if (!filteredRoot || !selectedNodeId) return null;
    return findParentStream(filteredRoot, selectedNodeId);
  }, [filteredRoot, selectedNodeId]);

  useEffect(() => {
    if (!filteredRoot) {
      setSelectedNodeId(null);
      return;
    }
    const available = getStreamNodes(filteredRoot);
    if (available.length === 0) {
      setSelectedNodeId(null);
      return;
    }
    if (selectedNodeId && !findNodeById(filteredRoot, selectedNodeId)) {
      setSelectedNodeId(available.length === 1 ? available[0].id : null);
    }
    if (filterQuery && available.length === 1 && !selectedNodeId) {
      setSelectedNodeId(available[0].id);
    }
  }, [filteredRoot, filterQuery, selectedNodeId]);

  const handleSelectNode = (node: TopologyNode) => {
    if (node.kind === "cluster") return;
    setSelectedNodeId(node.id);
  };

  const hasStreams = streamList.length > 0;

  return (
    <div className="page topology-page">
      <PageHeader
        eyebrow={t("topology.eyebrow")}
        title={t("topology.title")}
        subtitle={t("topology.subtitle")}
        badge={
          cluster ? (
            <span className="badge badge--live">
              {cluster.name}
              {topologyQuery.isFetching && <span className="badge__pulse" aria-label={t("topology.refreshing")} />}
            </span>
          ) : undefined
        }
        actions={
          <div className="topology-toolbar">
            <input
              className="topology-toolbar__search"
              type="search"
              placeholder={t("topology.filterPlaceholder")}
              value={filterInput}
              onChange={(event) => setFilterInput(event.target.value)}
              aria-label={t("topology.filterAria")}
            />
            <button
              className="btn btn--secondary"
              type="button"
              onClick={() => topologyQuery.refetch()}
              disabled={topologyQuery.isFetching}
            >
              {t("common.refresh")}
            </button>
          </div>
        }
      />

      <Alert variant="error">{error}</Alert>

      {topologyQuery.isLoading && <div className="skeleton skeleton--panel" />}

      {counts && (
        <motion.div
          className="stat-grid stat-grid--3 mb-24"
          variants={statsVariants}
          initial="hidden"
          animate="visible"
        >
          <motion.div variants={statItemVariants} transition={softSpring}>
            <StatCard label={t("systems.streams")} value={counts.streams} accent="sky" icon="▤" />
          </motion.div>
          <motion.div variants={statItemVariants} transition={softSpring}>
            <StatCard label={t("topology.subjects")} value={counts.subjects} accent="emerald" icon="◎" />
          </motion.div>
          <motion.div variants={statItemVariants} transition={softSpring}>
            <StatCard label={t("systems.consumers")} value={counts.consumers} accent="violet" icon="◉" />
          </motion.div>
        </motion.div>
      )}

      {filteredRoot && filteredRoot.children.length === 0 && !filterQuery && (
        <EmptyState title={t("topology.emptyTitle")} description={t("topology.emptyDescription")} />
      )}

      {filteredRoot && hasStreams && (
        <div className="topology-stage">
          <motion.div
            className="topology-explorer"
            initial={explorerInitial}
            animate={explorerAnimate}
            transition={transition}
          >
            <div className="topology-explorer__hierarchy">
              <TopologyTree
                root={filteredRoot}
                selectedNodeId={selectedNodeId}
                onSelectNode={handleSelectNode}
              />
            </div>
            <TopologyInspector
              selected={selectedNode}
              stream={parentStream}
              streams={streamList}
              root={filteredRoot}
              onClose={() => setSelectedNodeId(null)}
              onSelectStream={handleSelectNode}
            />
          </motion.div>
        </div>
      )}

      {filteredRoot && filteredRoot.children.length === 0 && filterQuery && (
        <EmptyState title={t("topology.noMatchesTitle")} description={t("topology.noMatchesDescription", { query: filterQuery })} />
      )}

      {root && filterQuery && !filteredRoot && (
        <EmptyState title={t("topology.noMatchesTitle")} description={t("topology.noMatchesDescription", { query: filterQuery })} />
      )}
    </div>
  );
}
