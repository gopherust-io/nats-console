import { useMemo } from "react";
import { Link } from "react-router-dom";
import type { TopologyNode } from "../lib/topology";

type TopologyFlowDiagramProps = {
  root?: TopologyNode;
  streams?: TopologyNode[];
  maxStreams?: number;
};

type ArrowDirection = "right" | "left" | "down";

function FlowNode({
  kind,
  name,
  meta,
  href,
}: {
  kind: "subject" | "stream" | "consumer";
  name: string;
  meta?: string;
  href?: string;
}) {
  const label = kind === "subject" ? "Subject" : kind === "stream" ? "Stream" : "Consumer";
  const body = (
    <div className={`topo-flow-node topo-flow-node--${kind}`}>
      <span className="topo-flow-node__kind">{label}</span>
      <span className="topo-flow-node__name" title={name}>
        {name}
      </span>
      {meta && <span className="topo-flow-node__meta">{meta}</span>}
    </div>
  );

  if (href) {
    return (
      <Link to={href} className="topo-flow-node-link">
        {body}
      </Link>
    );
  }

  return body;
}

function FlowArrow({ delay = 0, direction = "right" }: { delay?: number; direction?: ArrowDirection }) {
  const isVertical = direction === "down";

  return (
    <div
      className={`topo-flow-arrow topo-flow-arrow--${direction}`}
      style={{ animationDelay: `${delay}ms` }}
      aria-hidden
    >
      <svg
        className="topo-flow-arrow__static"
        viewBox={isVertical ? "0 0 14 52" : "0 0 52 14"}
        fill="none"
        aria-hidden
      >
        {isVertical ? (
          <>
            <path className="topo-flow-arrow__static-line" d="M7 2 V38" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            <path
              className="topo-flow-arrow__static-head"
              d="M3 38 L7 46 L11 38"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </>
        ) : (
          <>
            <path className="topo-flow-arrow__static-line" d="M2 7 H38" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            <path
              className="topo-flow-arrow__static-head"
              d="M38 3 L46 7 L38 11"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </>
        )}
      </svg>
      <span className="topo-flow-arrow__track" />
      <span className="topo-flow-arrow__dot" />
    </div>
  );
}

function StreamFlowCard({ stream }: { stream: TopologyNode }) {
  const subjects = stream.children.filter((node) => node.kind === "subject");
  const consumers = stream.children.filter((node) => node.kind === "consumer");
  const rows = Math.max(subjects.length, consumers.length, 1);

  return (
    <article className="topo-flow-card">
      <header className="topo-flow-card__header">
        <div className="topo-flow-card__stream-stack">
          <FlowNode kind="stream" name={stream.name} href={stream.href} />
          <FlowArrow direction="down" />
        </div>
      </header>

      <div className="topo-flow-card__col topo-flow-card__col--captures">
        <span className="topo-flow-card__col-label">Captures</span>
        {Array.from({ length: rows }, (_, index) => {
          const subject = subjects[index];
          return (
            <div key={subject?.id ?? `subject-empty-${index}`} className="topo-flow-card__row topo-flow-card__row--captures">
              {subject ? (
                <>
                  <FlowNode kind="subject" name={subject.name} />
                  <FlowArrow delay={index * 180} direction="right" />
                </>
              ) : (
                <span className="topo-flow-card__spacer" />
              )}
            </div>
          );
        })}
      </div>

      <div className="topo-flow-card__hub" aria-hidden>
        <div className="topo-flow-card__hub-ring" />
        <div className="topo-flow-card__hub-core">JS</div>
      </div>

      <div className="topo-flow-card__col topo-flow-card__col--delivers">
        <span className="topo-flow-card__col-label">Delivers to</span>
        {Array.from({ length: rows }, (_, index) => {
          const consumer = consumers[index];
          const filter = consumer?.meta?.find((item) => item.startsWith("Filter "));
          return (
            <div key={consumer?.id ?? `consumer-empty-${index}`} className="topo-flow-card__row topo-flow-card__row--delivers">
              {consumer ? (
                <>
                  <FlowArrow delay={index * 180 + 90} direction="right" />
                  <FlowNode
                    kind="consumer"
                    name={consumer.name}
                    meta={filter?.replace("Filter ", "filter: ")}
                    href={consumer.href}
                  />
                </>
              ) : (
                <span className="topo-flow-card__spacer" />
              )}
            </div>
          );
        })}
      </div>
    </article>
  );
}

export default function TopologyFlowDiagram({ root, streams: streamsProp, maxStreams = 3 }: TopologyFlowDiagramProps) {
  const streams = useMemo(() => {
    if (streamsProp) return streamsProp;
    if (root) return root.children.filter((node) => node.kind === "stream");
    return [];
  }, [root, streamsProp]);

  const visibleStreams = streams.slice(0, maxStreams);
  const hiddenCount = streams.length - visibleStreams.length;

  if (streams.length === 0) {
    return null;
  }

  const flowDescription =
    streams.length === 1
      ? "How messages flow from subject patterns into this stream and out to consumers."
      : "Subjects publish into streams; streams deliver to consumers. Follow the arrows: subject → stream → consumer.";

  return (
    <section className="topo-flow-section">
      <div className="topo-flow-section__head">
        <h2 className="topo-flow-section__title">Relationship flow</h2>
        <p className="topo-flow-section__desc">{flowDescription}</p>
        {hiddenCount > 0 && (
          <p className="topo-flow-section__note">
            Showing {visibleStreams.length} of {streams.length} streams. Select a stream in the overview to inspect others.
          </p>
        )}
      </div>
      <div className="topo-flow-grid">
        {visibleStreams.map((stream, index) => (
          <div
            key={stream.id}
            className="topo-flow-grid__item"
            style={{ animationDelay: `${index * 90}ms` }}
          >
            <StreamFlowCard stream={stream} />
          </div>
        ))}
      </div>
    </section>
  );
}
