import { Link } from "react-router-dom";
import type { StreamOverviewSort, TopologyNode } from "../lib/topology";
import { splitStreamChildren, streamMessageCount } from "../lib/topology";

type TopologyStreamOverviewProps = {
  streams: TopologyNode[];
  selectedStreamId: string | null;
  sortBy: StreamOverviewSort;
  onSortChange: (sort: StreamOverviewSort) => void;
  onSelectStream: (streamId: string) => void;
};

const sortOptions: Array<{ id: StreamOverviewSort; label: string }> = [
  { id: "name", label: "Name" },
  { id: "messages", label: "Messages" },
  { id: "subjects", label: "Subjects" },
  { id: "consumers", label: "Consumers" },
];

function statusLabel(status: TopologyNode["status"]) {
  if (status === "warning") return "Backlog";
  if (status === "idle") return "Idle";
  return "Active";
}

export default function TopologyStreamOverview({
  streams,
  selectedStreamId,
  sortBy,
  onSortChange,
  onSelectStream,
}: TopologyStreamOverviewProps) {
  return (
    <section className="panel topology-overview">
      <div className="topology-overview__head">
        <div>
          <h2 className="panel__title">Stream overview</h2>
          <p className="panel__desc">
            Scan all streams at a glance. Select one to inspect subjects, consumers, and flow.
          </p>
        </div>
        <label className="topology-overview__sort">
          <span>Sort by</span>
          <select value={sortBy} onChange={(event) => onSortChange(event.target.value as StreamOverviewSort)}>
            {sortOptions.map((option) => (
              <option key={option.id} value={option.id}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
      </div>

      <div className="topology-overview__table-wrap">
        <table className="topology-overview__table">
          <thead>
            <tr>
              <th>Stream</th>
              <th>Subjects</th>
              <th>Consumers</th>
              <th>Messages</th>
              <th>Status</th>
              <th aria-hidden />
            </tr>
          </thead>
          <tbody>
            {streams.map((stream) => {
              const { subjects, consumers } = splitStreamChildren(stream);
              const selected = stream.id === selectedStreamId;
              return (
                <tr
                  key={stream.id}
                  className={`topology-overview__row${selected ? " topology-overview__row--selected" : ""}`}
                >
                  <td>
                    <button
                      type="button"
                      className="topology-overview__stream-btn"
                      onClick={() => onSelectStream(stream.id)}
                      aria-pressed={selected}
                    >
                      <span className="topology-overview__stream-icon" aria-hidden>
                        ▤
                      </span>
                      <span className="topology-overview__stream-name">{stream.name}</span>
                    </button>
                  </td>
                  <td>
                    <span className="topology-overview__count">{subjects.length}</span>
                    {subjects.length > 0 && (
                      <span className="topology-overview__hint" title={subjects.map((s) => s.name).join(", ")}>
                        {subjects.length === 1 ? subjects[0].name : `${subjects[0].name} +${subjects.length - 1}`}
                      </span>
                    )}
                  </td>
                  <td>
                    <span className="topology-overview__count">{consumers.length}</span>
                    {consumers.length > 0 && (
                      <span className="topology-overview__hint" title={consumers.map((c) => c.name).join(", ")}>
                        {consumers.length === 1 ? consumers[0].name : `${consumers[0].name} +${consumers.length - 1}`}
                      </span>
                    )}
                  </td>
                  <td className="topology-overview__messages">{streamMessageCount(stream).toLocaleString()}</td>
                  <td>
                    <span className={`topology-overview__status topology-overview__status--${stream.status ?? "healthy"}`}>
                      {statusLabel(stream.status)}
                    </span>
                  </td>
                  <td className="topology-overview__actions">
                    {stream.href && (
                      <Link className="btn btn--secondary btn--small" to={stream.href}>
                        Open
                      </Link>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}
