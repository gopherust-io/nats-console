import { lazy, Suspense, useEffect } from "react";
import { Link } from "react-router";
import { AnimatePresence, motion } from "motion/react";
import { useTranslation } from "react-i18next";
import type { TopologyNode } from "../lib/topology";
import { splitStreamChildren, TOPOLOGY_LOCATION_STATE } from "../lib/topology";
import { useTopologyMotion } from "../lib/topologyMotion";

const TopologyFlowDiagram = lazy(() => import("./TopologyFlowDiagram"));

type TopologyInspectorProps = {
  selected: TopologyNode | null;
  stream: TopologyNode | null;
  onClose: () => void;
  onSelectNode?: (node: TopologyNode) => void;
};

function kindEyebrow(kind: TopologyNode["kind"], t: (key: string) => string): string {
  switch (kind) {
    case "stream":
      return t("topology.inspectorStream");
    case "subject":
      return t("topology.inspectorSubject");
    case "consumer":
      return t("topology.inspectorConsumer");
    default:
      return t("topology.inspectorNode");
  }
}

function SelectionMeta({ selected }: { selected: TopologyNode }) {
  const { t } = useTranslation();
  return (
    <div className="topology-detail__meta">
      {selected.role === "leader" && (
        <span className="topology-detail__chip topology-detail__chip--leader">{t("topology.roleLeader")}</span>
      )}
      {selected.role === "replica" && (
        <span className="topology-detail__chip topology-detail__chip--replica">{t("topology.roleReplica")}</span>
      )}
      {selected.status === "unhealthy" && (
        <span className="topology-detail__chip topology-detail__chip--danger">{t("topology.statusUnhealthy")}</span>
      )}
      {selected.status === "warning" && (
        <span className="topology-detail__chip topology-detail__chip--warn">{t("topology.needsAttention")}</span>
      )}
      {(selected.meta ?? [])
        .filter(
          (item) =>
            item !== "leader" &&
            item !== "replica" &&
            item !== "standalone" &&
            !/^R\d+$/.test(item) &&
            !item.startsWith("leader ") &&
            !item.startsWith("meta leader "),
        )
        .map((item) => {
          const warn =
            item === "pending" ||
            item === "ack pending" ||
            item === "waiting" ||
            item === "redelivered" ||
            item.startsWith("lag ");
          return (
            <span key={item} className={`topology-detail__chip${warn ? " topology-detail__chip--warn" : ""}`}>
              {item}
            </span>
          );
        })}
    </div>
  );
}

function SignalStagePanel({
  selected,
  flowStream,
  onClose,
  onSelectNode,
}: {
  selected: TopologyNode;
  flowStream: TopologyNode;
  onClose: () => void;
  onSelectNode?: (node: TopologyNode) => void;
}) {
  const { t } = useTranslation();
  const { reduceMotion, panelVariants, transition, softSpring } = useTopologyMotion();
  const { subjects, consumers } = splitStreamChildren(flowStream);
  const raft =
    selected.raft ??
    (selected.kind === "consumer" || selected.kind === "subject" ? flowStream.raft : undefined);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  return (
    <motion.aside
      className="topology-inspector topology-inspector--stage topo-signal-panel"
      aria-label={t("topology.signalPanelAria", { name: selected.name })}
      variants={panelVariants}
      initial="hidden"
      animate="visible"
      exit="exit"
      transition={reduceMotion ? transition : softSpring}
    >
      <header className="topo-signal-panel__chrome">
        <div className="topo-signal-panel__identity">
          <p className="topology-detail__eyebrow">{kindEyebrow(selected.kind, t)}</p>
          <h2 className="topo-signal-panel__title">{selected.name}</h2>
          <SelectionMeta selected={selected} />
          {selected.kind !== "stream" && (
            <p className="topology-inspector__context">
              {t("topology.inspectorInStream", { stream: flowStream.name })}
            </p>
          )}
        </div>
        <div className="topo-signal-panel__actions">
          {selected.href && (
            <Link className="btn btn--secondary btn--small" to={selected.href} state={TOPOLOGY_LOCATION_STATE}>
              {selected.kind === "consumer" ? t("topology.openConsumer") : t("topology.openStream")}
            </Link>
          )}
          {!selected.href && flowStream.href && (
            <Link className="btn btn--secondary btn--small" to={flowStream.href} state={TOPOLOGY_LOCATION_STATE}>
              {t("topology.openStream")}
            </Link>
          )}
          <button className="btn btn--ghost btn--small" type="button" onClick={onClose}>
            {t("topology.clearSelection")}
          </button>
        </div>
      </header>

      <div className="topo-signal-panel__body">
        <div className="topo-signal-panel__stage">
          <Suspense fallback={<div className="skeleton skeleton--panel" aria-hidden />}>
            <TopologyFlowDiagram
              streams={[flowStream]}
              maxStreams={1}
              highlightNodeId={selected.id}
              variant="fullscreen"
              hideHeader
              onSelectNode={onSelectNode}
            />
          </Suspense>
        </div>

        <footer className="topo-signal-panel__dock">
          {raft && (raft.leader || (raft.clusterSize ?? 0) > 1 || (raft.peers?.length ?? 0) > 0) && (
            <div className="topo-signal-dock__band">
              <span className="topo-signal-dock__label">{t("topology.raftTitle")}</span>
              <div className="topo-signal-dock__chips">
                {raft.leader ? (
                  <span className="topology-detail__chip topology-detail__chip--leader">
                    {t("topology.raftLeader", { name: raft.leader })}
                  </span>
                ) : (
                  <span className="topology-detail__chip topology-detail__chip--danger">
                    {t("topology.raftNoLeader")}
                  </span>
                )}
                {(raft.clusterSize ?? 0) > 1 && (
                  <span className="topology-detail__chip">R{raft.clusterSize}</span>
                )}
                {raft.group && (
                  <span className="topology-detail__chip topology-detail__chip--mono">{raft.group}</span>
                )}
                {(raft.peers?.length ?? 0) === 0 && (
                  <span className="topo-signal-dock__muted">{t("topology.raftNoPeers")}</span>
                )}
              </div>
            </div>
          )}

          <div className="topo-signal-dock__lanes">
            <div className="topo-signal-dock__lane">
              <div className="topo-signal-dock__lane-head">
                <span className="topo-signal-dock__label">{t("topology.subjects")}</span>
                <span className="topology-detail__badge">{subjects.length}</span>
              </div>
              {subjects.length === 0 ? (
                <p className="topo-signal-dock__muted">{t("topology.signalDockNoSubjects")}</p>
              ) : (
                <ul className="topo-signal-dock__pills">
                  {subjects.map((subject) => {
                    const active = selected.id === subject.id;
                    return (
                      <li key={subject.id}>
                        <button
                          type="button"
                          className={`topo-signal-dock__pill topo-signal-dock__pill--subject${active ? " is-active" : ""}`}
                          onClick={() => onSelectNode?.(subject)}
                          aria-pressed={active}
                        >
                          <span className="topo-signal-dock__glyph" aria-hidden>
                            ◎
                          </span>
                          <code className="topo-signal-dock__code">{subject.name}</code>
                        </button>
                      </li>
                    );
                  })}
                </ul>
              )}
            </div>

            <div className="topo-signal-dock__lane">
              <div className="topo-signal-dock__lane-head">
                <span className="topo-signal-dock__label">{t("systems.consumers")}</span>
                <span className="topology-detail__badge">{consumers.length}</span>
              </div>
              {consumers.length === 0 ? (
                <p className="topo-signal-dock__muted">{t("topology.signalDockNoConsumers")}</p>
              ) : (
                <ul className="topo-signal-dock__pills">
                  {consumers.map((consumer) => {
                    const filter = consumer.meta?.find((item) => item.startsWith("filter "));
                    const active = selected.id === consumer.id;
                    return (
                      <li key={consumer.id}>
                        <button
                          type="button"
                          className={`topo-signal-dock__pill topo-signal-dock__pill--consumer${active ? " is-active" : ""}`}
                          onClick={() => onSelectNode?.(consumer)}
                          aria-pressed={active}
                        >
                          <span className="topo-signal-dock__glyph" aria-hidden>
                            ◉
                          </span>
                          <span className="topo-signal-dock__name">{consumer.name}</span>
                          {filter && <span className="topo-signal-dock__filter">{filter}</span>}
                        </button>
                      </li>
                    );
                  })}
                </ul>
              )}
            </div>
          </div>
        </footer>
      </div>
    </motion.aside>
  );
}

export default function TopologyInspector({
  selected,
  stream,
  onClose,
  onSelectNode,
}: TopologyInspectorProps) {
  const flowStream =
    selected && selected.kind !== "cluster"
      ? stream ?? (selected.kind === "stream" ? selected : null)
      : null;
  const showStage = Boolean(selected && flowStream);

  return (
    <AnimatePresence mode="wait">
      {showStage && selected && flowStream ? (
        <SignalStagePanel
          key={flowStream.id}
          selected={selected}
          flowStream={flowStream}
          onClose={onClose}
          onSelectNode={onSelectNode}
        />
      ) : null}
    </AnimatePresence>
  );
}
