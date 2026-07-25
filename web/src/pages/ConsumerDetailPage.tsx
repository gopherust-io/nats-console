import { FormEvent, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import Alert from "../components/ui/Alert";
import {
  api,
  clusterPath,
  ConsumerInfo,
  ReplayConsumerRequest,
  ReplayConsumerResult,
} from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";

export default function ConsumerDetailPage() {
  const { name = "", consumer = "" } = useParams();
  const { clusterId } = useCluster();
  const { canWrite } = useAuth();
  const [info, setInfo] = useState<ConsumerInfo | null>(null);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  const [replayMode, setReplayMode] = useState<"reset" | "sidecar">("reset");
  const [replayFrom, setReplayFrom] = useState<"seq" | "time" | "beginning" | "new">("seq");
  const [replaySeq, setReplaySeq] = useState("");
  const [replayTime, setReplayTime] = useState("");
  const [replayPolicy, setReplayPolicy] = useState<"instant" | "original">("instant");
  const [filterSubject, setFilterSubject] = useState("");
  const [sidecarDurable, setSidecarDurable] = useState("");
  const [replaying, setReplaying] = useState(false);
  const [createdDurable, setCreatedDurable] = useState<string | null>(null);

  async function refresh() {
    if (!clusterId || !name || !consumer) return;
    const data = await api<ConsumerInfo>(
      clusterPath(clusterId, `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}`),
    );
    setInfo(data);
    const hint = data.ackFloor?.streamSeq || data.delivered?.streamSeq;
    if (hint && !replaySeq) {
      setReplaySeq(String(hint));
    }
  }

  useEffect(() => {
    if (!clusterId || !name || !consumer) return;
    refresh()
      .catch((err: Error) => setError(err.message));
    // eslint-disable-next-line react-hooks/exhaustive-deps -- load once per route params
  }, [clusterId, name, consumer]);

  async function deleteConsumer() {
    if (!clusterId || !confirm(`Delete consumer "${consumer}"?`)) return;
    try {
      await api(
        clusterPath(clusterId, `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}`),
        { method: "DELETE" },
      );
      window.location.href = `/streams/${name}`;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete consumer");
    }
  }

  async function submitReplay(event: FormEvent) {
    event.preventDefault();
    if (!clusterId) return;

    if (replayMode === "reset") {
      const ok = confirm(
        `Reset consumer "${consumer}"? This recreates the durable and moves its deliver cursor.`,
      );
      if (!ok) return;
    }

    const body: ReplayConsumerRequest = {
      mode: replayMode,
      from: replayFrom,
      replayPolicy,
    };
    if (replayFrom === "seq") {
      const seq = Number(replaySeq);
      if (!Number.isFinite(seq) || seq < 1) {
        setError("Sequence is required for from=seq");
        return;
      }
      body.seq = seq;
    }
    if (replayFrom === "time") {
      if (!replayTime.trim()) {
        setError("Time is required for from=time");
        return;
      }
      body.time = new Date(replayTime).toISOString();
    }
    if (filterSubject.trim()) {
      body.filterSubject = filterSubject.trim();
    }
    if (replayMode === "sidecar" && sidecarDurable.trim()) {
      body.durable = sidecarDurable.trim();
    }

    setReplaying(true);
    setError("");
    setSuccess("");
    setCreatedDurable(null);
    try {
      const result = await api<ReplayConsumerResult>(
        clusterPath(
          clusterId,
          `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}/replay`,
        ),
        { method: "POST", body: JSON.stringify(body) },
      );
      setSuccess(
        result.mode === "sidecar"
          ? `Created side-car durable "${result.durable}".`
          : `Reset durable "${result.durable}".`,
      );
      if (result.mode === "sidecar") {
        setCreatedDurable(result.durable);
      }
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to replay consumer");
    } finally {
      setReplaying(false);
    }
  }

  if (!clusterId) {
    return <p className="text-muted">Select a cluster to view this consumer.</p>;
  }

  if (!info) {
    return <div>{error || "Loading..."}</div>;
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <Link to={`/streams/${name}`} className="link-back">
            ← Back to {name}
          </Link>
          <h1>{info.name}</h1>
        </div>
        {canWrite && (
          <button className="btn danger" onClick={deleteConsumer}>
            Delete Consumer
          </button>
        )}
      </div>

      <Alert variant="error">{error}</Alert>
      <Alert variant="success">{success}</Alert>
      {createdDurable && (
        <p>
          <Link to={`/streams/${name}/consumers/${encodeURIComponent(createdDurable)}`}>
            Open {createdDurable}
          </Link>
        </p>
      )}

      <div className="card-grid">
        <div className="card">
          <div className="card-label">Pending</div>
          <div className="card-value">{info.numPending}</div>
        </div>
        <div className="card">
          <div className="card-label">Ack Pending</div>
          <div className="card-value">{info.numAckPending}</div>
        </div>
        <div className="card">
          <div className="card-label">Deliver Policy</div>
          <div className="card-value card-value--sm">
            {info.config.deliverPolicy}
          </div>
        </div>
        <div className="card">
          <div className="card-label">Ack Policy</div>
          <div className="card-value card-value--sm">
            {info.config.ackPolicy}
          </div>
        </div>
      </div>

      {canWrite && (
        <>
          <h2 className="mt-32">Replay</h2>
          <p className="text-muted">
            Reposition this durable or create a side-car consumer so messages are redelivered from a
            start point. Does not republish payloads onto subjects.
          </p>
          <form className="form-grid card mb-16" onSubmit={submitReplay}>
            <label>
              Mode
              <select
                value={replayMode}
                onChange={(e) => setReplayMode(e.target.value as "reset" | "sidecar")}
              >
                <option value="reset">Reset this consumer</option>
                <option value="sidecar">Create side-car durable</option>
              </select>
            </label>
            <label>
              From
              <select
                value={replayFrom}
                onChange={(e) =>
                  setReplayFrom(e.target.value as "seq" | "time" | "beginning" | "new")
                }
              >
                <option value="seq">sequence</option>
                <option value="time">time</option>
                <option value="beginning">beginning</option>
                <option value="new">new</option>
              </select>
            </label>
            {replayFrom === "seq" && (
              <label>
                Sequence
                <input
                  type="number"
                  min={1}
                  value={replaySeq}
                  onChange={(e) => setReplaySeq(e.target.value)}
                  required
                />
              </label>
            )}
            {replayFrom === "time" && (
              <label>
                Start Time
                <input
                  type="datetime-local"
                  value={replayTime}
                  onChange={(e) => setReplayTime(e.target.value)}
                  required
                />
              </label>
            )}
            <label>
              Replay Policy
              <select
                value={replayPolicy}
                onChange={(e) => setReplayPolicy(e.target.value as "instant" | "original")}
              >
                <option value="instant">instant</option>
                <option value="original">original</option>
              </select>
            </label>
            <label>
              Filter Subject (optional)
              <input
                value={filterSubject}
                onChange={(e) => setFilterSubject(e.target.value)}
                placeholder={info.config.filterSubject || "leave blank to keep"}
              />
            </label>
            {replayMode === "sidecar" && (
              <label>
                Side-car Durable Name (optional)
                <input
                  value={sidecarDurable}
                  onChange={(e) => setSidecarDurable(e.target.value)}
                  placeholder={`${consumer}-replay-…`}
                />
              </label>
            )}
            <button className="btn" type="submit" disabled={replaying}>
              {replaying ? "Replaying…" : "Replay"}
            </button>
          </form>
        </>
      )}

      <div className="card mt-24">
        <div className="card-label">Configuration</div>
        <pre className="mono">{JSON.stringify(info.config, null, 2)}</pre>
      </div>
    </div>
  );
}
