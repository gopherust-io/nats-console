import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import CreateConsumerPanel, { ConsumerConfigPayload } from "../components/CreateConsumerPanel";
import CreateStreamPanel, { StreamConfigPayload } from "../components/CreateStreamPanel";
import MessagePayloadViewer from "../components/MessagePayloadViewer";
import { payloadByteLength } from "../lib/payloadBytes";
import Pager, { DEFAULT_PAGE_SIZE, pageQuery } from "../components/Pager";
import VirtualTable from "../components/VirtualTable";
import Alert from "../components/ui/Alert";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import PageLoader from "../components/ui/PageLoader";
import StatCard from "../components/ui/StatCard";
import {
  api,
  clusterPath,
  ConsumerInfo,
  jetStreamUIBase,
  RawMessage,
  StreamInfo,
  tryParseJSON,
} from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { formatDateTime } from "../lib/datetime";
import { clusterQueryKey, invalidateJetStreamTopology } from "../lib/query";
import { isFromTopology } from "../lib/topology";

type ConsumerListResponse = {
  consumers: ConsumerInfo[];
  total: number;
  offset: number;
  limit: number;
};

type StreamTab = "overview" | "consumers" | "messages";

function parseTab(raw: string | null): StreamTab {
  if (raw === "consumers" || raw === "messages" || raw === "overview") return raw;
  return "overview";
}

export default function StreamDetailPage() {
  const { t } = useTranslation();
  const { name = "", clusterId: routeCluster, accountName } = useParams();
  const { clusterId } = useCluster();
  const id = routeCluster ?? clusterId;
  const jsBase = id ? jetStreamUIBase(id, accountName) : "";
  const hubHref = jsBase || "/systems";
  const streamHref = jsBase ? `${jsBase}/streams/${encodeURIComponent(name)}` : "/systems";
  const location = useLocation();
  const fromTopology = isFromTopology(location.state);
  const backHref = fromTopology ? "/admin/topology" : hubHref;
  const { canManageJetStream } = useAuth();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const tab = parseTab(searchParams.get("tab"));

  const [stream, setStream] = useState<StreamInfo | null>(null);
  const [consumerOffset, setConsumerOffset] = useState(0);
  const [seq, setSeq] = useState("");
  const [message, setMessage] = useState<RawMessage | null>(null);
  const [messageLoading, setMessageLoading] = useState(false);
  const [publishSubject, setPublishSubject] = useState("");
  const [publishPayload, setPublishPayload] = useState('{"hello":"world"}');
  const [publishRawMode, setPublishRawMode] = useState(false);
  const [error, setError] = useState("");
  const [editOpen, setEditOpen] = useState(false);
  const [editError, setEditError] = useState("");
  const [editBusy, setEditBusy] = useState(false);
  const [consumerPanelOpen, setConsumerPanelOpen] = useState(false);
  const [consumerPanelError, setConsumerPanelError] = useState("");
  const [consumerPanelBusy, setConsumerPanelBusy] = useState(false);
  const autoLoadedRef = useRef(false);
  const limit = DEFAULT_PAGE_SIZE;

  const setTab = useCallback(
    (next: StreamTab) => {
      setSearchParams(
        (prev) => {
          const params = new URLSearchParams(prev);
          if (next === "overview") params.delete("tab");
          else params.set("tab", next);
          return params;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  const loadMessage = useCallback(
    async (targetSeq?: string, direction?: "next" | "prev") => {
      if (!id) return;
      const currentSeq = targetSeq ?? seq;
      if (!currentSeq) return;
      setMessageLoading(true);
      try {
        let url = clusterPath(
          id,
          `/streams/${encodeURIComponent(name)}/messages?seq=${encodeURIComponent(currentSeq)}`,
        );
        if (direction) url += `&direction=${direction}`;
        const data = await api<RawMessage>(url);
        setMessage(data);
        setSeq(String(data.message.seq));
        setError("");
      } catch (err) {
        setError(err instanceof Error ? err.message : t("streams.loadMessageFailed"));
      } finally {
        setMessageLoading(false);
      }
    },
    [id, name, seq, t],
  );

  const loadMessageRef = useRef(loadMessage);
  loadMessageRef.current = loadMessage;

  useEffect(() => {
    if (!id || !name) return;
    autoLoadedRef.current = false;
    setMessage(null);
    api<StreamInfo>(clusterPath(id, `/streams/${encodeURIComponent(name)}`))
      .then((streamInfo) => {
        setStream(streamInfo);
        setPublishSubject(
          streamInfo.config.subjects?.find((s) => !s.includes("*") && !s.includes(">")) ??
            streamInfo.config.subjects?.[0] ??
            "",
        );
        const last = streamInfo.state.lastSeq;
        if (last > 0) {
          setSeq(String(last));
        } else {
          setSeq("");
        }
      })
      .catch((err: Error) => setError(err.message));
  }, [id, name]);

  useEffect(() => {
    if (!stream || autoLoadedRef.current) return;
    if (stream.state.lastSeq <= 0) return;
    autoLoadedRef.current = true;
    void loadMessageRef.current(String(stream.state.lastSeq));
  }, [stream]);

  const consumersQuery = useQuery({
    queryKey: [...clusterQueryKey(id, `consumers:${name}`), consumerOffset],
    queryFn: () =>
      api<ConsumerListResponse>(
        clusterPath(id!, `/streams/${encodeURIComponent(name)}/consumers${pageQuery(consumerOffset, limit)}`),
      ),
    enabled: Boolean(id && name),
  });

  const consumers = consumersQuery.data?.consumers ?? [];
  const consumerTotal = consumersQuery.data?.total ?? 0;
  const consumersError =
    consumersQuery.error instanceof Error ? consumersQuery.error.message : "";

  async function purgeStream() {
    if (!id || !confirm(t("streams.confirmPurge", { name }))) return;
    try {
      await api(clusterPath(id, `/streams/${encodeURIComponent(name)}/purge`), { method: "POST" });
      const updated = await api<StreamInfo>(clusterPath(id, `/streams/${encodeURIComponent(name)}`));
      setStream(updated);
      setMessage(null);
      autoLoadedRef.current = false;
    } catch (err) {
      setError(err instanceof Error ? err.message : t("streams.purgeFailed"));
    }
  }

  async function deleteStream() {
    if (!id || !confirm(t("streams.confirmDelete", { name }))) return;
    try {
      await api(clusterPath(id, `/streams/${encodeURIComponent(name)}`), { method: "DELETE" });
      await invalidateJetStreamTopology(id);
      navigate(fromTopology ? "/admin/topology" : hubHref);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("streams.deleteFailed"));
    }
  }

  async function saveStreamConfig(body: StreamConfigPayload) {
    if (!id) return;
    setEditBusy(true);
    setEditError("");
    try {
      const updated = await api<StreamInfo>(clusterPath(id, `/streams/${encodeURIComponent(name)}`), {
        method: "PUT",
        body: JSON.stringify({ ...body, name }),
      });
      setStream(updated);
      setEditOpen(false);
      await invalidateJetStreamTopology(id);
    } catch (err) {
      setEditError(err instanceof Error ? err.message : "Failed to update stream");
      throw err;
    } finally {
      setEditBusy(false);
    }
  }

  async function createConsumer(body: ConsumerConfigPayload) {
    if (!id) return;
    setConsumerPanelBusy(true);
    setConsumerPanelError("");
    try {
      await api(clusterPath(id, `/streams/${encodeURIComponent(name)}/consumers`), {
        method: "POST",
        body: JSON.stringify(body),
      });
      setConsumerPanelOpen(false);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: clusterQueryKey(id, `consumers:${name}`) }),
        invalidateJetStreamTopology(id),
      ]);
    } catch (err) {
      setConsumerPanelError(err instanceof Error ? err.message : "Failed to create consumer");
      throw err;
    } finally {
      setConsumerPanelBusy(false);
    }
  }

  async function publishMessage(event: FormEvent) {
    event.preventDefault();
    if (!id) return;
    try {
      const body =
        publishRawMode || !tryParseJSON(publishPayload).isJSON
          ? publishPayload
          : JSON.stringify(tryParseJSON(publishPayload).parsed);
      const data = btoa(unescape(encodeURIComponent(body)));
      const result = await api<{ seq: number }>(
        clusterPath(id, `/streams/${encodeURIComponent(name)}/messages`),
        {
          method: "POST",
          body: JSON.stringify({
            subject: publishSubject,
            data,
          }),
        },
      );
      const updated = await api<StreamInfo>(clusterPath(id, `/streams/${encodeURIComponent(name)}`));
      setStream(updated);
      setSeq(String(result.seq));
      await loadMessage(String(result.seq));
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : t("streams.publishFailed"));
    }
  }

  if (!id) {
    return <p className="text-muted">{t("streams.selectCluster")}</p>;
  }

  if (!stream) {
    if (error) return <Alert variant="error">{error}</Alert>;
    return <PageLoader />;
  }

  const firstSeq = stream.state.firstSeq;
  const lastSeq = stream.state.lastSeq;
  const hasMessages = lastSeq > 0 && stream.state.messages > 0;
  const sizeBytes = message ? payloadByteLength(message.message.data) : 0;

  return (
    <div className="page">
      <PageHeader
        eyebrow={t("streams.detailEyebrow")}
        title={stream.config.name}
        subtitle={stream.config.description || stream.config.subjects?.join(", ")}
        actions={
          <div className="actions">
            <Link className="btn secondary" to={`${streamHref}/live`}>
              {t("streams.liveTail")}
            </Link>
            {canManageJetStream && (
              <>
                <button
                  className="btn secondary"
                  type="button"
                  onClick={() => {
                    setEditError("");
                    setEditOpen(true);
                  }}
                >
                  {t("jetstream.editConfig")}
                </button>
                <button className="btn secondary" type="button" onClick={purgeStream}>
                  {t("streams.purgeStream")}
                </button>
                <button className="btn danger" type="button" onClick={deleteStream}>
                  {t("streams.deleteStream")}
                </button>
              </>
            )}
          </div>
        }
      />

      <p className="mb-12">
        <Link to={backHref} className="link-back" state={fromTopology ? { from: "topology" } : undefined}>
          {fromTopology ? t("topology.backToTopology") : t("streams.backToStreams")}
        </Link>
      </p>

      <Alert variant="error">{error || consumersError}</Alert>

      <CreateStreamPanel
        mode="edit"
        open={editOpen}
        initial={stream.config}
        busy={editBusy}
        error={editError}
        onClose={() => {
          setEditOpen(false);
          setEditError("");
        }}
        onSubmit={saveStreamConfig}
      />

      <nav className="nc-tabs stream-tabs" aria-label="Stream sections">
        {(
          [
            ["overview", t("streams.tabOverview")],
            ["consumers", t("streams.tabConsumers")],
            ["messages", t("streams.tabMessages")],
          ] as const
        ).map(([idTab, label]) => (
          <button
            key={idTab}
            type="button"
            className={`nc-tab${tab === idTab ? " active" : ""}`}
            aria-current={tab === idTab ? "page" : undefined}
            onClick={() => setTab(idTab)}
          >
            {label}
          </button>
        ))}
      </nav>

      {tab === "overview" && (
        <>
          <div className="card-grid">
            <StatCard label={t("streams.messagesCount")} value={stream.state.messages} accent="emerald" />
            <StatCard
              label={t("streams.firstLastSeq")}
              value={`${firstSeq} / ${lastSeq}`}
              accent="sky"
            />
            <StatCard label={t("streams.retention")} value={stream.config.retention} accent="violet" />
            <StatCard label={t("streams.storage")} value={stream.config.storage} accent="amber" />
          </div>
          <dl className="stream-meta-list card">
            <div className="stream-meta-list__row">
              <dt>{t("streams.subjects")}</dt>
              <dd className="mono">{stream.config.subjects?.join(", ") || "—"}</dd>
            </div>
            <div className="stream-meta-list__row">
              <dt>{t("streams.bytes")}</dt>
              <dd>{stream.state.bytes.toLocaleString()}</dd>
            </div>
            <div className="stream-meta-list__row">
              <dt>{t("streams.consumerCount")}</dt>
              <dd>{stream.state.consumerCount}</dd>
            </div>
          </dl>
        </>
      )}

      {tab === "consumers" && (
        <>
          <div className="section-header" style={{ marginTop: 0 }}>
            <h2>{t("streams.tabConsumers")}</h2>
            {canManageJetStream && (
              <button
                className="btn"
                type="button"
                onClick={() => {
                  setConsumerPanelError("");
                  setConsumerPanelOpen(true);
                }}
              >
                {t("streams.createConsumer")}
              </button>
            )}
          </div>
          <p className="text-muted mb-12">{t("consumer.clientRecommend")}</p>

          <CreateConsumerPanel
            mode="create"
            open={consumerPanelOpen}
            stream={stream.config}
            busy={consumerPanelBusy}
            error={consumerPanelError}
            onClose={() => {
              setConsumerPanelOpen(false);
              setConsumerPanelError("");
            }}
            onSubmit={createConsumer}
          />

          <div className="table-wrap">
            <VirtualTable
              columns={[
                { id: "name", header: "Name", width: "minmax(140px, 1.3fr)" },
                { id: "deliver", header: t("streams.deliverPolicy"), width: "minmax(120px, 1fr)" },
                { id: "ack", header: t("streams.ackPolicy"), width: "minmax(120px, 1fr)" },
                { id: "pending", header: t("streams.pending"), width: "96px", align: "right" },
                { id: "ackPending", header: t("streams.ackPending"), width: "112px", align: "right" },
              ]}
              items={consumers}
              empty={t("streams.noConsumers")}
              getKey={(consumer) => consumer.name}
              renderCell={(consumer, columnId) => {
                switch (columnId) {
                  case "name":
                    return (
                      <Link to={`${streamHref}/consumers/${encodeURIComponent(consumer.name)}`}>
                        {consumer.name}
                      </Link>
                    );
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
        </>
      )}

      {tab === "messages" && (
        <>
          {canManageJetStream && (
            <form className="form-grid card mb-16" onSubmit={publishMessage}>
              <h3 className="section-title form-grid__full">{t("streams.publishMessage")}</h3>
              <label>
                {t("streams.subject")}
                <input
                  value={publishSubject}
                  onChange={(e) => setPublishSubject(e.target.value)}
                  placeholder={stream.config.subjects?.join(", ")}
                  required
                />
              </label>
              <label className="form-grid__full">
                {t("streams.payload")}
                <textarea
                  rows={6}
                  value={publishPayload}
                  onChange={(e) => setPublishPayload(e.target.value)}
                  placeholder='{"hello":"world"}'
                  required
                />
              </label>
              <div className="form-grid__full">
                <button
                  className="btn secondary"
                  type="button"
                  aria-pressed={publishRawMode}
                  onClick={() => setPublishRawMode((v) => !v)}
                >
                  {publishRawMode ? t("streams.jsonMode") : t("streams.rawMode")}
                </button>
                <button className="btn" type="submit">
                  {t("streams.publish")}
                </button>
              </div>
            </form>
          )}

          {!hasMessages ? (
            <EmptyState title={t("streams.noMessagesTitle")} description={t("streams.noMessagesDescription")} />
          ) : (
            <>
              <form
                className="message-nav"
                onSubmit={(event) => {
                  event.preventDefault();
                  void loadMessage();
                }}
              >
                <div className="message-nav__group" role="group" aria-label={t("streams.sequence")}>
                  <button
                    type="button"
                    className="btn secondary"
                    disabled={messageLoading || firstSeq <= 0}
                    onClick={() => void loadMessage(String(firstSeq))}
                  >
                    {t("streams.first")}
                  </button>
                  <button
                    type="button"
                    className="btn secondary"
                    disabled={messageLoading || !message?.navigation?.prevSeq}
                    onClick={() => void loadMessage(String(message?.message.seq), "prev")}
                  >
                    ← {t("streams.prev")}
                  </button>
                </div>

                <div className="message-nav__seq">
                  <label htmlFor="stream-message-seq">{t("streams.sequence")}</label>
                  <div className="message-nav__seq-row">
                    <input
                      id="stream-message-seq"
                      type="number"
                      min={1}
                      value={seq}
                      onChange={(e) => setSeq(e.target.value)}
                      placeholder="1"
                    />
                    <button className="btn" type="submit" disabled={messageLoading || !seq}>
                      {t("streams.load")}
                    </button>
                  </div>
                </div>

                <div className="message-nav__group" role="group" aria-label={t("streams.next")}>
                  <button
                    type="button"
                    className="btn secondary"
                    disabled={messageLoading || !message?.navigation?.nextSeq}
                    onClick={() => void loadMessage(String(message?.message.seq), "next")}
                  >
                    {t("streams.next")} →
                  </button>
                  <button
                    type="button"
                    className="btn secondary"
                    disabled={messageLoading || lastSeq <= 0}
                    onClick={() => void loadMessage(String(lastSeq))}
                  >
                    {t("streams.last")}
                  </button>
                </div>

                <span className="message-nav__range">
                  {t("streams.seqRange", { first: firstSeq, last: lastSeq })}
                </span>
              </form>

              {messageLoading && !message && <p className="text-muted">{t("streams.messageLoading")}</p>}

              {!message && !messageLoading && (
                <EmptyState title={t("streams.loadMessageHint")} />
              )}

              {message && (
                <article className="card message-viewer">
                  <header className="message-meta">
                    <div className="message-meta__item">
                      <span className="message-meta__label">{t("streams.seq")}</span>
                      <span className="message-meta__value">
                        <span className="message-meta__chip mono">#{message.message.seq}</span>
                      </span>
                    </div>
                    <div className="message-meta__item message-meta__item--grow">
                      <span className="message-meta__label">{t("streams.subject")}</span>
                      <span className="message-meta__value mono" title={message.message.subject}>
                        {message.message.subject}
                      </span>
                    </div>
                    <div className="message-meta__item">
                      <span className="message-meta__label">{t("streams.time")}</span>
                      <span className="message-meta__value">
                        <time dateTime={message.message.time} title={message.message.time}>
                          {formatDateTime(message.message.time)}
                        </time>
                      </span>
                    </div>
                    <div className="message-meta__item">
                      <span className="message-meta__label">{t("streams.size")}</span>
                      <span className="message-meta__value">
                        {t("streams.sizeBytes", { count: sizeBytes })}
                      </span>
                    </div>
                  </header>
                  <MessagePayloadViewer data={message.message.data} headers={message.message.headers} />
                </article>
              )}
            </>
          )}
        </>
      )}
    </div>
  );
}
