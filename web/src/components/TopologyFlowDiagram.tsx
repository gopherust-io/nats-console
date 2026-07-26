import { useId, useMemo } from "react";
import { Link } from "react-router";
import { motion } from "motion/react";
import { useTranslation } from "react-i18next";
import type { TopologyNode } from "../lib/topology";
import { TOPOLOGY_FLOW_VISIBLE, useTopologyMotion } from "../lib/topologyMotion";
import { TOPOLOGY_LOCATION_STATE } from "../lib/topology";

type TopologyFlowDiagramProps = {
  root?: TopologyNode;
  streams?: TopologyNode[];
  maxStreams?: number;
  highlightNodeId?: string | null;
};

const VIEW_W = 640;
const VIEW_H = 360;
const HUB_X = 320;
const HUB_Y = 180;
const LEFT_X = 88;
const RIGHT_X = 552;
const NODE_R = 18;

function truncateLabel(name: string, max = 18): string {
  if (name.length <= max) return name;
  return `${name.slice(0, max - 1)}…`;
}

function laneYs(count: number, height = VIEW_H, pad = 48): number[] {
  if (count <= 0) return [];
  if (count === 1) return [height / 2];
  const usable = height - pad * 2;
  return Array.from({ length: count }, (_, i) => pad + (usable * i) / (count - 1));
}

function curvePath(x1: number, y1: number, x2: number, y2: number): string {
  const mx = (x1 + x2) / 2;
  return `M ${x1} ${y1} C ${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2}`;
}

function SignalDot({
  pathId,
  delay,
  duration,
  reduceMotion,
}: {
  pathId: string;
  delay: number;
  duration: number;
  reduceMotion: boolean;
}) {
  if (reduceMotion) return null;

  return (
    <circle r="3.5" className="topo-signal-dot" aria-hidden>
      <animateMotion dur={`${duration}s`} begin={`${delay}s`} repeatCount="indefinite" rotate="auto">
        <mpath href={`#${pathId}`} />
      </animateMotion>
    </circle>
  );
}

function SignalStage({
  stream,
  highlightNodeId,
}: {
  stream: TopologyNode;
  highlightNodeId?: string | null;
}) {
  const { t } = useTranslation();
  const uid = useId().replace(/:/g, "");
  const { reduceMotion, pathDraw, softSpring } = useTopologyMotion();

  const subjectsAll = stream.children.filter((node) => node.kind === "subject");
  const consumersAll = stream.children.filter((node) => node.kind === "consumer");
  const subjects = subjectsAll.slice(0, TOPOLOGY_FLOW_VISIBLE);
  const consumers = consumersAll.slice(0, TOPOLOGY_FLOW_VISIBLE);
  const subjectOverflow = subjectsAll.length - subjects.length;
  const consumerOverflow = consumersAll.length - consumers.length;

  const subjectYs = laneYs(Math.max(subjects.length, 1));
  const consumerYs = laneYs(Math.max(consumers.length, 1));

  const subjectPaths = subjects.map((subject, index) => {
    const y = subjectYs[index] ?? HUB_Y;
    const d = curvePath(LEFT_X + NODE_R + 8, y, HUB_X - 36, HUB_Y);
    return { id: `${uid}-s-${index}`, node: subject, y, d };
  });

  const consumerPaths = consumers.map((consumer, index) => {
    const y = consumerYs[index] ?? HUB_Y;
    const d = curvePath(HUB_X + 36, HUB_Y, RIGHT_X - NODE_R - 8, y);
    return { id: `${uid}-c-${index}`, node: consumer, y, d };
  });

  const emptySubject = subjects.length === 0;
  const emptyConsumer = consumers.length === 0;

  return (
    <div className="topo-signal-stage">
      <svg
        className="topo-signal-stage__svg"
        viewBox={`0 0 ${VIEW_W} ${VIEW_H}`}
        role="img"
        aria-label={t("topology.signalStageAria", { stream: stream.name })}
      >
        <defs>
          <radialGradient id={`${uid}-hub-glow`} cx="50%" cy="50%" r="50%">
            <stop offset="0%" stopColor="var(--accent)" stopOpacity="0.35" />
            <stop offset="100%" stopColor="var(--accent)" stopOpacity="0" />
          </radialGradient>
          <filter id={`${uid}-soft`} x="-40%" y="-40%" width="180%" height="180%">
            <feGaussianBlur stdDeviation="2.5" result="b" />
            <feMerge>
              <feMergeNode in="b" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        <circle cx={HUB_X} cy={HUB_Y} r="72" fill={`url(#${uid}-hub-glow)`} aria-hidden />

        {emptySubject && (
          <motion.path
            d={curvePath(LEFT_X + 24, HUB_Y, HUB_X - 36, HUB_Y)}
            className="topo-signal-path topo-signal-path--ghost"
            initial={{ pathLength: 0, opacity: 0 }}
            animate={{ pathLength: 1, opacity: 0.35 }}
            transition={pathDraw}
            fill="none"
          />
        )}
        {emptyConsumer && (
          <motion.path
            d={curvePath(HUB_X + 36, HUB_Y, RIGHT_X - 24, HUB_Y)}
            className="topo-signal-path topo-signal-path--ghost"
            initial={{ pathLength: 0, opacity: 0 }}
            animate={{ pathLength: 1, opacity: 0.35 }}
            transition={pathDraw}
            fill="none"
          />
        )}

        {subjectPaths.map((edge, index) => (
          <g key={edge.id}>
            <motion.path
              id={edge.id}
              d={edge.d}
              className={`topo-signal-path${highlightNodeId === edge.node.id ? " topo-signal-path--hot" : ""}`}
              initial={{ pathLength: 0, opacity: 0 }}
              animate={{ pathLength: 1, opacity: 1 }}
              transition={{ ...pathDraw, delay: reduceMotion ? 0 : index * 0.06 }}
              fill="none"
            />
            <SignalDot pathId={edge.id} delay={index * 0.35} duration={2.2 + index * 0.15} reduceMotion={reduceMotion} />
          </g>
        ))}

        {consumerPaths.map((edge, index) => (
          <g key={edge.id}>
            <motion.path
              id={edge.id}
              d={edge.d}
              className={`topo-signal-path${highlightNodeId === edge.node.id ? " topo-signal-path--hot" : ""}`}
              initial={{ pathLength: 0, opacity: 0 }}
              animate={{ pathLength: 1, opacity: 1 }}
              transition={{ ...pathDraw, delay: reduceMotion ? 0 : 0.12 + index * 0.06 }}
              fill="none"
            />
            <SignalDot
              pathId={edge.id}
              delay={0.4 + index * 0.35}
              duration={2.3 + index * 0.12}
              reduceMotion={reduceMotion}
            />
          </g>
        ))}

        {/* Subjects */}
        {(emptySubject ? [{ id: "empty-s", name: "—", y: HUB_Y, hot: false }] : subjectPaths.map((p) => ({
          id: p.node.id,
          name: p.node.name,
          y: p.y,
          hot: highlightNodeId === p.node.id,
        }))).map((node, index) => (
          <motion.g
            key={node.id}
            initial={reduceMotion ? false : { opacity: 0, scale: 0.85 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ ...softSpring, delay: reduceMotion ? 0 : index * 0.04 }}
          >
            <circle
              cx={LEFT_X}
              cy={node.y}
              r={NODE_R + (node.hot ? 4 : 0)}
              className={`topo-signal-node topo-signal-node--subject${node.hot ? " is-hot" : ""}`}
              filter={node.hot ? `url(#${uid}-soft)` : undefined}
            />
            <text x={LEFT_X} y={node.y + 1} className="topo-signal-node__glyph" textAnchor="middle" dominantBaseline="middle">
              ◎
            </text>
            <text x={LEFT_X} y={node.y + NODE_R + 16} className="topo-signal-label" textAnchor="middle">
              {truncateLabel(node.name, 16)}
            </text>
          </motion.g>
        ))}

        {/* Hub */}
        <motion.g
          initial={reduceMotion ? false : { opacity: 0, scale: 0.8 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={softSpring}
        >
          <motion.circle
            cx={HUB_X}
            cy={HUB_Y}
            r="34"
            className="topo-signal-hub-ring"
            animate={reduceMotion ? undefined : { scale: [1, 1.06, 1] }}
            transition={reduceMotion ? undefined : { duration: 3.2, repeat: Infinity, ease: "easeInOut" }}
            style={{ transformOrigin: `${HUB_X}px ${HUB_Y}px` }}
          />
          <circle
            cx={HUB_X}
            cy={HUB_Y}
            r="28"
            className={`topo-signal-hub${highlightNodeId === stream.id ? " is-hot" : ""}`}
          />
          <text x={HUB_X} y={HUB_Y - 2} className="topo-signal-hub__label" textAnchor="middle" dominantBaseline="middle">
            JS
          </text>
          <text x={HUB_X} y={HUB_Y + 48} className="topo-signal-label topo-signal-label--hub" textAnchor="middle">
            {truncateLabel(stream.name, 22)}
          </text>
          {stream.href && (
            <Link to={stream.href} state={TOPOLOGY_LOCATION_STATE} className="topo-signal-hub-link">
              <title>{stream.name}</title>
              <circle cx={HUB_X} cy={HUB_Y} r="34" fill="transparent" />
            </Link>
          )}
        </motion.g>

        {/* Consumers */}
        {(emptyConsumer ? [{ id: "empty-c", name: "—", y: HUB_Y, hot: false, href: undefined as string | undefined }] : consumerPaths.map((p) => ({
          id: p.node.id,
          name: p.node.name,
          y: p.y,
          hot: highlightNodeId === p.node.id,
          href: p.node.href,
        }))).map((node, index) => (
          <motion.g
            key={node.id}
            initial={reduceMotion ? false : { opacity: 0, scale: 0.85 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ ...softSpring, delay: reduceMotion ? 0 : 0.08 + index * 0.04 }}
          >
            <circle
              cx={RIGHT_X}
              cy={node.y}
              r={NODE_R + (node.hot ? 4 : 0)}
              className={`topo-signal-node topo-signal-node--consumer${node.hot ? " is-hot" : ""}`}
              filter={node.hot ? `url(#${uid}-soft)` : undefined}
            />
            <text x={RIGHT_X} y={node.y + 1} className="topo-signal-node__glyph" textAnchor="middle" dominantBaseline="middle">
              ◉
            </text>
            <text x={RIGHT_X} y={node.y + NODE_R + 16} className="topo-signal-label" textAnchor="middle">
              {truncateLabel(node.name, 16)}
            </text>
            {node.href && (
              <Link to={node.href} state={TOPOLOGY_LOCATION_STATE}>
                <title>{node.name}</title>
                <circle cx={RIGHT_X} cy={node.y} r={NODE_R + 6} fill="transparent" />
              </Link>
            )}
          </motion.g>
        ))}
      </svg>

      {(subjectOverflow > 0 || consumerOverflow > 0) && (
        <p className="topo-signal-stage__more">
          {subjectOverflow > 0 && consumerOverflow > 0
            ? t("topology.signalMoreBoth", { subjects: subjectOverflow, consumers: consumerOverflow })
            : subjectOverflow > 0
              ? t("topology.signalMoreSubjects", { count: subjectOverflow })
              : t("topology.signalMoreConsumers", { count: consumerOverflow })}
        </p>
      )}
    </div>
  );
}

export default function TopologyFlowDiagram({
  root,
  streams: streamsProp,
  maxStreams = 1,
  highlightNodeId = null,
}: TopologyFlowDiagramProps) {
  const { t } = useTranslation();
  const streams = useMemo(() => {
    if (streamsProp) return streamsProp;
    if (root) return root.children.filter((node) => node.kind === "stream");
    return [];
  }, [root, streamsProp]);

  const visibleStreams = streams.slice(0, maxStreams);

  if (streams.length === 0) {
    return null;
  }

  return (
    <section className="topo-flow-section topo-flow-section--signal">
      <div className="topo-flow-section__head">
        <h2 className="topo-flow-section__title">{t("topology.signalTitle")}</h2>
        <p className="topo-flow-section__desc">{t("topology.signalDesc")}</p>
      </div>
      {visibleStreams.map((stream) => (
        <SignalStage key={stream.id} stream={stream} highlightNodeId={highlightNodeId} />
      ))}
    </section>
  );
}
