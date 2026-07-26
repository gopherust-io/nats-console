import { type CSSProperties, type KeyboardEvent, type ReactNode } from "react";
import { Link } from "react-router";
import { motion } from "motion/react";
import { useTranslation } from "react-i18next";
import type { TopologyNode, TopologyNodeKind } from "../lib/topology";
import { splitStreamChildren, TOPOLOGY_LOCATION_STATE } from "../lib/topology";
import { useTopologyMotion } from "../lib/topologyMotion";

const kindLabels: Record<TopologyNodeKind, string> = {
  cluster: "Cluster",
  stream: "Stream",
  subject: "Subject",
  consumer: "Consumer",
};

const kindIcons: Record<TopologyNodeKind, string> = {
  cluster: "⬡",
  stream: "▤",
  subject: "◎",
  consumer: "◉",
};

type TopologyTreeProps = {
  root: TopologyNode;
  selectedNodeId?: string | null;
  onSelectNode?: (node: TopologyNode) => void;
};

function BranchRail({ visible }: { visible: boolean }) {
  const { reduceMotion, pathDraw } = useTopologyMotion();
  if (!visible) return null;

  return (
    <span className="topology-branch__rail" aria-hidden>
      <svg className="topology-branch__rail-svg" viewBox="0 0 16 36" preserveAspectRatio="none">
        <motion.path
          d="M 7 0 V 18 H 14"
          className="topology-branch__rail-path"
          fill="none"
          initial={{ pathLength: 0, opacity: 0 }}
          animate={{ pathLength: 1, opacity: 1 }}
          transition={reduceMotion ? { duration: 0 } : pathDraw}
        />
        <motion.circle
          cx="14"
          cy="18"
          r="2"
          className="topology-branch__rail-dot"
          initial={{ scale: 0, opacity: 0 }}
          animate={{ scale: 1, opacity: 1 }}
          transition={reduceMotion ? { duration: 0 } : { ...pathDraw, delay: 0.15 }}
        />
      </svg>
    </span>
  );
}

function BranchChildren({ className, children }: { className?: string; children: ReactNode }) {
  const { reduceMotion, collapseTransition } = useTopologyMotion();

  return (
    <motion.ul
      className={className}
      initial={reduceMotion ? false : { opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{
        ...collapseTransition,
        when: "beforeChildren",
        staggerChildren: reduceMotion ? 0 : 0.045,
        delayChildren: reduceMotion ? 0 : 0.04,
      }}
    >
      {children}
    </motion.ul>
  );
}

function NodeCard({
  node,
  selected,
  onSelect,
}: {
  node: TopologyNode;
  selected: boolean;
  onSelect: () => void;
}) {
  const { spring } = useTopologyMotion();
  const statusClass = node.status ? ` topology-node--${node.status}` : "";
  const selectedClass = selected ? " topology-node--selected" : "";
  const selectable = node.kind !== "cluster";

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (!selectable) return;
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onSelect();
    }
  };

  return (
    <motion.div
      className={`topology-node topology-node--chip${statusClass}${selectedClass}${selectable ? " topology-node--selectable" : ""}`}
      role={selectable ? "treeitem" : undefined}
      aria-selected={selectable ? selected : undefined}
      tabIndex={selectable ? 0 : undefined}
      onClick={selectable ? onSelect : undefined}
      onKeyDown={onKeyDown}
      layout
      transition={spring}
    >
      <span className={`topology-node__icon topology-node__icon--${node.kind}`} aria-hidden>
        {kindIcons[node.kind]}
      </span>
      <div className="topology-node__body">
        <div className="topology-node__head">
          <span className="topology-node__kind">{kindLabels[node.kind]}</span>
          {node.status && <span className={`topology-node__status topology-node__status--${node.status}`} />}
        </div>
        <div className="topology-node__name">
          {node.href ? (
            <Link
              to={node.href}
              state={TOPOLOGY_LOCATION_STATE}
              onClick={(event) => event.stopPropagation()}
              onKeyDown={(event) => event.stopPropagation()}
            >
              <motion.span layoutId={selectable ? `topo-name-${node.id}` : undefined}>{node.name}</motion.span>
            </Link>
          ) : (
            <motion.span layoutId={selectable ? `topo-name-${node.id}` : undefined}>{node.name}</motion.span>
          )}
        </div>
        {node.meta && node.meta.length > 0 && (
          <div className="topology-node__meta">
            {node.meta.map((item) => (
              <span key={item} className="topology-node__chip">
                {item}
              </span>
            ))}
          </div>
        )}
      </div>
    </motion.div>
  );
}

function GroupHeader({ label, count }: { label: string; count: number }) {
  return (
    <div className="topology-group__header">
      <span className="topology-group__label">{label}</span>
      <span className="topology-group__count">{count}</span>
    </div>
  );
}

function LeafBranch({
  node,
  depth,
  isLast,
  branchIndex = 0,
  selectedNodeId,
  onSelectNode,
}: {
  node: TopologyNode;
  depth: number;
  isLast: boolean;
  branchIndex?: number;
  selectedNodeId?: string | null;
  onSelectNode?: (node: TopologyNode) => void;
}) {
  const { itemVariants, transition } = useTopologyMotion();
  const branchStyle = { "--branch-index": branchIndex } as CSSProperties;

  return (
    <motion.li
      className={`topology-branch${isLast ? " topology-branch--last" : ""}`}
      data-depth={depth}
      style={branchStyle}
      variants={itemVariants}
      transition={transition}
    >
      <div className="topology-branch__row">
        <BranchRail visible={depth > 0} />
        <NodeCard
          node={node}
          selected={node.id === selectedNodeId}
          onSelect={() => onSelectNode?.(node)}
        />
      </div>
    </motion.li>
  );
}

function StreamGroups({
  stream,
  depth,
  selectedNodeId,
  onSelectNode,
}: {
  stream: TopologyNode;
  depth: number;
  selectedNodeId?: string | null;
  onSelectNode?: (node: TopologyNode) => void;
}) {
  const { itemVariants, transition } = useTopologyMotion();
  const { subjects, consumers } = splitStreamChildren(stream);
  const isSelected = stream.id === selectedNodeId;
  const hasChildren = subjects.length > 0 || consumers.length > 0;

  return (
    <motion.li
      className="topology-branch topology-branch--last"
      data-depth={depth}
      variants={itemVariants}
      transition={transition}
    >
      <div className="topology-branch__row">
        <BranchRail visible={depth > 0} />
        <NodeCard node={stream} selected={isSelected} onSelect={() => onSelectNode?.(stream)} />
      </div>
      {hasChildren && (
        <BranchChildren className="topology-branch__children is-open">
          {subjects.length > 0 && (
            <li className="topology-group">
              <GroupHeader label="Subjects" count={subjects.length} />
              <BranchChildren className="topology-group__items">
                {subjects.map((subject, index) => (
                  <LeafBranch
                    key={subject.id}
                    node={subject}
                    depth={depth + 2}
                    isLast={index === subjects.length - 1}
                    branchIndex={index}
                    selectedNodeId={selectedNodeId}
                    onSelectNode={onSelectNode}
                  />
                ))}
              </BranchChildren>
            </li>
          )}
          {consumers.length > 0 && (
            <li className="topology-group">
              <GroupHeader label="Consumers" count={consumers.length} />
              <BranchChildren className="topology-group__items">
                {consumers.map((consumer, index) => (
                  <LeafBranch
                    key={consumer.id}
                    node={consumer}
                    depth={depth + 2}
                    isLast={index === consumers.length - 1}
                    branchIndex={index}
                    selectedNodeId={selectedNodeId}
                    onSelectNode={onSelectNode}
                  />
                ))}
              </BranchChildren>
            </li>
          )}
        </BranchChildren>
      )}
    </motion.li>
  );
}

function TreeBranch({
  node,
  depth,
  isLast,
  selectedNodeId,
  onSelectNode,
  branchIndex = 0,
}: {
  node: TopologyNode;
  depth: number;
  isLast: boolean;
  selectedNodeId?: string | null;
  onSelectNode?: (node: TopologyNode) => void;
  branchIndex?: number;
}) {
  const { itemVariants, transition } = useTopologyMotion();

  if (node.kind === "stream") {
    return (
      <StreamGroups
        stream={node}
        depth={depth}
        selectedNodeId={selectedNodeId}
        onSelectNode={onSelectNode}
      />
    );
  }

  const hasChildren = node.children.length > 0;
  const branchStyle = { "--branch-index": branchIndex } as CSSProperties;

  return (
    <motion.li
      className={`topology-branch${isLast ? " topology-branch--last" : ""}`}
      data-depth={depth}
      style={branchStyle}
      variants={itemVariants}
      transition={transition}
    >
      <div className="topology-branch__row">
        <BranchRail visible={depth > 0} />
        <NodeCard
          node={node}
          selected={node.id === selectedNodeId}
          onSelect={() => onSelectNode?.(node)}
        />
      </div>
      {hasChildren && (
        <BranchChildren className="topology-branch__children is-open">
          {node.children.map((child, index) => (
            <TreeBranch
              key={child.id}
              node={child}
              depth={depth + 1}
              isLast={index === node.children.length - 1}
              selectedNodeId={selectedNodeId}
              onSelectNode={onSelectNode}
              branchIndex={index}
            />
          ))}
        </BranchChildren>
      )}
    </motion.li>
  );
}

export default function TopologyTree({
  root,
  selectedNodeId = null,
  onSelectNode,
}: TopologyTreeProps) {
  const { t } = useTranslation();
  const { listVariants } = useTopologyMotion();

  return (
    <div className="topology-tree topology-tree--constellation">
      <div className="topology-tree__head">
        <div>
          <h2 className="topology-tree__title">{t("topology.hierarchyTitle")}</h2>
          <p className="topology-tree__subtitle">{t("topology.hierarchySubtitle")}</p>
        </div>
      </div>

      <div className="topology-tree__canvas">
        <motion.ul
          className="topology-tree__root"
          role="tree"
          aria-label={t("topology.hierarchyTitle")}
          variants={listVariants}
          initial="hidden"
          animate="visible"
        >
          <TreeBranch
            node={root}
            depth={0}
            isLast
            selectedNodeId={selectedNodeId}
            onSelectNode={onSelectNode}
          />
        </motion.ul>
      </div>
    </div>
  );
}
