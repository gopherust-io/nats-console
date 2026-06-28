import { useMemo, useState, type CSSProperties } from "react";
import { Link } from "react-router-dom";
import type { TopologyNode, TopologyNodeKind } from "../lib/topology";
import { splitStreamChildren } from "../lib/topology";

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
  defaultExpanded?: boolean;
  selectedStreamId?: string | null;
};

function NodeCard({
  node,
  expanded,
  hasChildren,
  onToggle,
}: {
  node: TopologyNode;
  expanded: boolean;
  hasChildren: boolean;
  onToggle: () => void;
}) {
  const statusClass = node.status ? ` topology-node--${node.status}` : "";

  return (
    <div className={`topology-node${statusClass}`}>
      <span className={`topology-node__icon topology-node__icon--${node.kind}`} aria-hidden>
        {kindIcons[node.kind]}
      </span>
      <div className="topology-node__body">
        <div className="topology-node__head">
          <span className="topology-node__kind">{kindLabels[node.kind]}</span>
          {node.status && <span className={`topology-node__status topology-node__status--${node.status}`} />}
        </div>
        <div className="topology-node__name">
          {node.href ? <Link to={node.href}>{node.name}</Link> : node.name}
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
      {hasChildren && (
        <button
          type="button"
          className={`topology-node__toggle${expanded ? " is-open" : ""}`}
          onClick={onToggle}
          aria-expanded={expanded}
          aria-label={expanded ? "Collapse" : "Expand"}
        >
          ▾
        </button>
      )}
    </div>
  );
}

function GroupHeader({
  label,
  count,
  expanded,
  onToggle,
}: {
  label: string;
  count: number;
  expanded: boolean;
  onToggle: () => void;
}) {
  return (
    <button type="button" className="topology-group__header" onClick={onToggle} aria-expanded={expanded}>
      <span className="topology-group__label">{label}</span>
      <span className="topology-group__count">{count}</span>
      <span className={`topology-group__chevron${expanded ? " is-open" : ""}`} aria-hidden>
        ▾
      </span>
    </button>
  );
}

function LeafBranch({
  node,
  depth,
  isLast,
  branchIndex = 0,
}: {
  node: TopologyNode;
  depth: number;
  isLast: boolean;
  branchIndex?: number;
}) {
  const branchStyle = { "--branch-index": branchIndex } as CSSProperties;

  return (
    <li className={`topology-branch${isLast ? " topology-branch--last" : ""}`} data-depth={depth} style={branchStyle}>
      <div className="topology-branch__row">
        {depth > 0 && <span className="topology-branch__rail" aria-hidden />}
        <NodeCard node={node} expanded={false} hasChildren={false} onToggle={() => undefined} />
      </div>
    </li>
  );
}

function StreamGroups({
  stream,
  depth,
  defaultExpanded,
  selectedStreamId,
}: {
  stream: TopologyNode;
  depth: number;
  defaultExpanded: boolean;
  selectedStreamId?: string | null;
}) {
  const { subjects, consumers } = splitStreamChildren(stream);
  const isSelected = stream.id === selectedStreamId;
  const [streamOpen, setStreamOpen] = useState(() => defaultExpanded || isSelected);
  const [subjectsOpen, setSubjectsOpen] = useState(() => isSelected || subjects.length <= 4);
  const [consumersOpen, setConsumersOpen] = useState(() => isSelected || consumers.length <= 4);

  return (
    <li className="topology-branch topology-branch--last" data-depth={depth}>
      <div className="topology-branch__row">
        {depth > 0 && <span className="topology-branch__rail" aria-hidden />}
        <NodeCard
          node={stream}
          expanded={streamOpen}
          hasChildren={subjects.length + consumers.length > 0}
          onToggle={() => setStreamOpen((value) => !value)}
        />
      </div>
      {streamOpen && (subjects.length > 0 || consumers.length > 0) && (
        <ul className="topology-branch__children is-open">
          {subjects.length > 0 && (
            <li className="topology-group">
              <GroupHeader
                label="Subjects"
                count={subjects.length}
                expanded={subjectsOpen}
                onToggle={() => setSubjectsOpen((value) => !value)}
              />
              {subjectsOpen && (
                <ul className="topology-group__items">
                  {subjects.map((subject, index) => (
                    <LeafBranch
                      key={subject.id}
                      node={subject}
                      depth={depth + 2}
                      isLast={index === subjects.length - 1}
                      branchIndex={index}
                    />
                  ))}
                </ul>
              )}
            </li>
          )}
          {consumers.length > 0 && (
            <li className="topology-group">
              <GroupHeader
                label="Consumers"
                count={consumers.length}
                expanded={consumersOpen}
                onToggle={() => setConsumersOpen((value) => !value)}
              />
              {consumersOpen && (
                <ul className="topology-group__items">
                  {consumers.map((consumer, index) => (
                    <LeafBranch
                      key={consumer.id}
                      node={consumer}
                      depth={depth + 2}
                      isLast={index === consumers.length - 1}
                      branchIndex={index}
                    />
                  ))}
                </ul>
              )}
            </li>
          )}
        </ul>
      )}
    </li>
  );
}

function TreeBranch({
  node,
  depth,
  isLast,
  defaultExpanded,
  selectedStreamId,
  branchIndex = 0,
}: {
  node: TopologyNode;
  depth: number;
  isLast: boolean;
  defaultExpanded: boolean;
  selectedStreamId?: string | null;
  branchIndex?: number;
}) {
  const [expanded, setExpanded] = useState(() => depth === 0 || defaultExpanded);

  if (node.kind === "stream") {
    return (
      <StreamGroups
        stream={node}
        depth={depth}
        defaultExpanded={defaultExpanded}
        selectedStreamId={selectedStreamId}
      />
    );
  }

  const hasChildren = node.children.length > 0;
  const branchStyle = { "--branch-index": branchIndex } as CSSProperties;

  return (
    <li
      className={`topology-branch${isLast ? " topology-branch--last" : ""}`}
      data-depth={depth}
      style={branchStyle}
    >
      <div className="topology-branch__row">
        {depth > 0 && <span className="topology-branch__rail" aria-hidden />}
        <NodeCard
          node={node}
          expanded={expanded}
          hasChildren={hasChildren}
          onToggle={() => setExpanded((value) => !value)}
        />
      </div>
      {hasChildren && (
        <ul className={`topology-branch__children${expanded ? " is-open" : ""}`}>
          {node.children.map((child, index) => (
            <TreeBranch
              key={child.id}
              node={child}
              depth={depth + 1}
              isLast={index === node.children.length - 1}
              defaultExpanded={defaultExpanded}
              selectedStreamId={selectedStreamId}
              branchIndex={index}
            />
          ))}
        </ul>
      )}
    </li>
  );
}

export default function TopologyTree({
  root,
  defaultExpanded = false,
  selectedStreamId = null,
}: TopologyTreeProps) {
  const legend = useMemo(
    () => [
      { kind: "subject" as const, label: "Subject publishes to a pattern" },
      { kind: "stream" as const, label: "Stream captures matching messages" },
      { kind: "consumer" as const, label: "Consumer receives from the stream" },
    ],
    [],
  );

  return (
    <div className="topology-tree">
      <div className="topology-tree__head">
        <div>
          <h2 className="topology-tree__title">Hierarchy explorer</h2>
          <p className="topology-tree__subtitle">Subjects and consumers are grouped under each stream.</p>
        </div>
      </div>

      <div className="topology-tree__legend">
        {legend.map((item) => (
          <span key={item.kind} className="topology-tree__legend-item">
            <span className={`topology-node__icon topology-node__icon--${item.kind}`}>{kindIcons[item.kind]}</span>
            {item.label}
          </span>
        ))}
      </div>

      <div className="topology-tree__canvas">
        <ul className="topology-tree__root">
          <TreeBranch
            node={root}
            depth={0}
            isLast
            defaultExpanded={defaultExpanded}
            selectedStreamId={selectedStreamId}
          />
        </ul>
      </div>
    </div>
  );
}
