import { FormEvent, lazy, Suspense, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useLocation, useParams } from "react-router";
import Alert from "../components/ui/Alert";
import ConfirmDialog from "../components/ui/ConfirmDialog";
import QueryErrorState from "../components/ui/QueryErrorState";
import CreateConsumerPanel, { ConsumerConfigPayload } from "../components/CreateConsumerPanel";
import { useConfirmDialog } from "../hooks/useConfirmDialog";
import { Seg, SegBtn } from "../components/Seg";
import {
  api,
  BehaviorFingerprintReport,
  clusterPath,
  ConsumerInfo,
  jetStreamUIBase,
  ReplayConsumerRequest,
  ReplayConsumerResult,
  ReplayDryRun,
  StreamInfo,
} from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { STREAM_STATE_POLL_MS } from "../lib/constants";
import { consumerLag, formatAckWaitNs, formatSeqPair } from "../lib/consumerMetrics";
import { clusterQueryKey, invalidateJetStreamTopology, visibilityAwareInterval } from "../lib/query";
import { isFromTopology, TOPOLOGY_LOCATION_STATE } from "../lib/topology";
import { isFromZombies, ZOMBIES_LOCATION_STATE, ZOMBIES_TOPOLOGY_HREF } from "../lib/zombie";
import { isFromNaming, NAMING_LOCATION_STATE, NAMING_TOPOLOGY_HREF } from "../lib/subjectNaming";
import { GENOME_LOCATION_STATE, GENOME_TOPOLOGY_HREF, isFromGenome } from "../lib/eventGenome";
import {
  fetchHiddenBottlenecks,
  findingMatchesConsumer,
  HIDDEN_BOTTLENECKS_HREF,
} from "../lib/hiddenBottlenecks";

const ReplayDryRunPanel = lazy(() => import("../components/ReplayDryRunPanel"));
const BehaviorFingerprintPanel = lazy(() => import("../components/BehaviorFingerprintPanel"));
const IncidentCapsulePanel = lazy(() => import("../components/IncidentCapsulePanel"));

type ConsumerDetailData = {
  info: ConsumerInfo;
  streamLastSeq: number;
};

export default function ConsumerDetailPage() {
  const { t } = useTranslation();
  const { askConfirm, confirmDialog } = useConfirmDialog();
  const { name = "", consumer = "", clusterId: routeCluster, accountName } = useParams();
  const { clusterId } = useCluster();
  const id = routeCluster ?? clusterId;
  const jsBase = id ? jetStreamUIBase(id, accountName) : "";
  const streamHref = jsBase ? `${jsBase}/streams/${encodeURIComponent(name)}` : "/systems";
  const location = useLocation();
  const fromZombies = isFromZombies(location.state);
  const fromNaming = isFromNaming(location.state);
  const fromGenome = isFromGenome(location.state);
  const fromTopology = isFromTopology(location.state);
  const backHref = fromZombies
    ? ZOMBIES_TOPOLOGY_HREF
    : fromNaming
      ? NAMING_TOPOLOGY_HREF
      : fromGenome
        ? GENOME_TOPOLOGY_HREF
        : fromTopology
          ? "/admin/topology"
          : streamHref;
  const backState = fromZombies
    ? ZOMBIES_LOCATION_STATE
    : fromNaming
      ? NAMING_LOCATION_STATE
      : fromGenome
        ? GENOME_LOCATION_STATE
        : fromTopology
          ? TOPOLOGY_LOCATION_STATE
          : undefined;
  const backLabel = fromZombies
    ? t("topology.backToZombies")
    : fromNaming
      ? t("topology.backToNaming")
      : fromGenome
        ? t("topology.backToGenome")
        : fromTopology
          ? t("topology.backToTopology")
          : t("consumers.backToStream", { name });
  const { canManageJetStream } = useAuth();
  const canManageJS = canManageJetStream(id);
  const queryClient = useQueryClient();
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [editOpen, setEditOpen] = useState(false);
  const [editError, setEditError] = useState("");
  const [editBusy, setEditBusy] = useState(false);

  const [replayMode, setReplayMode] = useState<"reset" | "sidecar">("reset");
  const [replayScope, setReplayScope] = useState<"continuous" | "one" | "range">("continuous");
  const [replayFrom, setReplayFrom] = useState<"seq" | "time" | "beginning" | "new">("seq");
  const [replaySeq, setReplaySeq] = useState("");
  const [replayUntilSeq, setReplayUntilSeq] = useState("");
  const [replayTime, setReplayTime] = useState("");
  const [replayUntilTime, setReplayUntilTime] = useState("");
  const [replayPolicy, setReplayPolicy] = useState<"instant" | "original">("instant");
  const [filterSubject, setFilterSubject] = useState("");
  const [sidecarDurable, setSidecarDurable] = useState("");
  const [replaying, setReplaying] = useState(false);
  const [createdDurable, setCreatedDurable] = useState<string | null>(null);
  const [replayConfirmBody, setReplayConfirmBody] = useState<ReplayConsumerRequest | null>(null);
  const seededReplayRef = useRef(false);

  const detailQueryKey = clusterQueryKey(id, `consumer-detail:${name}:${consumer}`);
  const detailQuery = useQuery({
    queryKey: detailQueryKey,
    queryFn: async (): Promise<ConsumerDetailData> => {
      const [data, stream] = await Promise.all([
        api<ConsumerInfo>(
          clusterPath(id!, `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}`),
        ),
        api<StreamInfo>(clusterPath(id!, `/streams/${encodeURIComponent(name)}`)),
      ]);
      return { info: data.data, streamLastSeq: stream.data.state.lastSeq };
    },
    enabled: Boolean(id && name && consumer),
    refetchInterval: visibilityAwareInterval(STREAM_STATE_POLL_MS),
  });

  const dryRunQuery = useQuery({
    queryKey: clusterQueryKey(
      id,
      `replay-dry-run:${name}:${consumer}:${replayConfirmBody ? JSON.stringify(replayConfirmBody) : ""}`,
    ),
    queryFn: async () =>
      (
        await api<ReplayDryRun>(
          clusterPath(
            id!,
            `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}/replay/dry-run`,
          ),
          { method: "POST", body: JSON.stringify(replayConfirmBody) },
        )
      ).data,
    enabled: Boolean(id && name && consumer && replayConfirmBody),
  });

  const fingerprintQuery = useQuery({
    queryKey: clusterQueryKey(id, `behavior-fingerprint:${name}:${consumer}`),
    queryFn: async () =>
      (
        await api<BehaviorFingerprintReport>(
          clusterPath(
            id!,
            `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}/behavior-fingerprint`,
          ),
        )
      ).data,
    enabled: Boolean(id && name && consumer),
    refetchInterval: visibilityAwareInterval(12_000),
  });

  const info = detailQuery.data?.info ?? null;
  const streamLastSeq = detailQuery.data?.streamLastSeq ?? 0;
  const fingerprint = fingerprintQuery.data ?? null;
  const fingerprintAnomaly = Boolean(fingerprint?.available && fingerprint.anomaly);

  const bottlenecksQuery = useQuery({
    queryKey: clusterQueryKey(id, "hidden-bottlenecks"),
    queryFn: () => fetchHiddenBottlenecks(id!),
    enabled: Boolean(id && name && consumer),
    staleTime: 60_000,
  });
  const hasHiddenBottleneck = findingMatchesConsumer(
    bottlenecksQuery.data?.findings ?? [],
    name,
    consumer,
  );

  useEffect(() => {
    seededReplayRef.current = false;
    setError("");
    setSuccess("");
    setEditOpen(false);
    setEditError("");
    setReplayMode("reset");
    setReplayScope("continuous");
    setReplayFrom("seq");
    setReplaySeq("");
    setReplayUntilSeq("");
    setReplayTime("");
    setReplayUntilTime("");
    setReplayPolicy("instant");
    setFilterSubject("");
    setSidecarDurable("");
    setReplaying(false);
    setCreatedDurable(null);
    setReplayConfirmBody(null);
  }, [id, name, consumer]);

  useEffect(() => {
    if (!info || seededReplayRef.current) return;
    seededReplayRef.current = true;
    const hint = info.ackFloor?.streamSeq || info.delivered?.streamSeq;
    if (hint) {
      setReplaySeq(String(hint));
    }
  }, [info]);

  function deleteConsumer() {
    if (!id) return;
    askConfirm({
      title: t("streams.confirmDeleteConsumerTitle"),
      description: t("streams.confirmDeleteConsumer", { name: consumer }),
      action: async () => {
        try {
          await api(
            clusterPath(id, `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}`),
            { method: "DELETE" },
          );
          await invalidateJetStreamTopology(id);
          window.location.href = fromZombies
            ? ZOMBIES_TOPOLOGY_HREF
            : fromNaming
              ? NAMING_TOPOLOGY_HREF
              : fromTopology
                ? "/admin/topology"
                : streamHref;
        } catch (err) {
          setError(err instanceof Error ? err.message : "Failed to delete consumer");
        }
      },
    });
  }

  async function executeReplay(body: ReplayConsumerRequest) {
    setReplaying(true);
    setError("");
    setSuccess("");
    setCreatedDurable(null);
    try {
      const result = (
        await api<ReplayConsumerResult>(
          clusterPath(
            id!,
            `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}/replay`,
          ),
          { method: "POST", body: JSON.stringify(body) },
        )
      ).data;
      const bound =
        result.untilSeq || result.limit
          ? ` Bound untilSeq=${result.untilSeq ?? "—"} limit=${result.limit ?? "—"}.`
          : "";
      setSuccess(
        (result.mode === "sidecar"
          ? `Created side-car durable "${result.durable}".`
          : `Reset durable "${result.durable}".`) + bound,
      );
      if (result.mode === "sidecar") {
        setCreatedDurable(result.durable);
      }
      await queryClient.invalidateQueries({ queryKey: detailQueryKey });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to replay consumer");
    } finally {
      setReplaying(false);
      setReplayConfirmBody(null);
    }
  }

  function buildReplayBody(): ReplayConsumerRequest | null {
    const selector = replayScope === "continuous" ? replayFrom : replayFrom === "time" ? "time" : "seq";
    if (replayScope !== "continuous" && (selector === "beginning" || selector === "new")) {
      setError("One message and range require sequence or time");
      return null;
    }

    const body: ReplayConsumerRequest = {
      mode: replayMode,
      from: selector === "time" ? "time" : selector === "beginning" || selector === "new" ? selector : "seq",
      replayPolicy,
    };

    if (body.from === "seq") {
      const seq = Number(replaySeq);
      if (!Number.isFinite(seq) || seq < 1) {
        setError("Sequence is required");
        return null;
      }
      body.seq = seq;
      if (replayScope === "one") {
        body.untilSeq = seq;
        body.limit = 1;
      } else if (replayScope === "range") {
        const until = Number(replayUntilSeq);
        if (!Number.isFinite(until) || until < seq) {
          setError("Until sequence must be >= start sequence");
          return null;
        }
        body.untilSeq = until;
        body.limit = until - seq + 1;
      } else if (replayUntilSeq.trim()) {
        const until = Number(replayUntilSeq);
        if (!Number.isFinite(until) || until < seq) {
          setError("Until sequence must be >= start sequence");
          return null;
        }
        body.untilSeq = until;
      }
    }

    if (body.from === "time") {
      if (!replayTime.trim()) {
        setError("Start time is required");
        return null;
      }
      body.time = new Date(replayTime).toISOString();
      if (replayScope === "one") {
        body.limit = 1;
      } else if (replayScope === "range") {
        if (!replayUntilTime.trim()) {
          setError("Until time is required for a time range");
          return null;
        }
        body.untilTime = new Date(replayUntilTime).toISOString();
      } else if (replayUntilTime.trim()) {
        body.untilTime = new Date(replayUntilTime).toISOString();
      }
    }

    if (filterSubject.trim()) {
      body.filterSubject = filterSubject.trim();
    }
    if (replayMode === "sidecar" && sidecarDurable.trim()) {
      body.durable = sidecarDurable.trim();
    }

    return body;
  }

  async function submitReplay(event: FormEvent) {
    event.preventDefault();
    if (!id) return;

    const body = buildReplayBody();
    if (!body) return;

    setReplayConfirmBody(body);
  }

  async function confirmReplay() {
    if (!replayConfirmBody) return;
    await executeReplay(replayConfirmBody);
  }

  async function saveConsumerConfig(body: ConsumerConfigPayload) {
    if (!id) return;
    setEditBusy(true);
    setEditError("");
    try {
      const updated = (await api<ConsumerInfo>(
        clusterPath(id, `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}`),
        {
          method: "PUT",
          body: JSON.stringify({ ...body, durableName: consumer }),
        },
      )).data;
      queryClient.setQueryData(detailQueryKey, (prev: ConsumerDetailData | undefined) =>
        prev ? { ...prev, info: updated } : { info: updated, streamLastSeq },
      );
      setEditOpen(false);
      await invalidateJetStreamTopology(id);
    } catch (err) {
      setEditError(err instanceof Error ? err.message : "Failed to update consumer");
      throw err;
    } finally {
      setEditBusy(false);
    }
  }

  if (!id) {
    return <p className="text-muted">Select a cluster to view this consumer.</p>;
  }

  if (!info) {
    if (detailQuery.isError) {
      return (
        <QueryErrorState error={detailQuery.error} onRetry={() => void detailQuery.refetch()} />
      );
    }
    return <div>{error || t("common.loading")}</div>;
  }

  const replayConfirmOpen = Boolean(replayConfirmBody);
  const replayConfirmIsSidecar = replayConfirmBody?.mode === "sidecar";

  return (
    <div>
      {confirmDialog}
      <ConfirmDialog
        open={replayConfirmOpen}
        title={
          replayConfirmIsSidecar
            ? t("streams.confirmSidecarReplayTitle")
            : t("streams.confirmResetConsumerTitle")
        }
        description={
          <>
            <p>
              {replayConfirmIsSidecar
                ? t("streams.confirmSidecarReplay", { name: consumer })
                : t("streams.confirmResetConsumer", { name: consumer })}
            </p>
            <Suspense fallback={null}>
              <ReplayDryRunPanel
                data={dryRunQuery.data}
                loading={dryRunQuery.isFetching}
                error={
                  dryRunQuery.error instanceof Error
                    ? dryRunQuery.error.message
                    : dryRunQuery.isError
                      ? "error"
                      : null
                }
              />
            </Suspense>
          </>
        }
        confirmLabel={replayConfirmIsSidecar ? t("consumer.replay") : t("streams.reset")}
        busy={replaying}
        onCancel={() => {
          if (!replaying) setReplayConfirmBody(null);
        }}
        onConfirm={() => void confirmReplay()}
      />
      <div className="page-header">
        <div>
          <Link to={backHref} className="link-back" state={backState}>
            {backLabel}
          </Link>
          <h1>
            {info.name}
            {info.slowConsumer ? (
              <span className="topology-detail__chip topology-detail__chip--warn" style={{ marginLeft: "0.5rem" }}>
                {t("consumers.slowConsumer")}
              </span>
            ) : null}
            {fingerprintAnomaly ? (
              <span className="topology-detail__chip topology-detail__chip--warn" style={{ marginLeft: "0.5rem" }}>
                {t("consumers.fingerprintChip")}
              </span>
            ) : null}
            {hasHiddenBottleneck ? (
              <Link
                to={`${HIDDEN_BOTTLENECKS_HREF}?consumer=${encodeURIComponent(consumer)}`}
                className="topology-detail__chip topology-detail__chip--warn"
                style={{ marginLeft: "0.5rem", textDecoration: "none" }}
              >
                {t("hiddenBottlenecks.chip")}
              </Link>
            ) : null}
          </h1>
          {info.slowConsumer && info.slowReasons?.length ? (
            <p className="text-muted">{t("consumers.slowReasons", { reasons: info.slowReasons.join(", ") })}</p>
          ) : null}
          {info.config.description ? <p className="text-muted">{info.config.description}</p> : null}
        </div>
        {canManageJS && (
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

      {detailQuery.isError && !detailQuery.data && (
        <QueryErrorState error={detailQuery.error} onRetry={() => void detailQuery.refetch()} />
      )}
      {error && <Alert variant="error">{error}</Alert>}
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
          <div className="card-label">{t("streams.lag")}</div>
          <div className="card-value">
            {consumerLag(streamLastSeq, info.delivered?.streamSeq)}
          </div>
        </div>
        <div className="card">
          <div className="card-label">{t("streams.pending")}</div>
          <div className="card-value">{info.numPending}</div>
        </div>
        <div className="card">
          <div className="card-label">{t("streams.ackPending")}</div>
          <div className="card-value">{info.numAckPending}</div>
        </div>
        <div className="card">
          <div className="card-label">{t("streams.waiting")}</div>
          <div className="card-value">{info.numWaiting ?? 0}</div>
        </div>
        <div className="card">
          <div className="card-label">{t("streams.redelivered")}</div>
          <div className="card-value">{info.numRedelivered ?? 0}</div>
        </div>
        <div className="card">
          <div className="card-label">{t("streams.lastSeq")}</div>
          <div className="card-value">{streamLastSeq}</div>
        </div>
        <div className="card">
          <div className="card-label">{t("streams.deliveredSeq")}</div>
          <div className="card-value card-value--sm">
            {formatSeqPair(info.delivered?.streamSeq, info.delivered?.consumerSeq)}
          </div>
        </div>
        <div className="card">
          <div className="card-label">{t("streams.ackFloor")}</div>
          <div className="card-value card-value--sm">
            {formatSeqPair(info.ackFloor?.streamSeq, info.ackFloor?.consumerSeq)}
          </div>
        </div>
        <div className="card">
          <div className="card-label">{t("streams.ackWait")}</div>
          <div className="card-value card-value--sm">{formatAckWaitNs(info.config.ackWaitNs)}</div>
        </div>
        <div className="card">
          <div className="card-label">{t("streams.deliverPolicy")}</div>
          <div className="card-value card-value--sm">{info.config.deliverPolicy}</div>
        </div>
        <div className="card">
          <div className="card-label">{t("streams.ackPolicy")}</div>
          <div className="card-value card-value--sm">{info.config.ackPolicy}</div>
        </div>
      </div>

      <div className="mt-32">
        <Suspense fallback={null}>
          <BehaviorFingerprintPanel
            durable={info.name}
            data={fingerprint}
            loading={fingerprintQuery.isLoading}
            error={
              fingerprintQuery.isError
                ? fingerprintQuery.error instanceof Error
                  ? fingerprintQuery.error.message
                  : "error"
                : null
            }
          />
        </Suspense>
      </div>

      <div className="mt-32">
        <Suspense fallback={null}>
          <IncidentCapsulePanel
            clusterId={id!}
            streamName={name}
            consumer={info.name}
            canManage={canManageJS}
          />
        </Suspense>
      </div>

      <div
        className={
          canManageJS
            ? "consumer-replay-config mt-32"
            : "consumer-replay-config consumer-replay-config--solo mt-32"
        }
      >
        {canManageJS && (
          <section className="consumer-replay-config__panel">
            <h2 className="consumer-replay-config__title">Replay</h2>
            <form className="form-grid card consumer-replay-config__card" onSubmit={submitReplay}>
              <label>
                Mode
                <Seg role="group" aria-label="Mode">
                  <SegBtn
                    aria-pressed={replayMode === "reset"}
                    hint="Delete and recreate this durable at a new start position. Live consumers using this name will see the new cursor."
                    onClick={() => setReplayMode("reset")}
                  >
                    Reset this consumer
                  </SegBtn>
                  <SegBtn
                    aria-pressed={replayMode === "sidecar"}
                    hint="Create a separate backfill durable. The live consumer is left untouched."
                    onClick={() => setReplayMode("sidecar")}
                  >
                    Side-car durable
                  </SegBtn>
                </Seg>
              </label>
              <label>
                Scope
                <Seg role="group" aria-label="Scope">
                  {(
                    [
                      [
                        "continuous",
                        "Continuous",
                        "Deliver from the start point onward with no end bound (optional until fields still record intent).",
                      ],
                      [
                        "one",
                        "One message",
                        "Target exactly one stored message (by sequence, or the first at/after a timestamp).",
                      ],
                      [
                        "range",
                        "Range",
                        "Closed inclusive window by sequence or time. Bound is recorded for clients; JetStream does not stop by itself.",
                      ],
                    ] as const
                  ).map(([value, label, hint]) => (
                    <SegBtn
                      key={value}
                      aria-pressed={replayScope === value}
                      hint={hint}
                      onClick={() => {
                        setReplayScope(value);
                        if (value !== "continuous" && (replayFrom === "beginning" || replayFrom === "new")) {
                          setReplayFrom("seq");
                        }
                      }}
                    >
                      {label}
                    </SegBtn>
                  ))}
                </Seg>
              </label>
              <label>
                By
                <Seg role="group" aria-label="By">
                  {(
                    replayScope === "continuous"
                      ? ([
                          ["seq", "Sequence", "Start at a stream sequence number."],
                          ["time", "Time", "Start at the first message at or after this timestamp."],
                          ["beginning", "Beginning", "Deliver from the first retained message in the stream."],
                          ["new", "New", "Only messages published after the consumer is (re)created."],
                        ] as const)
                      : ([
                          ["seq", "Sequence", "Select the message(s) by stream sequence."],
                          ["time", "Time", "Select the message(s) by timestamp window."],
                        ] as const)
                  ).map(([value, label, hint]) => (
                    <SegBtn
                      key={value}
                      aria-pressed={
                        (replayScope === "continuous" ? replayFrom : replayFrom === "time" ? "time" : "seq") ===
                        value
                      }
                      hint={hint}
                      onClick={() => setReplayFrom(value)}
                    >
                      {label}
                    </SegBtn>
                  ))}
                </Seg>
              </label>
              {(replayScope !== "continuous" ? replayFrom !== "time" : replayFrom === "seq") && (
                <label>
                  {replayScope === "range" || replayScope === "continuous" ? "Start Sequence" : "Sequence"}
                  <input
                    type="number"
                    min={1}
                    value={replaySeq}
                    onChange={(e) => setReplaySeq(e.target.value)}
                    required
                  />
                </label>
              )}
              {(replayScope === "range" || replayScope === "continuous") &&
                (replayScope === "range" ? replayFrom !== "time" : replayFrom === "seq") && (
                <label>
                  Until Sequence {replayScope === "continuous" ? "(optional)" : ""}
                  <input
                    type="number"
                    min={1}
                    value={replayUntilSeq}
                    onChange={(e) => setReplayUntilSeq(e.target.value)}
                    required={replayScope === "range"}
                  />
                </label>
              )}
              {replayFrom === "time" && (
                <label>
                  {replayScope === "range" || replayScope === "continuous" ? "Start Time" : "Time"}
                  <input
                    type="datetime-local"
                    value={replayTime}
                    onChange={(e) => setReplayTime(e.target.value)}
                    required
                  />
                </label>
              )}
              {(replayScope === "range" || replayScope === "continuous") && replayFrom === "time" && (
                <label>
                  Until Time {replayScope === "continuous" ? "(optional)" : ""}
                  <input
                    type="datetime-local"
                    value={replayUntilTime}
                    onChange={(e) => setReplayUntilTime(e.target.value)}
                    required={replayScope === "range"}
                  />
                </label>
              )}
              <label>
                Replay Policy
                <Seg role="group" aria-label="Replay Policy">
                  <SegBtn
                    aria-pressed={replayPolicy === "instant"}
                    hint="Deliver historical messages as fast as the client can consume them."
                    onClick={() => setReplayPolicy("instant")}
                  >
                    Instant
                  </SegBtn>
                  <SegBtn
                    aria-pressed={replayPolicy === "original"}
                    hint="Pace delivery using the original publish timestamps between messages."
                    onClick={() => setReplayPolicy("original")}
                  >
                    Original
                  </SegBtn>
                </Seg>
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
              <div className="actions">
                <button className="btn" type="submit" disabled={replaying}>
                  {replaying ? "Replaying…" : "Replay"}
                </button>
              </div>
            </form>
          </section>
        )}

        <section className="consumer-replay-config__panel">
          <h2 className="consumer-replay-config__title">Configuration</h2>
          <div className="card consumer-replay-config__card">
            <dl className="stream-meta-list stream-meta-list--compact">
              {(
                [
                  ["Durable", info.config.durableName || info.name],
                  ["Description", info.config.description || "—"],
                  ["Deliver policy", info.config.deliverPolicy],
                  ["Ack policy", info.config.ackPolicy],
                  ["Replay policy", info.config.replayPolicy || "instant"],
                  [
                    "Filter",
                    info.config.filterSubjects?.length
                      ? info.config.filterSubjects.join(", ")
                      : info.config.filterSubject || "—",
                  ],
                  ["Start sequence", info.config.optStartSeq ? String(info.config.optStartSeq) : "—"],
                  ["Start time", info.config.optStartTime || "—"],
                  ["Ack wait", formatAckWaitNs(info.config.ackWaitNs)],
                  ["Max deliver", info.config.maxDeliver != null ? String(info.config.maxDeliver) : "—"],
                  [
                    "Max ack pending",
                    info.config.maxAckPending != null ? String(info.config.maxAckPending) : "—",
                  ],
                  ["Max waiting", info.config.maxWaiting != null ? String(info.config.maxWaiting) : "—"],
                  [
                    "Inactive threshold",
                    formatAckWaitNs(info.config.inactiveThresholdNs),
                  ],
                  ["Replicas", info.config.replicas != null ? String(info.config.replicas) : "—"],
                  ["Memory storage", info.config.memoryStorage ? "yes" : "no"],
                  ["Flow control", info.config.flowControl ? "yes" : "no"],
                  ["Headers only", info.config.headersOnly ? "yes" : "no"],
                  ["Deliver subject", info.config.deliverSubject || "—"],
                  ["Deliver group", info.config.deliverGroup || "—"],
                ] as const
              ).map(([label, value]) => (
                <div className="stream-meta-list__row" key={label}>
                  <dt>{label}</dt>
                  <dd className={typeof value === "string" && value.includes(".") ? "mono" : undefined}>
                    {value}
                  </dd>
                </div>
              ))}
              {info.config.metadata && Object.keys(info.config.metadata).length > 0 && (
                <div className="stream-meta-list__row">
                  <dt>Metadata</dt>
                  <dd className="mono">
                    {Object.entries(info.config.metadata)
                      .map(([k, v]) => `${k}=${v}`)
                      .join(", ")}
                  </dd>
                </div>
              )}
            </dl>
          </div>
        </section>
      </div>
    </div>
  );
}
