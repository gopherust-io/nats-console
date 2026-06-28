import { Link } from "react-router-dom";
import type { StreamOverviewSort, TopologyNode } from "../lib/topology";
import { splitStreamChildren, streamMessageCount } from "../lib/topology";
import VirtualTable, { type VirtualTableColumn } from "./VirtualTable";

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

const columns: VirtualTableColumn[] = [
  { id: "stream", header: "Stream", width: "minmax(160px, 1.4fr)" },
  { id: "subjects", header: "Subjects", width: "minmax(120px, 1fr)" },
  { id: "consumers", header: "Consumers", width: "minmax(120px, 1fr)" },
  { id: "messages", header: "Messages", width: "100px", align: "right" },
  { id: "status", header: "Status", width: "100px" },
  { id: "actions", header: "", width: "88px", align: "right" },
];

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

      <div className="topology-overview__virtual">
        <VirtualTable
          columns={columns}
          items={streams}
          rowHeight={56}
          maxHeight={560}
          empty="No streams in this cluster"
          getKey={(stream) => stream.id}
          renderCell={(stream, columnId) => {
            const { subjects, consumers } = splitStreamChildren(stream);
            const selected = stream.id === selectedStreamId;

            switch (columnId) {
              case "stream":
                return (
                  <button
                    type="button"
                    className={`topology-overview__stream-btn${selected ? " topology-overview__stream-btn--selected" : ""}`}
                    onClick={() => onSelectStream(stream.id)}
                    aria-pressed={selected}
                  >
                    <span className="topology-overview__stream-icon" aria-hidden>
                      ▤
                    </span>
                    <span className="topology-overview__stream-name">{stream.name}</span>
                  </button>
                );
              case "subjects":
                return (
                  <>
                    <span className="topology-overview__count">{subjects.length}</span>
                    {subjects.length > 0 && (
                      <span className="topology-overview__hint" title={subjects.map((s) => s.name).join(", ")}>
                        {subjects.length === 1 ? subjects[0].name : `${subjects[0].name} +${subjects.length - 1}`}
                      </span>
                    )}
                  </>
                );
              case "consumers":
                return (
                  <>
                    <span className="topology-overview__count">{consumers.length}</span>
                    {consumers.length > 0 && (
                      <span className="topology-overview__hint" title={consumers.map((c) => c.name).join(", ")}>
                        {consumers.length === 1 ? consumers[0].name : `${consumers[0].name} +${consumers.length - 1}`}
                      </span>
                    )}
                  </>
                );
              case "messages":
                return (
                  <span className="topology-overview__messages">{streamMessageCount(stream).toLocaleString()}</span>
                );
              case "status":
                return (
                  <span
                    className={`topology-overview__status topology-overview__status--${stream.status ?? "healthy"}`}
                  >
                    {statusLabel(stream.status)}
                  </span>
                );
              case "actions":
                return stream.href ? (
                  <Link className="btn btn--secondary btn--small" to={stream.href}>
                    Open
                  </Link>
                ) : null;
              default:
                return null;
            }
          }}
        />
      </div>
    </section>
  );
}
