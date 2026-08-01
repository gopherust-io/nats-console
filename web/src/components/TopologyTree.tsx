import {
  memo,
  useCallback,
  useState,
  type CSSProperties,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import { Link } from "react-router";
import { AnimatePresence, motion } from "motion/react";
import { useTranslation } from "react-i18next";
import type { TopologyNode, TopologyNodeKind } from "../lib/topology";
import { splitStreamChildren, TOPOLOGY_LOCATION_STATE } from "../lib/topology";
import { useTopologyMotion } from "../lib/topologyMotion";

const kindLabelKeys: Record<TopologyNodeKind, string> = {
  cluster: "topology.kindCluster",
  stream: "topology.kindStream",
  subject: "topology.kindSubject",
  consumer: "topology.kindConsumer",
};

const kindIconPaths: Record<Exclude<TopologyNodeKind, "subject">, string> = {
  cluster: "M12 3l8 4.5v9L12 21l-8-4.5v-9L12 3zm0 2.2L6.5 8.5 12 11.8l5.5-3.3L12 5.2z",
  stream: "M4 7h16M4 12h12M4 17h14",
  consumer: "M12 4a3 3 0 110 6 3 3 0 010-6zM5.5 20v-1a4.5 4.5 0 019 0v1",
};

type TopologyTreeProps = {
  root: TopologyNode;
  selectedNodeId?: string | null;
  onSelectNode?: (node: TopologyNode) => void;
  /** When false, skip Motion layout/layoutId work (filter typing / refetch). */
  layoutEnabled?: boolean;
};

type ExpandApi = {
  isExpanded: (id: string) => boolean;
  toggle: (id: string) => void;
};

function KindIcon({ kind }: { kind: TopologyNodeKind }) {
  if (kind === "subject") {
    return (
      <svg
        className="topology-node__glyph"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth={1.75}
        aria-hidden
      >
        <circle cx="12" cy="12" r="7" />
        <circle cx="12" cy="12" r="2.25" fill="currentColor" stroke="none" />
      </svg>
    );
  }

  return (
    <svg
      className="topology-node__glyph"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      aria-hidden
    >
      <path d={kindIconPaths[kind]} strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function BranchRail({ visible }: { visible: boolean }) {
  if (!visible) return null;
  return <span className="topology-branch__rail" aria-hidden />;
}

function BranchChildren({
  className,
  open,
  children,
}: {
  className?: string;
  open?: boolean;
  children: ReactNode;
}) {
  const { reduceMotion, collapseTransition } = useTopologyMotion();
  const isOpen = open !== false;

  return (
    <AnimatePresence initial={false}>
      {isOpen && (
        <motion.ul
          className={className}
          initial={reduceMotion ? false : { opacity: 0, y: -4 }}
          animate={{ opacity: 1, y: 0 }}
          exit={reduceMotion ? { opacity: 0 } : { opacity: 0, y: -4 }}
          transition={{
            ...collapseTransition,
            when: "beforeChildren",
            staggerChildren: reduceMotion ? 0 : 0.04,
            delayChildren: reduceMotion ? 0 : 0.03,
          }}
        >
          {children}
        </motion.ul>
      )}
    </AnimatePresence>
  );
}

function isRaftMetaChip(item: string): boolean {
  return (
    item === "leader" ||
    item === "replica" ||
    item === "standalone" ||
    /^R\d+$/.test(item) ||
    item.startsWith("leader ") ||
    item.startsWith("meta leader ")
  );
}

const NodeCard = memo(function NodeCard({
  node,
  selected,
  layoutEnabled,
  onSelectNode,
  expanded,
  onToggle,
  compact,
}: {
  node: TopologyNode;
  selected: boolean;
  layoutEnabled: boolean;
  onSelectNode?: (node: TopologyNode) => void;
  expanded?: boolean;
  onToggle?: () => void;
  compact?: boolean;
}) {
  const { t } = useTranslation();
  const { spring } = useTopologyMotion();
  const statusClass = node.status ? ` topology-node--${node.status}` : "";
  const kindClass = ` topology-node--kind-${node.kind}`;
  const compactClass = compact ? " topology-node--compact" : "";
  const canToggle = Boolean(onToggle);
  const selectable = node.kind !== "cluster" && Boolean(onSelectNode);

  const onSelect = useCallback(() => {
    if (!selectable) {
      if (canToggle) onToggle?.();
      return;
    }
    onSelectNode?.(node);
  }, [canToggle, node, onSelectNode, onToggle, selectable]);

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onSelect();
    }
  };

  const metaChips = (node.meta ?? []).filter((item) => !isRaftMetaChip(item));
  const showRole = node.role === "leader" || node.role === "replica";
  const replicaCount = node.raft?.clusterSize && node.raft.clusterSize > 1 ? node.raft.clusterSize : 0;
  const nameLayoutId = layoutEnabled ? `topo-name-${node.id}` : undefined;
  const selectedClass = selectable && selected ? " topology-node--selected" : "";
  const selectableClass = selectable ? " topology-node--selectable" : "";

  return (
    <motion.div
      className={`topology-node topology-node--chip${kindClass}${compactClass}${statusClass}${selectedClass}${selectableClass}`}
      role="treeitem"
      aria-selected={selectable ? selected : undefined}
      aria-expanded={canToggle ? expanded : undefined}
      tabIndex={0}
      onClick={onSelect}
      onKeyDown={onKeyDown}
      layout={layoutEnabled}
      transition={spring}
    >
      {canToggle && (
        <button
          type="button"
          className={`topology-node__toggle${expanded ? " is-open" : ""}`}
          aria-label={expanded ? t("topology.collapseNode") : t("topology.expandNode")}
          aria-expanded={expanded}
          onClick={(event) => {
            event.stopPropagation();
            onToggle?.();
          }}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <svg viewBox="0 0 16 16" width="12" height="12" aria-hidden>
            <path
              d="M4 6l4 4 4-4"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.75"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </button>
      )}
      <span className={`topology-node__icon topology-node__icon--${node.kind}`} aria-hidden>
        <KindIcon kind={node.kind} />
      </span>
      <div className="topology-node__body">
        <div className="topology-node__head">
          <span className="topology-node__kind">{t(kindLabelKeys[node.kind])}</span>
          {showRole && (
            <span className={`topology-node__role topology-node__role--${node.role}`}>
              {node.role === "leader" ? t("topology.roleLeader") : t("topology.roleReplica")}
            </span>
          )}
          {replicaCount > 0 && (
            <span className="topology-node__role topology-node__role--replicas">R{replicaCount}</span>
          )}
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
              <motion.span layoutId={nameLayoutId}>{node.name}</motion.span>
            </Link>
          ) : (
            <motion.span layoutId={nameLayoutId}>{node.name}</motion.span>
          )}
        </div>
        {metaChips.length > 0 && (
          <div className="topology-node__meta">
            {metaChips.map((item) => (
              <span key={item} className="topology-node__chip">
                {item}
              </span>
            ))}
          </div>
        )}
      </div>
    </motion.div>
  );
});

const LeafBranch = memo(function LeafBranch({
  node,
  depth,
  isLast,
  branchIndex = 0,
  selectedNodeId,
  onSelectNode,
  layoutEnabled,
}: {
  node: TopologyNode;
  depth: number;
  isLast: boolean;
  branchIndex?: number;
  selectedNodeId?: string | null;
  onSelectNode?: (node: TopologyNode) => void;
  layoutEnabled: boolean;
}) {
  const { itemVariants, transition } = useTopologyMotion();
  const branchStyle = { "--branch-index": branchIndex } as CSSProperties;

  return (
    <motion.li
      className={`topology-branch topology-branch--leaf${isLast ? " topology-branch--last" : ""}`}
      data-depth={depth}
      data-kind={node.kind}
      style={branchStyle}
      variants={itemVariants}
      transition={transition}
    >
      <div className="topology-branch__row">
        <BranchRail visible={depth > 0} />
        <NodeCard
          node={node}
          selected={node.id === selectedNodeId}
          layoutEnabled={layoutEnabled}
          onSelectNode={onSelectNode}
          compact
        />
      </div>
    </motion.li>
  );
});

const StreamGroups = memo(function StreamGroups({
  stream,
  depth,
  isLast,
  selectedNodeId,
  onSelectNode,
  layoutEnabled,
  expand,
}: {
  stream: TopologyNode;
  depth: number;
  isLast: boolean;
  selectedNodeId?: string | null;
  onSelectNode?: (node: TopologyNode) => void;
  layoutEnabled: boolean;
  expand: ExpandApi;
}) {
  const { itemVariants, transition } = useTopologyMotion();
  const { subjects, consumers } = splitStreamChildren(stream);
  const leaves = [...subjects, ...consumers];
  const isSelected = stream.id === selectedNodeId;
  const hasChildren = leaves.length > 0;
  const isOpen = expand.isExpanded(stream.id);

  return (
    <motion.li
      className={`topology-branch topology-branch--stream${isLast ? " topology-branch--last" : ""}${hasChildren && isOpen ? " topology-branch--open" : ""}`}
      data-depth={depth}
      data-kind="stream"
      variants={itemVariants}
      transition={transition}
    >
      <div className="topology-branch__row">
        <BranchRail visible={depth > 0} />
        <NodeCard
          node={stream}
          selected={isSelected}
          layoutEnabled={layoutEnabled}
          onSelectNode={onSelectNode}
          expanded={hasChildren ? isOpen : undefined}
          onToggle={hasChildren ? () => expand.toggle(stream.id) : undefined}
        />
      </div>
      {hasChildren && (
        <BranchChildren className={`topology-branch__children${isOpen ? " is-open" : ""}`} open={isOpen}>
          {leaves.map((leaf, index) => (
            <LeafBranch
              key={leaf.id}
              node={leaf}
              depth={depth + 1}
              isLast={index === leaves.length - 1}
              branchIndex={index}
              selectedNodeId={selectedNodeId}
              onSelectNode={onSelectNode}
              layoutEnabled={layoutEnabled}
            />
          ))}
        </BranchChildren>
      )}
    </motion.li>
  );
});

const TreeBranch = memo(function TreeBranch({
  node,
  depth,
  isLast,
  selectedNodeId,
  onSelectNode,
  layoutEnabled,
  expand,
  branchIndex = 0,
}: {
  node: TopologyNode;
  depth: number;
  isLast: boolean;
  selectedNodeId?: string | null;
  onSelectNode?: (node: TopologyNode) => void;
  layoutEnabled: boolean;
  expand: ExpandApi;
  branchIndex?: number;
}) {
  const { itemVariants, transition } = useTopologyMotion();

  if (node.kind === "stream") {
    return (
      <StreamGroups
        stream={node}
        depth={depth}
        isLast={isLast}
        selectedNodeId={selectedNodeId}
        onSelectNode={onSelectNode}
        layoutEnabled={layoutEnabled}
        expand={expand}
      />
    );
  }

  const hasChildren = node.children.length > 0;
  const isOpen = expand.isExpanded(node.id);
  const branchStyle = { "--branch-index": branchIndex } as CSSProperties;
  const depthClass = depth === 0 ? " topology-branch--root" : "";

  return (
    <motion.li
      className={`topology-branch${depthClass}${isLast ? " topology-branch--last" : ""}${hasChildren && isOpen ? " topology-branch--open" : ""}`}
      data-depth={depth}
      data-kind={node.kind}
      style={branchStyle}
      variants={itemVariants}
      transition={transition}
    >
      <div className="topology-branch__row">
        <BranchRail visible={depth > 0} />
        <NodeCard
          node={node}
          selected={node.id === selectedNodeId}
          layoutEnabled={layoutEnabled}
          onSelectNode={onSelectNode}
          expanded={hasChildren ? isOpen : undefined}
          onToggle={hasChildren ? () => expand.toggle(node.id) : undefined}
        />
      </div>
      {hasChildren && (
        <BranchChildren className={`topology-branch__children${isOpen ? " is-open" : ""}`} open={isOpen}>
          {node.children.map((child, index) => (
            <TreeBranch
              key={child.id}
              node={child}
              depth={depth + 1}
              isLast={index === node.children.length - 1}
              selectedNodeId={selectedNodeId}
              onSelectNode={onSelectNode}
              layoutEnabled={layoutEnabled}
              expand={expand}
              branchIndex={index}
            />
          ))}
        </BranchChildren>
      )}
    </motion.li>
  );
});

export default function TopologyTree({
  root,
  selectedNodeId = null,
  onSelectNode,
  layoutEnabled = true,
}: TopologyTreeProps) {
  const { t } = useTranslation();
  const { listVariants } = useTopologyMotion();
  const [expanded, setExpanded] = useState<Record<string, true>>({});

  const expand: ExpandApi = {
    isExpanded: (id: string) => Boolean(expanded[id]),
    toggle: (id: string) => {
      setExpanded((prev) => {
        if (prev[id]) {
          const next = { ...prev };
          delete next[id];
          return next;
        }
        return { ...prev, [id]: true };
      });
    },
  };

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
            layoutEnabled={layoutEnabled}
            expand={expand}
          />
        </motion.ul>
      </div>
    </div>
  );
}
