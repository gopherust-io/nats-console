import { useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { Link, Navigate, useSearchParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { motion } from "motion/react";
import TopologyInspector from "../components/TopologyInspector";
import TopologyTree from "../components/TopologyTree";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import QueryErrorState from "../components/ui/QueryErrorState";
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
import { MONITORING_POLL_MS } from "../lib/constants";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";
import { useTopologyMotion } from "../lib/topologyMotion";
import {
  fetchZombies,
  sortZombieFindings,
  zombieFindingHref,
  zombieFindingLabel,
  ZOMBIES_LOCATION_STATE,
  type ZombieFinding,
} from "../lib/zombie";
import {
  fetchSubjectNaming,
  NAMING_LOCATION_STATE,
  sortSubjectNamingFindings,
  subjectNamingFindingHref,
  subjectNamingFindingLabel,
  type SubjectNamingFinding,
} from "../lib/subjectNaming";
import {
  eventGenomeCatalogHref,
  eventGenomeFindingHref,
  eventGenomeFindingLabel,
  fetchEventGenome,
  GENOME_LOCATION_STATE,
  sortEventGenomeFindings,
  type EventGenomeFinding,
} from "../lib/eventGenome";
import { ARCHITECTURE_REVIEW_HREF } from "../lib/architectureReview";
import { downloadArchitectureExport } from "../lib/architectureExport";
import { fetchAssistantConfig } from "../lib/assistant";

const FILTER_DEBOUNCE_MS = 200;

type TopologyView = "constellation" | "zombies" | "naming" | "genome";

function parseTopologyView(value: string | null): TopologyView {
  if (value === "zombies") return "zombies";
  if (value === "naming") return "naming";
  if (value === "genome") return "genome";
  return "constellation";
}

function zombieKindLabel(t: (key: string) => string, kind: string): string {
  switch (kind) {
    case "empty_stream":
      return t("topology.zombiesKindEmptyStream");
    case "idle_consumer":
      return t("topology.zombiesKindIdleConsumer");
    case "unconsumed_subject":
      return t("topology.zombiesKindUnconsumedSubject");
    case "unpublished_subject":
      return t("topology.zombiesKindUnpublishedSubject");
    case "unbound_consumer":
      return t("topology.zombiesKindUnboundConsumer");
    default:
      return kind;
  }
}

function namingKindLabel(t: (key: string) => string, kind: string): string {
  switch (kind) {
    case "wrong_case":
      return t("topology.namingKindWrongCase");
    case "missing_dots":
      return t("topology.namingKindMissingDots");
    case "non_dot_separator":
      return t("topology.namingKindNonDotSeparator");
    case "shallow_hierarchy":
      return t("topology.namingKindShallowHierarchy");
    case "inconsistent_variant":
      return t("topology.namingKindInconsistentVariant");
    default:
      return kind;
  }
}

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
  const [searchParams, setSearchParams] = useSearchParams();
  const legacyReview = searchParams.get("view") === "review";
  const view = parseTopologyView(searchParams.get("view"));
  const setView = useCallback(
    (next: TopologyView) => {
      setSearchParams(
        (prev) => {
          const nextParams = new URLSearchParams(prev);
          if (next === "zombies" || next === "naming" || next === "genome") {
            nextParams.set("view", next);
          } else {
            nextParams.delete("view");
          }
          return nextParams;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );
  const [filterInput, setFilterInput] = useState("");
  const [filterQuery, setFilterQuery] = useState("");
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);

  const assistantConfigQuery = useQuery({
    queryKey: ["assistant-config"],
    queryFn: fetchAssistantConfig,
    staleTime: 60_000,
  });

  const topologyQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "topology"),
    queryFn: () => fetchTopology(clusterId!, cluster?.name ?? "Cluster", { fresh: true }),
    enabled: Boolean(clusterId) && view === "constellation",
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS),
  });

  const zombiesQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "zombies"),
    queryFn: () => fetchZombies(clusterId!, { fresh: true }),
    enabled: Boolean(clusterId) && view === "zombies",
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS),
  });

  const namingQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "subject-naming"),
    queryFn: () => fetchSubjectNaming(clusterId!, { fresh: true }),
    enabled: Boolean(clusterId) && view === "naming",
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS),
  });

  const genomeQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "event-genome"),
    queryFn: () => fetchEventGenome(clusterId!, { fresh: true }),
    enabled: Boolean(clusterId) && view === "genome",
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS),
  });

  useEffect(() => {
    const timer = window.setTimeout(() => setFilterQuery(filterInput.trim()), FILTER_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [filterInput]);

  const error = topologyQuery.isError;
  const zombiesError = zombiesQuery.isError;
  const namingError = namingQuery.isError;
  const genomeError = genomeQuery.isError;
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

  const zombieFindings = useMemo(
    () => sortZombieFindings(zombiesQuery.data?.findings ?? []),
    [zombiesQuery.data?.findings],
  );
  const zombieTotal = zombiesQuery.data?.totals.total ?? zombieFindings.length;

  const namingFindings = useMemo(
    () => sortSubjectNamingFindings(namingQuery.data?.findings ?? []),
    [namingQuery.data?.findings],
  );
  const namingTotal = namingQuery.data?.totals.total ?? namingFindings.length;

  const genomeFindings = useMemo(
    () => sortEventGenomeFindings(genomeQuery.data?.findings ?? []),
    [genomeQuery.data?.findings],
  );
  const genomeTotal = genomeQuery.data?.totals.total ?? genomeFindings.length;

  const selectedNode = useMemo(() => {
    if (!filteredRoot || !selectedNodeId) return null;
    return findNodeById(filteredRoot, selectedNodeId);
  }, [filteredRoot, selectedNodeId]);

  const parentStream = useMemo(() => {
    if (!filteredRoot || !selectedNodeId) return null;
    return findParentStream(filteredRoot, selectedNodeId);
  }, [filteredRoot, selectedNodeId]);

  useEffect(() => {
    if (!filteredRoot || !selectedNodeId) {
      if (!filteredRoot) setSelectedNodeId(null);
      return;
    }
    if (!findNodeById(filteredRoot, selectedNodeId)) {
      setSelectedNodeId(null);
    }
  }, [filteredRoot, selectedNodeId]);

  const handleSelectNode = useCallback((node: TopologyNode) => {
    if (node.kind === "cluster") return;
    setSelectedNodeId(node.id);
  }, []);

  const handleClearSelection = useCallback(() => {
    setSelectedNodeId(null);
  }, []);

  const flowStream =
    selectedNode && selectedNode.kind !== "cluster"
      ? parentStream ?? (selectedNode.kind === "stream" ? selectedNode : null)
      : null;
  const showSignalOverlay = Boolean(selectedNode && flowStream);

  useEffect(() => {
    if (!showSignalOverlay) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, [showSignalOverlay]);

  const layoutEnabled = filterQuery === "" && !topologyQuery.isFetching;
  const hasStreams = streamList.length > 0;
  const showConstellation = view === "constellation";
  const showZombies = view === "zombies";
  const showNaming = view === "naming";
  const showGenome = view === "genome";
  const refreshBusy =
    topologyQuery.isFetching ||
    zombiesQuery.isFetching ||
    namingQuery.isFetching ||
    genomeQuery.isFetching;

  const handleGenerateArchitecture = useCallback(async () => {
    if (!clusterId || exporting) return;
    setExporting(true);
    try {
      await downloadArchitectureExport(clusterId, {
        fresh: true,
        ai: Boolean(assistantConfigQuery.data?.aiEnabled),
      });
    } catch {
      // keep toolbar quiet; operators can retry
    } finally {
      setExporting(false);
    }
  }, [assistantConfigQuery.data?.aiEnabled, clusterId, exporting]);

  if (legacyReview) {
    return <Navigate to={ARCHITECTURE_REVIEW_HREF} replace />;
  }

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
            {showConstellation && (
              <input
                className="topology-toolbar__search"
                type="search"
                placeholder={t("topology.filterPlaceholder")}
                value={filterInput}
                onChange={(event) => setFilterInput(event.target.value)}
                aria-label={t("topology.filterAria")}
              />
            )}
            <button
              className="btn btn--primary"
              type="button"
              onClick={() => void handleGenerateArchitecture()}
              disabled={!clusterId || exporting}
            >
              {exporting ? t("archExport.generating") : t("archExport.generate")}
            </button>
            <button
              className="btn btn--secondary"
              type="button"
              onClick={() => {
                topologyQuery.refetch();
                zombiesQuery.refetch();
                namingQuery.refetch();
                genomeQuery.refetch();
              }}
              disabled={refreshBusy}
            >
              {t("common.refresh")}
            </button>
          </div>
        }
      />

      <nav className="nc-tabs topology-tabs" aria-label={t("topology.tabsAria")}>
        <button
          type="button"
          className={`nc-tab${showConstellation ? " active" : ""}`}
          aria-current={showConstellation ? "page" : undefined}
          onClick={() => setView("constellation")}
        >
          {t("topology.tabConstellation")}
        </button>
        <button
          type="button"
          className={`nc-tab${showZombies ? " active" : ""}`}
          aria-current={showZombies ? "page" : undefined}
          onClick={() => setView("zombies")}
        >
          {t("topology.tabZombies")}
          {!zombiesQuery.isLoading && !zombiesError && zombieTotal > 0 && (
            <span className="topology-tabs__count">{zombieTotal}</span>
          )}
        </button>
        <button
          type="button"
          className={`nc-tab${showNaming ? " active" : ""}`}
          aria-current={showNaming ? "page" : undefined}
          onClick={() => setView("naming")}
        >
          {t("topology.tabNaming")}
          {!namingQuery.isLoading && !namingError && namingTotal > 0 && (
            <span className="topology-tabs__count">{namingTotal}</span>
          )}
        </button>
        <button
          type="button"
          className={`nc-tab${showGenome ? " active" : ""}`}
          aria-current={showGenome ? "page" : undefined}
          onClick={() => setView("genome")}
        >
          {t("topology.tabGenome")}
          {!genomeQuery.isLoading && !genomeError && genomeTotal > 0 && (
            <span className="topology-tabs__count">{genomeTotal}</span>
          )}
        </button>
      </nav>

      {showConstellation && error && (
        <QueryErrorState error={topologyQuery.error} onRetry={() => void topologyQuery.refetch()} />
      )}
      {showZombies && zombiesError && (
        <QueryErrorState error={zombiesQuery.error} onRetry={() => void zombiesQuery.refetch()} />
      )}
      {showNaming && namingError && (
        <QueryErrorState error={namingQuery.error} onRetry={() => void namingQuery.refetch()} />
      )}
      {showGenome && genomeError && (
        <QueryErrorState error={genomeQuery.error} onRetry={() => void genomeQuery.refetch()} />
      )}

      {topologyQuery.isLoading && <div className="skeleton skeleton--panel" />}

      {showConstellation && counts && (
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

      {showConstellation && filteredRoot && filteredRoot.children.length === 0 && !filterQuery && (
        <EmptyState title={t("topology.emptyTitle")} description={t("topology.emptyDescription")} />
      )}

      {showConstellation && filteredRoot && hasStreams && (
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
                layoutEnabled={layoutEnabled}
              />
            </div>
          </motion.div>
        </div>
      )}

      {showSignalOverlay &&
        selectedNode &&
        flowStream &&
        createPortal(
          <div
            className="topo-signal-overlay"
            role="presentation"
            onClick={handleClearSelection}
          >
            <div
              className="topo-signal-overlay__panel"
              role="dialog"
              aria-modal="true"
              onClick={(event) => event.stopPropagation()}
            >
              <TopologyInspector
                selected={selectedNode}
                stream={flowStream}
                onClose={handleClearSelection}
                onSelectNode={handleSelectNode}
              />
            </div>
          </div>,
          document.body,
        )}

      {showConstellation && filteredRoot && filteredRoot.children.length === 0 && filterQuery && (
        <EmptyState title={t("topology.noMatchesTitle")} description={t("topology.noMatchesDescription", { query: filterQuery })} />
      )}

      {showConstellation && root && filterQuery && !filteredRoot && (
        <EmptyState title={t("topology.noMatchesTitle")} description={t("topology.noMatchesDescription", { query: filterQuery })} />
      )}

      {showZombies && (
        <section className="topology-zombies topology-zombies--panel" aria-label={t("topology.zombiesTitle")}>
          <div className="topology-zombies__header">
            <div>
              <h3 className="topology-zombies__title">{t("topology.zombiesTitle")}</h3>
              <p className="topology-zombies__subtitle">{t("topology.zombiesSubtitle")}</p>
            </div>
            {!zombiesQuery.isLoading && !zombiesError && (
              <span className="badge">{t("topology.zombiesTotal", { count: zombieTotal })}</span>
            )}
          </div>
          {zombiesQuery.isLoading && <p className="topology-zombies__empty">{t("common.loading")}</p>}
          {!zombiesQuery.isLoading && !zombiesError && zombieFindings.length === 0 && (
            <EmptyState title={t("topology.zombiesEmpty")} description={t("topology.zombiesSubtitle")} />
          )}
          {!zombiesQuery.isLoading && !zombiesError && zombieFindings.length > 0 && (
            <ul className="topology-zombies__list">
              {zombieFindings.map((finding: ZombieFinding, index: number) => {
                const href =
                  clusterId != null
                    ? zombieFindingHref(finding, clusterId, accountName || "Default")
                    : null;
                return (
                  <li key={`${finding.kind}-${finding.stream}-${finding.consumer}-${finding.subject}-${index}`}>
                    <div className="topology-zombies__item">
                      <div>
                        <span className="topology-zombies__kind">{zombieKindLabel(t, finding.kind)}</span>
                        <span className="topology-zombies__label">{zombieFindingLabel(finding)}</span>
                      </div>
                      {href && (
                        <Link className="btn btn--secondary" to={href} state={ZOMBIES_LOCATION_STATE}>
                          {t("topology.zombiesOpen")}
                        </Link>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </section>
      )}

      {showNaming && (
        <section className="topology-zombies topology-zombies--panel" aria-label={t("topology.namingTitle")}>
          <div className="topology-zombies__header">
            <div>
              <h3 className="topology-zombies__title">{t("topology.namingTitle")}</h3>
              <p className="topology-zombies__subtitle">{t("topology.namingSubtitle")}</p>
            </div>
            {!namingQuery.isLoading && !namingError && (
              <span className="badge">{t("topology.namingTotal", { count: namingTotal })}</span>
            )}
          </div>
          {namingQuery.isLoading && <p className="topology-zombies__empty">{t("common.loading")}</p>}
          {!namingQuery.isLoading && !namingError && namingFindings.length === 0 && (
            <EmptyState title={t("topology.namingEmpty")} description={t("topology.namingSubtitle")} />
          )}
          {!namingQuery.isLoading && !namingError && namingFindings.length > 0 && (
            <ul className="topology-zombies__list">
              {namingFindings.map((finding: SubjectNamingFinding, index: number) => {
                const href =
                  clusterId != null
                    ? subjectNamingFindingHref(finding, clusterId, accountName || "Default")
                    : null;
                return (
                  <li key={`${finding.kind}-${finding.stream}-${finding.consumer}-${finding.subject}-${index}`}>
                    <div className="topology-zombies__item">
                      <div>
                        <span className="topology-zombies__kind">{namingKindLabel(t, finding.kind)}</span>
                        <span className="topology-zombies__label">{subjectNamingFindingLabel(finding)}</span>
                        {finding.suggested && (
                          <span className="topology-zombies__suggested">
                            {t("topology.namingSuggested")}: <strong>{finding.suggested}</strong>
                          </span>
                        )}
                        {finding.cluster && finding.cluster.length > 1 && (
                          <div className="topology-zombies__cluster">
                            {finding.cluster.map((peer) => (
                              <span key={peer} className="topology-zombies__chip">
                                {peer}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>
                      {href && (
                        <Link className="btn btn--secondary" to={href} state={NAMING_LOCATION_STATE}>
                          {t("topology.namingOpen")}
                        </Link>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </section>
      )}

      {showGenome && (
        <section className="topology-zombies topology-zombies--panel" aria-label={t("topology.genomeTitle")}>
          <div className="topology-zombies__header">
            <div>
              <h3 className="topology-zombies__title">{t("topology.genomeTitle")}</h3>
              <p className="topology-zombies__subtitle">{t("topology.genomeSubtitle")}</p>
            </div>
            {!genomeQuery.isLoading && !genomeError && (
              <span className="badge">{t("topology.genomeTotal", { count: genomeTotal })}</span>
            )}
          </div>
          {genomeQuery.isLoading && <p className="topology-zombies__empty">{t("common.loading")}</p>}
          {!genomeQuery.isLoading && !genomeError && genomeFindings.length === 0 && (
            <EmptyState title={t("topology.genomeEmpty")} description={t("topology.genomeSubtitle")} />
          )}
          {!genomeQuery.isLoading && !genomeError && genomeFindings.length > 0 && (
            <ul className="topology-zombies__list">
              {genomeFindings.map((finding: EventGenomeFinding, index: number) => {
                const href =
                  clusterId != null
                    ? eventGenomeFindingHref(finding, clusterId, accountName || "Default")
                    : null;
                return (
                  <li key={`${finding.genome}-${finding.stream}-${finding.consumer}-${finding.subject}-${index}`}>
                    <div className="topology-zombies__item">
                      <div>
                        <span className="topology-zombies__kind">{finding.genome}</span>
                        <span className="topology-zombies__label">{eventGenomeFindingLabel(finding)}</span>
                        {finding.suggested && (
                          <span className="topology-zombies__suggested">
                            {t("topology.genomeSuggested")}: <strong>{finding.suggested}</strong>
                          </span>
                        )}
                        {finding.cluster && finding.cluster.length > 1 && (
                          <div className="topology-zombies__cluster">
                            {finding.cluster.map((peer) => (
                              <span key={peer} className="topology-zombies__chip">
                                {peer}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>
                      <div className="topology-zombies__actions">
                        <Link className="btn btn--secondary" to={eventGenomeCatalogHref(finding.subject)}>
                          {t("topology.genomeCatalog")}
                        </Link>
                        {href && (
                          <Link className="btn btn--secondary" to={href} state={GENOME_LOCATION_STATE}>
                            {t("topology.genomeOpen")}
                          </Link>
                        )}
                      </div>
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </section>
      )}
    </div>
  );
}
