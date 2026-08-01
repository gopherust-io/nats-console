import { useId, useLayoutEffect, useMemo, useRef, useState } from "react";
import { motion } from "motion/react";
import { useTranslation } from "react-i18next";
import type { TopologyNode } from "../lib/topology";
import { TOPOLOGY_FLOW_VISIBLE, useTopologyMotion } from "../lib/topologyMotion";

export type TopologyFlowVariant = "panel" | "fullscreen";

type TopologyFlowDiagramProps = {
  root?: TopologyNode;
  streams?: TopologyNode[];
  maxStreams?: number;
  highlightNodeId?: string | null;
  variant?: TopologyFlowVariant;
  hideHeader?: boolean;
  onSelectNode?: (node: TopologyNode) => void;
};

type StageLayout = {
  viewW: number;
  viewH: number;
  hubX: number;
  hubY: number;
  leftX: number;
  rightX: number;
  nodeR: number;
  hubR: number;
  hubRingR: number;
  hubGlowR: number;
  labelMax: number;
};

const PANEL_LAYOUT: StageLayout = {
  viewW: 640,
  viewH: 360,
  hubX: 320,
  hubY: 180,
  leftX: 88,
  rightX: 552,
  nodeR: 18,
  hubR: 28,
  hubRingR: 34,
  hubGlowR: 72,
  labelMax: 16,
};

/** Match container pixels so meet fills with no letterbox and no stretch. */
function buildFullscreenLayout(viewW: number, viewH: number): StageLayout {
  const w = Math.max(Math.round(viewW), 320);
  const h = Math.max(Math.round(viewH), 200);
  const sideInset = Math.max(160, Math.min(w * 0.15, 280));
  const s = Math.max(0.72, Math.min(w / 1400, h / 820));
  const nodeR = Math.round(30 * s);
  const hubR = Math.round(56 * s);
  return {
    viewW: w,
    viewH: h,
    hubX: w / 2,
    hubY: h / 2,
    leftX: sideInset,
    rightX: w - sideInset,
    nodeR,
    hubR,
    hubRingR: hubR + 12,
    hubGlowR: Math.round(hubR * 2.5),
    labelMax: 28,
  };
}

function truncateLabel(name: string, max = 18): string {
  if (name.length <= max) return name;
  return `${name.slice(0, max - 1)}…`;
}

function laneYs(count: number, height: number, pad = 48): number[] {
  if (count <= 0) return [];
  if (count === 1) return [height / 2];
  const usable = height - pad * 2;
  return Array.from({ length: count }, (_, i) => pad + (usable * i) / (count - 1));
}

function curvePath(x1: number, y1: number, x2: number, y2: number): string {
  const mx = (x1 + x2) / 2;
  return `M ${x1} ${y1} C ${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2}`;
}

function SignalStage({
  stream,
  highlightNodeId,
  layout: baseLayout,
  fullscreen,
  onSelectNode,
}: {
  stream: TopologyNode;
  highlightNodeId?: string | null;
  layout: StageLayout;
  fullscreen: boolean;
  onSelectNode?: (node: TopologyNode) => void;
}) {
  const { t } = useTranslation();
  const uid = useId().replace(/:/g, "");
  const stageRef = useRef<HTMLDivElement>(null);
  const [stageBox, setStageBox] = useState({ w: 1400, h: 820 });
  const { reduceMotion, pathDraw, softSpring } = useTopologyMotion();

  useLayoutEffect(() => {
    if (!fullscreen) return;
    const el = stageRef.current;
    if (!el) return;
    const measure = () => {
      const { width, height } = el.getBoundingClientRect();
      if (width < 2 || height < 2) return;
      setStageBox((prev) =>
        Math.abs(prev.w - width) < 1 && Math.abs(prev.h - height) < 1
          ? prev
          : { w: width, h: height },
      );
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [fullscreen]);

  const layout = useMemo(
    () => (fullscreen ? buildFullscreenLayout(stageBox.w, stageBox.h) : baseLayout),
    [fullscreen, stageBox.w, stageBox.h, baseLayout],
  );
  const { viewW, viewH, hubX, hubY, leftX, rightX, nodeR, hubR, hubRingR, hubGlowR, labelMax } = layout;
  const lanePad = fullscreen ? Math.max(64, Math.round(viewH * 0.12)) : 48;

  const subjectsAll = stream.children.filter((node) => node.kind === "subject");
  const consumersAll = stream.children.filter((node) => node.kind === "consumer");
  const subjects = subjectsAll.slice(0, TOPOLOGY_FLOW_VISIBLE);
  const consumers = consumersAll.slice(0, TOPOLOGY_FLOW_VISIBLE);
  const subjectOverflow = subjectsAll.length - subjects.length;
  const consumerOverflow = consumersAll.length - consumers.length;

  const subjectYs = laneYs(Math.max(subjects.length, 1), viewH, lanePad);
  const consumerYs = laneYs(Math.max(consumers.length, 1), viewH, lanePad);

  const subjectPaths = subjects.map((subject, index) => {
    const y = subjectYs[index] ?? hubY;
    const d = curvePath(leftX + nodeR + 8, y, hubX - hubRingR - 2, hubY);
    return { id: `${uid}-s-${index}`, node: subject, y, d };
  });

  const consumerPaths = consumers.map((consumer, index) => {
    const y = consumerYs[index] ?? hubY;
    const d = curvePath(hubX + hubRingR + 2, hubY, rightX - nodeR - 8, y);
    return { id: `${uid}-c-${index}`, node: consumer, y, d };
  });

  const emptySubject = subjects.length === 0;
  const emptyConsumer = consumers.length === 0;
  const hubStatus = stream.status ?? "healthy";
  const raftCaption =
    stream.raft && (stream.raft.clusterSize ?? 0) > 1
      ? [
          stream.role === "leader"
            ? t("topology.roleLeader")
            : stream.role === "replica"
              ? t("topology.roleReplica")
              : null,
          stream.raft.leader || t("topology.raftNoLeader"),
          `R${stream.raft.clusterSize}`,
        ]
          .filter(Boolean)
          .join(" · ")
      : null;

  return (
    <div
      ref={stageRef}
      className={`topo-signal-stage${fullscreen ? " topo-signal-stage--fullscreen" : ""}`}
    >
      <svg
        className="topo-signal-stage__svg"
        viewBox={`0 0 ${viewW} ${viewH}`}
        preserveAspectRatio="xMidYMid meet"
        role="img"
        aria-label={t("topology.signalStageAria", { stream: stream.name })}
      >
        <defs>
          <radialGradient id={`${uid}-hub-glow`} cx="50%" cy="50%" r="50%">
            <stop offset="0%" stopColor="var(--accent)" stopOpacity="0.35" />
            <stop offset="100%" stopColor="var(--accent)" stopOpacity="0" />
          </radialGradient>
          <linearGradient id={`${uid}-lane-left`} x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="var(--success)" stopOpacity="0.12" />
            <stop offset="100%" stopColor="var(--success)" stopOpacity="0" />
          </linearGradient>
          <linearGradient id={`${uid}-lane-right`} x1="100%" y1="0%" x2="0%" y2="0%">
            <stop offset="0%" stopColor="var(--accent)" stopOpacity="0.1" />
            <stop offset="100%" stopColor="var(--accent)" stopOpacity="0" />
          </linearGradient>
          <filter id={`${uid}-soft`} x="-40%" y="-40%" width="180%" height="180%">
            <feGaussianBlur stdDeviation="2.5" result="b" />
            <feMerge>
              <feMergeNode in="b" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        {/* Lane washes only in panel — they read as side frames under fullscreen letterbox/stretch. */}
        {!fullscreen && (
          <>
            <rect x={0} y={0} width={viewW * 0.28} height={viewH} fill={`url(#${uid}-lane-left)`} aria-hidden />
            <rect
              x={viewW * 0.72}
              y={0}
              width={viewW * 0.28}
              height={viewH}
              fill={`url(#${uid}-lane-right)`}
              aria-hidden
            />
          </>
        )}

        <circle cx={hubX} cy={hubY} r={hubGlowR} fill={`url(#${uid}-hub-glow)`} aria-hidden />

        {emptySubject && (
          <motion.path
            d={curvePath(leftX + 24, hubY, hubX - hubRingR - 2, hubY)}
            className="topo-signal-path topo-signal-path--ghost"
            initial={{ pathLength: 0, opacity: 0 }}
            animate={{ pathLength: 1, opacity: 0.35 }}
            transition={pathDraw}
            fill="none"
          />
        )}
        {emptyConsumer && (
          <motion.path
            d={curvePath(hubX + hubRingR + 2, hubY, rightX - 24, hubY)}
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
          </g>
        ))}

        {consumerPaths.map((edge, index) => {
          const hasAckPending = Boolean(edge.node.meta?.includes("ack pending"));
          return (
            <g key={edge.id}>
              <motion.path
                id={edge.id}
                d={edge.d}
                className={`topo-signal-path${highlightNodeId === edge.node.id ? " topo-signal-path--hot" : ""}${hasAckPending ? " topo-signal-path--nack" : ""}`}
                initial={{ pathLength: 0, opacity: 0 }}
                animate={{ pathLength: 1, opacity: 1 }}
                transition={{ ...pathDraw, delay: reduceMotion ? 0 : 0.12 + index * 0.06 }}
                fill="none"
              />
            </g>
          );
        })}

        {(emptySubject
          ? [{ id: "empty-s", name: "—", y: hubY, hot: false, node: null as TopologyNode | null }]
          : subjectPaths.map((p) => ({
              id: p.node.id,
              name: p.node.name,
              y: p.y,
              hot: highlightNodeId === p.node.id,
              node: p.node,
            }))
        ).map((node, index) => (
          <motion.g
            key={node.id}
            className={node.node && onSelectNode ? "topo-signal-hit" : undefined}
            initial={reduceMotion ? false : { opacity: 0, scale: 0.85 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ ...softSpring, delay: reduceMotion ? 0 : index * 0.04 }}
            onClick={node.node && onSelectNode ? () => onSelectNode(node.node!) : undefined}
            style={node.node && onSelectNode ? { cursor: "pointer" } : undefined}
          >
            <circle
              cx={leftX}
              cy={node.y}
              r={nodeR + (node.hot ? 4 : 0)}
              className={`topo-signal-node topo-signal-node--subject${node.hot ? " is-hot" : ""}`}
              filter={node.hot ? `url(#${uid}-soft)` : undefined}
            />
            <text x={leftX} y={node.y + 1} className="topo-signal-node__glyph" textAnchor="middle" dominantBaseline="middle">
              ◎
            </text>
            <text x={leftX} y={node.y + nodeR + 18} className="topo-signal-label" textAnchor="middle">
              {truncateLabel(node.name, labelMax)}
            </text>
            {node.node && onSelectNode && (
              <circle cx={leftX} cy={node.y} r={nodeR + 10} fill="transparent">
                <title>{node.name}</title>
              </circle>
            )}
          </motion.g>
        ))}

        <motion.g
          className={onSelectNode ? "topo-signal-hit" : undefined}
          initial={reduceMotion ? false : { opacity: 0, scale: 0.8 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={softSpring}
          onClick={onSelectNode ? () => onSelectNode(stream) : undefined}
          style={onSelectNode ? { cursor: "pointer" } : undefined}
        >
          <motion.circle
            cx={hubX}
            cy={hubY}
            r={hubRingR}
            className={`topo-signal-hub-ring topo-signal-hub-ring--${hubStatus}`}
            initial={reduceMotion ? false : { scale: 0.96 }}
            animate={{ scale: 1 }}
            transition={reduceMotion ? undefined : { duration: 0.45, ease: "easeOut" }}
            style={{ transformOrigin: `${hubX}px ${hubY}px` }}
          />
          <circle
            cx={hubX}
            cy={hubY}
            r={hubR}
            className={`topo-signal-hub topo-signal-hub--${hubStatus}${highlightNodeId === stream.id ? " is-hot" : ""}`}
          />
          <text x={hubX} y={hubY - 2} className="topo-signal-hub__label" textAnchor="middle" dominantBaseline="middle">
            JS
          </text>
          <text x={hubX} y={hubY + hubR + 22} className="topo-signal-label topo-signal-label--hub" textAnchor="middle">
            {truncateLabel(stream.name, labelMax + 4)}
          </text>
          {raftCaption && (
            <text x={hubX} y={hubY + hubR + 40} className="topo-signal-label topo-signal-label--raft" textAnchor="middle">
              {raftCaption}
            </text>
          )}
          {onSelectNode && (
            <circle cx={hubX} cy={hubY} r={hubRingR} fill="transparent" className="topo-signal-hub-link">
              <title>{stream.name}</title>
            </circle>
          )}
        </motion.g>

        {(emptyConsumer
          ? [
              {
                id: "empty-c",
                name: "—",
                y: hubY,
                hot: false,
                status: undefined as TopologyNode["status"],
                node: null as TopologyNode | null,
              },
            ]
          : consumerPaths.map((p) => {
              const metaWarn = Boolean(
                p.node.meta?.some(
                  (item) =>
                    item === "pending" ||
                    item === "ack pending" ||
                    item === "waiting" ||
                    item === "redelivered" ||
                    item.startsWith("lag "),
                ),
              );
              const status =
                p.node.status === "unhealthy"
                  ? "unhealthy"
                  : p.node.status === "warning" || metaWarn
                    ? "warning"
                    : p.node.status;
              return {
                id: p.node.id,
                name: p.node.name,
                y: p.y,
                hot: highlightNodeId === p.node.id,
                status,
                node: p.node,
              };
            })
        ).map((node, index) => (
          <motion.g
            key={node.id}
            className={node.node && onSelectNode ? "topo-signal-hit" : undefined}
            initial={reduceMotion ? false : { opacity: 0, scale: 0.85 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ ...softSpring, delay: reduceMotion ? 0 : 0.08 + index * 0.04 }}
            onClick={node.node && onSelectNode ? () => onSelectNode(node.node!) : undefined}
            style={node.node && onSelectNode ? { cursor: "pointer" } : undefined}
          >
            <circle
              cx={rightX}
              cy={node.y}
              r={nodeR + (node.hot ? 4 : 0)}
              className={`topo-signal-node topo-signal-node--consumer${node.status ? ` topo-signal-node--${node.status}` : ""}${node.hot ? " is-hot" : ""}`}
              filter={node.hot ? `url(#${uid}-soft)` : undefined}
            />
            <text x={rightX} y={node.y + 1} className="topo-signal-node__glyph" textAnchor="middle" dominantBaseline="middle">
              ◉
            </text>
            <text x={rightX} y={node.y + nodeR + 18} className="topo-signal-label" textAnchor="middle">
              {truncateLabel(node.name, labelMax)}
            </text>
            {node.node && onSelectNode && (
              <circle cx={rightX} cy={node.y} r={nodeR + 10} fill="transparent">
                <title>{node.name}</title>
              </circle>
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
  variant = "panel",
  hideHeader = false,
  onSelectNode,
}: TopologyFlowDiagramProps) {
  const { t } = useTranslation();
  const fullscreen = variant === "fullscreen";
  const layout = PANEL_LAYOUT;
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
    <section
      className={`topo-flow-section topo-flow-section--signal${fullscreen ? " topo-flow-section--fullscreen" : ""}`}
    >
      {!hideHeader && (
        <div className="topo-flow-section__head">
          <h2 className="topo-flow-section__title">{t("topology.signalTitle")}</h2>
          <p className="topo-flow-section__desc">{t("topology.signalDesc")}</p>
        </div>
      )}
      {visibleStreams.map((stream) => (
        <SignalStage
          key={stream.id}
          stream={stream}
          highlightNodeId={highlightNodeId}
          layout={layout}
          fullscreen={fullscreen}
          onSelectNode={onSelectNode}
        />
      ))}
    </section>
  );
}
