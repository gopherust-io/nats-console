import { FormEvent, useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import Pager, { DEFAULT_PAGE_SIZE, pageQuery } from "../components/Pager";
import VirtualTable from "../components/VirtualTable";
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
import { clusterQueryKey } from "../lib/query";

type ConsumerListResponse = {
  consumers: ConsumerInfo[];
  total: number;
  offset: number;
  limit: number;
};

export default function StreamDetailPage() {
  const { name = "" } = useParams();
  const { clusterId } = useCluster();
  const { canWrite } = useAuth();
  const queryClient = useQueryClient();
  const [stream, setStream] = useState<StreamInfo | null>(null);
  const [consumerOffset, setConsumerOffset] = useState(0);
  const [seq, setSeq] = useState("");
  const [message, setMessage] = useState<RawMessage | null>(null);
  const [rawMode, setRawMode] = useState(false);
  const [publishSubject, setPublishSubject] = useState("");
  const [publishPayload, setPublishPayload] = useState('{"hello":"world"}');
  const [publishRawMode, setPublishRawMode] = useState(false);
  const [error, setError] = useState("");
  const [showConsumerForm, setShowConsumerForm] = useState(false);
  const [consumerName, setConsumerName] = useState("");
  const [deliverPolicy, setDeliverPolicy] = useState("all");
  const [ackPolicy, setAckPolicy] = useState("explicit");
  const limit = DEFAULT_PAGE_SIZE;

  useEffect(() => {
    if (!clusterId || !name) return;
    api<StreamInfo>(clusterPath(clusterId, `/streams/${encodeURIComponent(name)}`))
      .then((streamInfo) => {
        setStream(streamInfo);
        setPublishSubject(
          streamInfo.config.subjects?.find((s) => !s.includes("*") && !s.includes(">")) ??
            streamInfo.config.subjects?.[0] ??
            "",
        );
        setSeq((current) =>
          current || (streamInfo.state.lastSeq > 0 ? String(streamInfo.state.lastSeq) : ""),
        );
      })
      .catch((err: Error) => setError(err.message));
  }, [clusterId, name]);

  const consumersQuery = useQuery({
    queryKey: [...clusterQueryKey(clusterId, `consumers:${name}`), consumerOffset],
    queryFn: () =>
      api<ConsumerListResponse>(
        clusterPath(clusterId!, `/streams/${encodeURIComponent(name)}/consumers${pageQuery(consumerOffset, limit)}`),
      ),
    enabled: Boolean(clusterId && name),
  });

  const consumers = consumersQuery.data?.consumers ?? [];
  const consumerTotal = consumersQuery.data?.total ?? 0;
  const consumersError =
    consumersQuery.error instanceof Error ? consumersQuery.error.message : "";

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
          durableName: consumerName,
          deliverPolicy,
          ackPolicy,
        }),
      });
      setShowConsumerForm(false);
      setConsumerName("");
      await queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `consumers:${name}`) });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create consumer");
    }
  }

  if (!clusterId) {
    return <p className="text-muted">Select a cluster to view this stream.</p>;
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

      {(error || consumersError) && <div className="error">{error || consumersError}</div>}

      <div className="card-grid">
        <div className="card">
          <div className="card-label">Messages</div>
          <div className="card-value">{stream.state.messages}</div>
        </div>
        <div className="card">
          <div className="card-label">First / Last Seq</div>
          <div className="card-value card-value--sm">
            {stream.state.firstSeq} / {stream.state.lastSeq}
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
        <VirtualTable
          columns={[
            { id: "name", header: "Name", width: "minmax(140px, 1.3fr)" },
            { id: "deliver", header: "Deliver Policy", width: "minmax(120px, 1fr)" },
            { id: "ack", header: "Ack Policy", width: "minmax(120px, 1fr)" },
            { id: "pending", header: "Pending", width: "96px", align: "right" },
            { id: "ackPending", header: "Ack Pending", width: "112px", align: "right" },
          ]}
          items={consumers}
          empty="No consumers"
          getKey={(consumer) => consumer.name}
          renderCell={(consumer, columnId) => {
            switch (columnId) {
              case "name":
                return <Link to={`/streams/${name}/consumers/${consumer.name}`}>{consumer.name}</Link>;
              case "deliver":
                return consumer.config.deliverPolicy;
              case "ack":
                return consumer.config.ackPolicy;
              case "pending":
                return consumer.numPending;
              case "ackPending":
                return consumer.numAckPending;
              default:
                return null;
            }
          }}
        />
      </div>

      <Pager total={consumerTotal} offset={consumerOffset} limit={limit} onPageChange={setConsumerOffset} />

      <h2 className="mt-32">Message Browser</h2>

      {canWrite && (
        <form
          className="form-grid card mb-16"
          onSubmit={async (event) => {
            event.preventDefault();
            if (!clusterId) return;
            try {
              const body =
                publishRawMode || !tryParseJSON(publishPayload).isJSON
                  ? publishPayload
                  : JSON.stringify(tryParseJSON(publishPayload).parsed);
              const data = btoa(unescape(encodeURIComponent(body)));
              const result = await api<{ seq: number }>(
                clusterPath(clusterId, `/streams/${encodeURIComponent(name)}/messages`),
                {
                  method: "POST",
                  body: JSON.stringify({
                    subject: publishSubject,
                    data,
                  }),
                },
              );
              setSeq(String(result.seq));
              await loadMessage(String(result.seq));
              setError("");
            } catch (err) {
              setError(err instanceof Error ? err.message : "Failed to publish message");
            }
          }}
        >
          <h3 className="section-title">Publish Message</h3>
          <label>
            Subject
            <input
              value={publishSubject}
              onChange={(e) => setPublishSubject(e.target.value)}
              placeholder={stream.config.subjects?.join(", ")}
              required
            />
          </label>
          <label className="form-grid__full">
            Payload
            <textarea
              rows={6}
              value={publishPayload}
              onChange={(e) => setPublishPayload(e.target.value)}
              placeholder='{"hello":"world"}'
              required
            />
          </label>
          <div className="form-grid__full">
            <button className="btn secondary" type="button" onClick={() => setPublishRawMode((v) => !v)}>
              {publishRawMode ? "JSON mode" : "Raw mode"}
            </button>
            <button className="btn" type="submit">
              Publish
            </button>
          </div>
        </form>
      )}

      <div className="form-grid form-grid--inline">
        <label>
          Sequence
          <input value={seq} onChange={(e) => setSeq(e.target.value)} placeholder="1" />
        </label>
        <button className="btn" onClick={() => loadMessage()}>
          Load
        </button>
        <button className="btn secondary" disabled={!message?.navigation?.prevSeq} onClick={() => loadMessage(String(message?.navigation?.prevSeq), "prev")}>
          ← Prev
        </button>
        <button className="btn secondary" disabled={!message?.navigation?.nextSeq} onClick={() => loadMessage(String(message?.navigation?.nextSeq), "next")}>
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
          {message.message.hdrs && Array.isArray(message.message.hdrs) && message.message.hdrs.length > 0 && (
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
