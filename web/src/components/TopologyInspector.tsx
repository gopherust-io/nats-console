import { Link } from "react-router";
import { AnimatePresence, motion } from "motion/react";
import { useTranslation } from "react-i18next";
import TopologyFlowDiagram from "./TopologyFlowDiagram";
import type { TopologyNode } from "../lib/topology";
import { getStreamNodes, splitStreamChildren, TOPOLOGY_LOCATION_STATE } from "../lib/topology";
import { useTopologyMotion } from "../lib/topologyMotion";

type TopologyInspectorProps = {
  selected: TopologyNode | null;
  stream: TopologyNode | null;
  streams?: TopologyNode[];
  root?: TopologyNode | null;
  onClose: () => void;
  onSelectStream?: (stream: TopologyNode) => void;
};

function SubjectList({ subjects }: { subjects: TopologyNode[] }) {
  if (subjects.length === 0) {
    return <p className="topology-detail__empty">No subject patterns configured.</p>;
  }

  return (
    <ul className="topology-detail__list">
      {subjects.map((subject) => (
        <li key={subject.id} className="topology-detail__list-item">
          <span className="topology-detail__list-icon" aria-hidden>
            ◎
          </span>
          <code className="topology-detail__pattern">{subject.name}</code>
        </li>
      ))}
    </ul>
  );
}

function ConsumerList({ consumers }: { consumers: TopologyNode[] }) {
  if (consumers.length === 0) {
    return <p className="topology-detail__empty">No consumers attached.</p>;
  }

  return (
    <ul className="topology-detail__list">
      {consumers.map((consumer) => {
        const filter = consumer.meta?.find((item) => item.startsWith("Filter "));
        const pending = consumer.meta?.find((item) => item.endsWith(" pending"));
        return (
          <li key={consumer.id} className="topology-detail__list-item">
            <span className="topology-detail__list-icon" aria-hidden>
              ◉
            </span>
            <div className="topology-detail__consumer">
              {consumer.href ? (
                <Link to={consumer.href} state={TOPOLOGY_LOCATION_STATE} className="topology-detail__consumer-name">
                  {consumer.name}
                </Link>
              ) : (
                <span className="topology-detail__consumer-name">{consumer.name}</span>
              )}
              <div className="topology-detail__consumer-meta">
                {filter && <span className="topology-detail__chip">{filter}</span>}
                {pending && <span className="topology-detail__chip topology-detail__chip--warn">{pending}</span>}
                {consumer.status === "warning" && !pending && (
                  <span className="topology-detail__chip topology-detail__chip--warn">Needs attention</span>
                )}
              </div>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

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

function IdleConstellation({
  streams,
  onSelectStream,
}: {
  streams: TopologyNode[];
  onSelectStream?: (stream: TopologyNode) => void;
}) {
  const { t } = useTranslation();
  const { reduceMotion, orbit } = useTopologyMotion();
  const visible = streams.slice(0, 8);
  const cx = 140;
  const cy = 110;
  const radius = 62;

  return (
    <div className="topo-idle-constellation">
      <h2 className="topology-inspector__empty-title">{t("topology.inspectorEmptyTitle")}</h2>
      <p className="topology-inspector__empty-desc">{t("topology.idleConstellationDesc")}</p>
      <svg
        className="topo-idle-constellation__svg"
        viewBox="0 0 280 220"
        role="img"
        aria-label={t("topology.idleConstellationAria")}
      >
        <circle cx={cx} cy={cy} r={radius} className="topo-idle-constellation__orbit" />
        <circle cx={cx} cy={cy} r="10" className="topo-idle-constellation__core" />
        <motion.g
          animate={reduceMotion ? undefined : { rotate: 360 }}
          transition={orbit}
          style={{ transformOrigin: `${cx}px ${cy}px` }}
        >
          {visible.map((stream, index) => {
            const angle = (Math.PI * 2 * index) / Math.max(visible.length, 1) - Math.PI / 2;
            const x = cx + Math.cos(angle) * radius;
            const y = cy + Math.sin(angle) * radius;
            return (
              <g key={stream.id}>
                <circle
                  cx={x}
                  cy={y}
                  r="11"
                  className="topo-idle-constellation__star"
                  role="button"
                  tabIndex={0}
                  onClick={() => onSelectStream?.(stream)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      onSelectStream?.(stream);
                    }
                  }}
                >
                  <title>{stream.name}</title>
                </circle>
              </g>
            );
          })}
        </motion.g>
      </svg>
      {streams.length > visible.length && (
        <p className="topo-idle-constellation__note">
          {t("topology.idleConstellationMore", { count: streams.length - visible.length })}
        </p>
      )}
    </div>
  );
}

export default function TopologyInspector({
  selected,
  stream,
  streams: streamsProp,
  root,
  onClose,
  onSelectStream,
}: TopologyInspectorProps) {
  const { t } = useTranslation();
  const { panelVariants, transition } = useTopologyMotion();
  const flowStream = stream ?? (selected?.kind === "stream" ? selected : null);
  const { subjects, consumers } = flowStream ? splitStreamChildren(flowStream) : { subjects: [], consumers: [] };
  const streams = streamsProp ?? (root ? getStreamNodes(root) : []);

  return (
    <aside className="topology-inspector topology-inspector--stage panel" aria-live="polite">
      <AnimatePresence mode="wait" initial={false}>
        {!selected ? (
          <motion.div
            key="empty"
            className="topology-inspector__empty"
            variants={panelVariants}
            initial="hidden"
            animate="visible"
            exit="exit"
            transition={transition}
          >
            <IdleConstellation streams={streams} onSelectStream={onSelectStream} />
          </motion.div>
        ) : (
          <motion.div
            key={selected.id}
            className="topology-inspector__body"
            variants={panelVariants}
            initial="hidden"
            animate="visible"
            exit="exit"
            transition={transition}
          >
            <div className="topology-detail__head">
              <div>
                <p className="topology-detail__eyebrow">{kindEyebrow(selected.kind, t)}</p>
                <motion.h2 className="panel__title topology-inspector__title" layoutId={`topo-name-${selected.id}`}>
                  {selected.name}
                </motion.h2>
                {selected.meta && selected.meta.length > 0 && (
                  <div className="topology-detail__meta">
                    {selected.meta.map((item) => (
                      <span key={item} className="topology-detail__chip">
                        {item}
                      </span>
                    ))}
                  </div>
                )}
                {selected.kind !== "stream" && flowStream && (
                  <p className="topology-inspector__context">
                    {t("topology.inspectorInStream", { stream: flowStream.name })}
                  </p>
                )}
              </div>
              <div className="topology-detail__actions">
                {selected.href && (
                  <Link className="btn btn--secondary" to={selected.href} state={TOPOLOGY_LOCATION_STATE}>
                    {selected.kind === "consumer" ? t("topology.openConsumer") : t("topology.openStream")}
                  </Link>
                )}
                {!selected.href && flowStream?.href && (
                  <Link className="btn btn--secondary" to={flowStream.href} state={TOPOLOGY_LOCATION_STATE}>
                    {t("topology.openStream")}
                  </Link>
                )}
                <button className="btn btn--ghost" type="button" onClick={onClose}>
                  {t("topology.clearSelection")}
                </button>
              </div>
            </div>

            {flowStream && (
              <TopologyFlowDiagram streams={[flowStream]} maxStreams={1} highlightNodeId={selected.id} />
            )}

            {selected.kind === "stream" && (
              <div className="topology-detail__columns">
                <div className="topology-detail__column">
                  <h3 className="topology-detail__column-title">
                    {t("topology.subjects")} <span className="topology-detail__badge">{subjects.length}</span>
                  </h3>
                  <p className="topology-detail__column-desc">{t("topology.subjectsDesc")}</p>
                  <SubjectList subjects={subjects} />
                </div>
                <div className="topology-detail__column">
                  <h3 className="topology-detail__column-title">
                    {t("systems.consumers")} <span className="topology-detail__badge">{consumers.length}</span>
                  </h3>
                  <p className="topology-detail__column-desc">{t("topology.consumersDesc")}</p>
                  <ConsumerList consumers={consumers} />
                </div>
              </div>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </aside>
  );
}
