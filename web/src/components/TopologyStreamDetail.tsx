import { Link } from "react-router-dom";
import TopologyFlowDiagram from "./TopologyFlowDiagram";
import type { TopologyNode } from "../lib/topology";
import { splitStreamChildren } from "../lib/topology";

type TopologyStreamDetailProps = {
  stream: TopologyNode;
  onClose: () => void;
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
                <Link to={consumer.href} className="topology-detail__consumer-name">
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

export default function TopologyStreamDetail({ stream, onClose }: TopologyStreamDetailProps) {
  const { subjects, consumers } = splitStreamChildren(stream);

  return (
    <section className="topology-detail panel">
      <div className="topology-detail__head">
        <div>
          <p className="topology-detail__eyebrow">Selected stream</p>
          <h2 className="panel__title">{stream.name}</h2>
          {stream.meta && stream.meta.length > 0 && (
            <div className="topology-detail__meta">
              {stream.meta.map((item) => (
                <span key={item} className="topology-detail__chip">
                  {item}
                </span>
              ))}
            </div>
          )}
        </div>
        <div className="topology-detail__actions">
          {stream.href && (
            <Link className="btn btn--secondary" to={stream.href}>
              Open stream
            </Link>
          )}
          <button className="btn btn--ghost" type="button" onClick={onClose}>
            Clear selection
          </button>
        </div>
      </div>

      <TopologyFlowDiagram streams={[stream]} />

      <div className="topology-detail__columns">
        <div className="topology-detail__column">
          <h3 className="topology-detail__column-title">
            Subjects <span className="topology-detail__badge">{subjects.length}</span>
          </h3>
          <p className="topology-detail__column-desc">Publish patterns captured by this stream.</p>
          <SubjectList subjects={subjects} />
        </div>
        <div className="topology-detail__column">
          <h3 className="topology-detail__column-title">
            Consumers <span className="topology-detail__badge">{consumers.length}</span>
          </h3>
          <p className="topology-detail__column-desc">Durable subscribers reading from the stream.</p>
          <ConsumerList consumers={consumers} />
        </div>
      </div>
    </section>
  );
}
