import { FormEvent, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation, useParams } from "react-router";
import Alert from "../components/ui/Alert";
import CreateConsumerPanel, { ConsumerConfigPayload } from "../components/CreateConsumerPanel";
import {
  api,
  clusterPath,
  ConsumerInfo,
  jetStreamUIBase,
  ReplayConsumerRequest,
  ReplayConsumerResult,
} from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { invalidateJetStreamTopology } from "../lib/query";
import { isFromTopology, TOPOLOGY_LOCATION_STATE } from "../lib/topology";

export default function ConsumerDetailPage() {
  const { t } = useTranslation();
  const { name = "", consumer = "", clusterId: routeCluster, accountName } = useParams();
  const { clusterId } = useCluster();
  const id = routeCluster ?? clusterId;
  const jsBase = id ? jetStreamUIBase(id, accountName) : "";
  const streamHref = jsBase ? `${jsBase}/streams/${encodeURIComponent(name)}` : "/systems";
  const location = useLocation();
  const fromTopology = isFromTopology(location.state);
  const backHref = fromTopology ? "/admin/topology" : streamHref;
  const { canManageJetStream } = useAuth();
  const [info, setInfo] = useState<ConsumerInfo | null>(null);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [editOpen, setEditOpen] = useState(false);
  const [editError, setEditError] = useState("");
  const [editBusy, setEditBusy] = useState(false);

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
    if (!id || !name || !consumer) return;
    const data = await api<ConsumerInfo>(
      clusterPath(id, `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}`),
    );
    setInfo(data);
    const hint = data.ackFloor?.streamSeq || data.delivered?.streamSeq;
    if (hint && !replaySeq) {
      setReplaySeq(String(hint));
    }
  }

  useEffect(() => {
    if (!id || !name || !consumer) return;
    refresh()
      .catch((err: Error) => setError(err.message));
    // eslint-disable-next-line react-hooks/exhaustive-deps -- load once per route params
  }, [id, name, consumer]);

  async function deleteConsumer() {
    if (!id || !confirm(`Delete consumer "${consumer}"?`)) return;
    try {
      await api(
        clusterPath(id, `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}`),
        { method: "DELETE" },
      );
      await invalidateJetStreamTopology(id);
      window.location.href = fromTopology ? "/admin/topology" : streamHref;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete consumer");
    }
  }

  async function saveConsumerConfig(body: ConsumerConfigPayload) {
    if (!id) return;
    setEditBusy(true);
    setEditError("");
    try {
      const updated = await api<ConsumerInfo>(
        clusterPath(id, `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}`),
        {
          method: "PUT",
          body: JSON.stringify({ ...body, durableName: consumer }),
        },
      );
      setInfo(updated);
      setEditOpen(false);
      await invalidateJetStreamTopology(id);
    } catch (err) {
      setEditError(err instanceof Error ? err.message : "Failed to update consumer");
      throw err;
    } finally {
      setEditBusy(false);
    }
  }

  async function submitReplay(event: FormEvent) {
    event.preventDefault();
    if (!id) return;

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
          id,
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

  if (!id) {
    return <p className="text-muted">Select a cluster to view this consumer.</p>;
  }

  if (!info) {
    return <div>{error || "Loading..."}</div>;
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <Link
            to={backHref}
            className="link-back"
            state={fromTopology ? TOPOLOGY_LOCATION_STATE : undefined}
          >
            {fromTopology ? t("topology.backToTopology") : t("consumers.backToStream", { name })}
          </Link>
          <h1>{info.name}</h1>
          {info.config.description ? <p className="text-muted">{info.config.description}</p> : null}
        </div>
        {canManageJetStream && (
          <div className="actions">
            <button
              type="button"
              className="btn secondary"
              onClick={() => {
                setEditError("");
                setEditOpen(true);
              }}
            >
              {t("jetstream.editConfig")}
            </button>
            <button className="btn danger" type="button" onClick={deleteConsumer}>
              Delete Consumer
            </button>
          </div>
        )}
      </div>

      <Alert variant="error">{error}</Alert>
      <Alert variant="success">{success}</Alert>

      <CreateConsumerPanel
        mode="edit"
        open={editOpen}
        initial={info.config}
        busy={editBusy}
        error={editError}
        onClose={() => {
          setEditOpen(false);
          setEditError("");
        }}
        onSubmit={saveConsumerConfig}
      />
      {createdDurable && (
        <p>
          <Link to={`${streamHref}/consumers/${encodeURIComponent(createdDurable)}`}>
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

      {canManageJetStream && (
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
