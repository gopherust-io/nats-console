import { FormEvent, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  api,
  clusterPath,
  ConsumerInfo,
  decodeBase64,
  RawMessage,
  StreamInfo,
  tryParseJSON,
} from "../lib/api";
import { useCluster } from "../lib/cluster";
import { useAuth } from "../lib/auth";

type ConsumerListResponse = {
  consumers: ConsumerInfo[];
  total: number;
};

export default function StreamDetailPage() {
  const { name = "" } = useParams();
  const { clusterId } = useCluster();
  const { canWrite } = useAuth();
  const [stream, setStream] = useState<StreamInfo | null>(null);
  const [consumers, setConsumers] = useState<ConsumerInfo[]>([]);
  const [seq, setSeq] = useState("");
  const [message, setMessage] = useState<RawMessage | null>(null);
  const [rawMode, setRawMode] = useState(false);
  const [error, setError] = useState("");
  const [showConsumerForm, setShowConsumerForm] = useState(false);
  const [consumerName, setConsumerName] = useState("");
  const [deliverPolicy, setDeliverPolicy] = useState("all");
  const [ackPolicy, setAckPolicy] = useState("explicit");

  useEffect(() => {
    if (!clusterId || !name) return;
    Promise.all([
      api<StreamInfo>(clusterPath(clusterId, `/streams/${encodeURIComponent(name)}`)),
      api<ConsumerListResponse>(clusterPath(clusterId, `/streams/${encodeURIComponent(name)}/consumers`)),
    ])
      .then(([streamInfo, consumerInfo]) => {
        setStream(streamInfo);
        setConsumers(consumerInfo.consumers);
        if (!seq && streamInfo.state.last_seq > 0) {
          setSeq(String(streamInfo.state.last_seq));
        }
      })
      .catch((err: Error) => setError(err.message));
  }, [clusterId, name]);

  async function loadMessage(targetSeq?: string, direction?: "next" | "prev") {
    if (!clusterId) return;
    const currentSeq = targetSeq ?? seq;
    if (!currentSeq) return;
    try {
      let url = clusterPath(clusterId, `/streams/${encodeURIComponent(name)}/messages?seq=${encodeURIComponent(currentSeq)}`);
      if (direction) url += `&direction=${direction}`;
      const data = await api<RawMessage>(url);
      setMessage(data);
      setSeq(String(data.message.seq));
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load message");
    }
  }

  async function purgeStream() {
    if (!clusterId || !confirm(`Purge all messages in "${name}"?`)) return;
    try {
      await api(clusterPath(clusterId, `/streams/${encodeURIComponent(name)}/purge`), { method: "POST" });
      const updated = await api<StreamInfo>(clusterPath(clusterId, `/streams/${encodeURIComponent(name)}`));
      setStream(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to purge stream");
    }
  }

  async function createConsumer(event: FormEvent) {
    event.preventDefault();
    if (!clusterId) return;
    try {
      await api(clusterPath(clusterId, `/streams/${encodeURIComponent(name)}/consumers`), {
        method: "POST",
        body: JSON.stringify({
          durable_name: consumerName,
          deliver_policy: deliverPolicy,
          ack_policy: ackPolicy,
        }),
      });
      setShowConsumerForm(false);
      setConsumerName("");
      const consumerInfo = await api<ConsumerListResponse>(
        clusterPath(clusterId, `/streams/${encodeURIComponent(name)}/consumers`),
      );
      setConsumers(consumerInfo.consumers);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create consumer");
    }
  }

  if (!stream) {
    return <div>{error || "Loading..."}</div>;
  }

  const payload = message ? decodeBase64(message.message.data) : "";
  const parsed = tryParseJSON(payload);

  return (
    <div>
      <div className="page-header">
        <div>
          <Link to="/streams" className="link-back">
            ← Back to streams
          </Link>
          <h1>{stream.config.name}</h1>
        </div>
        <div className="actions">
          <Link className="btn secondary" to={`/streams/${name}/live`}>
            Live Tail
          </Link>
          {canWrite && (
            <button className="btn secondary" onClick={purgeStream}>
              Purge Stream
            </button>
          )}
        </div>
      </div>

      {error && <div className="error">{error}</div>}

      <div className="card-grid">
        <div className="card">
          <div className="card-label">Messages</div>
          <div className="card-value">{stream.state.messages}</div>
        </div>
        <div className="card">
          <div className="card-label">First / Last Seq</div>
          <div className="card-value card-value--sm">
            {stream.state.first_seq} / {stream.state.last_seq}
          </div>
        </div>
        <div className="card">
          <div className="card-label">Retention</div>
          <div className="card-value card-value--sm">
            {stream.config.retention}
          </div>
        </div>
      </div>

      <div className="section-header">
        <h2>Consumers</h2>
        {canWrite && (
          <button className="btn" onClick={() => setShowConsumerForm((v) => !v)}>
            {showConsumerForm ? "Cancel" : "Create Consumer"}
          </button>
        )}
      </div>

      {showConsumerForm && (
        <form className="form-grid card mb-16" onSubmit={createConsumer}>
          <label>
            Durable Name
            <input value={consumerName} onChange={(e) => setConsumerName(e.target.value)} required />
          </label>
          <label>
            Deliver Policy
            <select value={deliverPolicy} onChange={(e) => setDeliverPolicy(e.target.value)}>
              <option value="all">all</option>
              <option value="last">last</option>
              <option value="new">new</option>
              <option value="by_start_sequence">by_start_sequence</option>
            </select>
          </label>
          <label>
            Ack Policy
            <select value={ackPolicy} onChange={(e) => setAckPolicy(e.target.value)}>
              <option value="explicit">explicit</option>
              <option value="none">none</option>
              <option value="all">all</option>
            </select>
          </label>
          <button className="btn" type="submit">
            Create
          </button>
        </form>
      )}

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Deliver Policy</th>
              <th>Ack Policy</th>
              <th>Pending</th>
              <th>Ack Pending</th>
            </tr>
          </thead>
          <tbody>
            {consumers.map((consumer) => (
              <tr key={consumer.name}>
                <td>
                  <Link to={`/streams/${name}/consumers/${consumer.name}`}>{consumer.name}</Link>
                </td>
                <td>{consumer.config.deliver_policy}</td>
                <td>{consumer.config.ack_policy}</td>
                <td>{consumer.num_pending}</td>
                <td>{consumer.num_ack_pending}</td>
              </tr>
            ))}
            {consumers.length === 0 && (
              <tr>
                <td colSpan={5} className="text-muted">
                  No consumers
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <h2 className="mt-32">Message Browser</h2>
      <div className="form-grid form-grid--inline">
        <label>
          Sequence
          <input value={seq} onChange={(e) => setSeq(e.target.value)} placeholder="1" />
        </label>
        <button className="btn" onClick={() => loadMessage()}>
          Load
        </button>
        <button className="btn secondary" disabled={!message?.navigation?.prev_seq} onClick={() => loadMessage(String(message?.navigation?.prev_seq), "prev")}>
          ← Prev
        </button>
        <button className="btn secondary" disabled={!message?.navigation?.next_seq} onClick={() => loadMessage(String(message?.navigation?.next_seq), "next")}>
          Next →
        </button>
      </div>

      {message && (
        <div className="card mt-16">
          <div className="card-label">
            #{message.message.seq} · {message.message.subject} · {message.message.time}
          </div>
          <div className="mb-8">
            <button className="btn secondary" onClick={() => setRawMode((v) => !v)}>
              {rawMode ? "Show JSON" : "Show Raw"}
            </button>
          </div>
          {message.message.hdrs && message.message.hdrs.length > 0 && (
            <div className="mono text-muted mb-8">
              Headers: {message.message.hdrs.length} bytes
            </div>
          )}
          <div className="mono">
            {rawMode || !parsed.isJSON
              ? payload
              : JSON.stringify(parsed.parsed, null, 2)}
          </div>
        </div>
      )}
    </div>
  );
}
