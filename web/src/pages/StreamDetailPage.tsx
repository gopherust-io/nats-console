import { FormEvent, KeyboardEvent, lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import CreateConsumerPanel, { ConsumerConfigPayload } from "../components/CreateConsumerPanel";
import CreateStreamPanel, { StreamConfigPayload } from "../components/CreateStreamPanel";
import BlastRadiusPanel from "../components/BlastRadiusPanel";
import DlqPanel from "../components/DlqPanel";
import LinedTextarea from "../components/LinedTextarea";
import MessageDownloadMenu from "../components/MessageDownloadMenu";
import MessageImportButton from "../components/MessageImportButton";
import MessagePayloadViewer from "../components/MessagePayloadViewer";
import StreamFavoriteButton from "../components/StreamFavoriteButton";
import TimeRangeSelector from "../components/metrics/TimeRangeSelector";
import { payloadByteLength } from "../lib/payloadBytes";
import { applyJsonTextareaKey } from "../lib/jsonTextarea";
import { locateJsonError } from "../lib/jsonError";
import {
  encodePublishPayload,
  type PublishFormat,
} from "../lib/publishEncode";
import {
  deletePayloadTemplate,
  readPayloadTemplates,
  savePayloadTemplate,
  templatesForStream,
  type PayloadTemplate,
} from "../lib/payloadTemplates";
import type { MessageImportItem } from "../lib/messageImport";
import Pager, { DEFAULT_PAGE_SIZE, pageQuery } from "../components/Pager";
import VirtualTable from "../components/VirtualTable";
import Alert from "../components/ui/Alert";
import ConfirmDialog from "../components/ui/ConfirmDialog";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import PageLoader from "../components/ui/PageLoader";
import QueryErrorState from "../components/ui/QueryErrorState";
import StatCard from "../components/ui/StatCard";
import JetStreamSectionTabs from "../components/JetStreamSectionTabs";
import {
  api,
  BlastRadius,
  clusterPath,
  ConsumerInfo,
  formatMessagePayload,
  jetStreamUIBase,
  RawMessage,
  StreamInfo,
  tryParseJSON,
} from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { consumerLag } from "../lib/consumerMetrics";
import { formatDateTime } from "../lib/datetime";
import { useClusterMetricsHistory } from "../hooks/useClusterMetricsHistory";
import { MetricsRangePreset } from "../lib/metricsHistory";
import { rowFromMessage } from "../lib/messageDownload";
import { decodeMessagePayload } from "../lib/messagePayloadDecode";
import { STREAM_STATE_POLL_MS } from "../lib/constants";
import { clusterQueryKey, invalidateJetStreamTopology, visibilityAwareInterval } from "../lib/query";
import { streamMetric, streamRateMetricsCSV, StreamMetricKind } from "../lib/streamMetrics";
import { isFromTopology, TOPOLOGY_LOCATION_STATE } from "../lib/topology";
import { isFromZombies, ZOMBIES_LOCATION_STATE, ZOMBIES_TOPOLOGY_HREF } from "../lib/zombie";
import { isFromNaming, NAMING_LOCATION_STATE, NAMING_TOPOLOGY_HREF } from "../lib/subjectNaming";
import { GENOME_LOCATION_STATE, GENOME_TOPOLOGY_HREF, isFromGenome } from "../lib/eventGenome";

const MetricsTimeSeriesChart = lazy(() => import("../components/metrics/MetricsTimeSeriesChart"));

type StreamTab = "overview" | "consumers" | "messages" | "dlq";

function parseTab(raw: string | null): StreamTab {
  if (raw === "consumers" || raw === "messages" || raw === "overview" || raw === "dlq") return raw;
  return "overview";
}

function isDlqStream(stream: StreamInfo | null): boolean {
  if (!stream) return false;
  if (stream.isDlq) return true;
  const name = stream.config.name ?? "";
  if (name.endsWith("_DLQ")) return true;
  return stream.config.metadata?.["nats-consol/role"] === "dlq";
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
          : hubHref;
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
          : null;
  const { canManageJetStream } = useAuth();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const tab = parseTab(searchParams.get("tab"));

  const [consumerOffset, setConsumerOffset] = useState(0);
  const [seq, setSeq] = useState("");
  const [message, setMessage] = useState<RawMessage | null>(null);
  const [messageLoading, setMessageLoading] = useState(false);
  const [publishSubject, setPublishSubject] = useState("");
  const [publishFormat, setPublishFormat] = useState<PublishFormat>("json");
  const [publishPayload, setPublishPayload] = useState('{\n  "hello": "world"\n}');
  const [publishBinaryPayload, setPublishBinaryPayload] = useState("");
  const [publishPayloadTouched, setPublishPayloadTouched] = useState(false);
  const [templateName, setTemplateName] = useState("");
  const [templates, setTemplates] = useState<PayloadTemplate[]>(() => readPayloadTemplates());
  const [templateHint, setTemplateHint] = useState("");
  const [error, setError] = useState("");
  const [editOpen, setEditOpen] = useState(false);
  const [editError, setEditError] = useState("");
  const [editBusy, setEditBusy] = useState(false);
  const [confirmAction, setConfirmAction] = useState<"delete" | "purge" | null>(null);
  const [confirmBusy, setConfirmBusy] = useState(false);
  const [consumerPanelOpen, setConsumerPanelOpen] = useState(false);
  const [consumerPanelError, setConsumerPanelError] = useState("");
  const [consumerPanelBusy, setConsumerPanelBusy] = useState(false);
  const [copyHint, setCopyHint] = useState("");
  const [metricsRange, setMetricsRange] = useState<MetricsRangePreset>("1h");
  const autoLoadedRef = useRef(false);
  const seededRef = useRef(false);
  const limit = DEFAULT_PAGE_SIZE;

  const rateMetrics = useMemo(() => (name ? streamRateMetricsCSV(name) : ""), [name]);
  const ratesHistory = useClusterMetricsHistory(id, metricsRange, rateMetrics, {
    enabled: tab === "overview",
  });

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
        const data = (await api<RawMessage>(url)).data;
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

  const streamQueryKey = clusterQueryKey(id, `stream:${name}`);
  const streamQuery = useQuery({
    queryKey: streamQueryKey,
    queryFn: async () => (await api<StreamInfo>(clusterPath(id!, `/streams/${encodeURIComponent(name)}`))).data,
    enabled: Boolean(id && name),
    refetchInterval: visibilityAwareInterval(STREAM_STATE_POLL_MS),
  });
  const stream = streamQuery.data ?? null;
  const showDlqTab = isDlqStream(stream);

  const impactQuery = useQuery({
    queryKey: clusterQueryKey(id, `stream-impact:${name}`),
    queryFn: async () => (await api<BlastRadius>(clusterPath(id!, `/streams/${encodeURIComponent(name)}/impact`))).data,
    enabled: Boolean(id && name && confirmAction === "delete"),
  });

  useEffect(() => {
    if (tab === "dlq" && stream && !showDlqTab) {
      setTab("overview");
    }
  }, [tab, stream, showDlqTab, setTab]);

  useEffect(() => {
    autoLoadedRef.current = false;
    seededRef.current = false;
    setMessage(null);
    setError("");
  }, [id, name]);

  useEffect(() => {
    if (!stream || seededRef.current) return;
    seededRef.current = true;
    setPublishSubject(
      stream.config.subjects?.find((s) => !s.includes("*") && !s.includes(">")) ??
        stream.config.subjects?.[0] ??
        "",
    );
    const last = stream.state.lastSeq;
    if (last > 0) {
      setSeq(String(last));
      if (!autoLoadedRef.current) {
        autoLoadedRef.current = true;
        void loadMessageRef.current(String(last));
      }
    } else {
      setSeq("");
    }
  }, [stream]);

  useEffect(() => {
    if (streamQuery.isError && !streamQuery.data) {
      // surface via QueryErrorState below; clear stale action errors
      setError("");
    }
  }, [streamQuery.isError, streamQuery.data]);

  const consumersQuery = useQuery({
    queryKey: [...clusterQueryKey(id, `consumers:${name}`), consumerOffset],
    queryFn: async () => {
      const r = await api<ConsumerInfo[]>(
        clusterPath(id!, `/streams/${encodeURIComponent(name)}/consumers${pageQuery(consumerOffset, limit)}`),
      );
      return { consumers: r.data ?? [], total: r.meta?.total ?? 0 };
    },
    enabled: Boolean(id && name),
    refetchInterval: visibilityAwareInterval(STREAM_STATE_POLL_MS),
  });

  const consumers = consumersQuery.data?.consumers ?? [];
  const consumerTotal = consumersQuery.data?.total ?? 0;

  async function purgeStream() {
    if (!id) return;
    setConfirmBusy(true);
    try {
      await api(clusterPath(id, `/streams/${encodeURIComponent(name)}/purge`), { method: "POST" });
      const updated = (await api<StreamInfo>(clusterPath(id, `/streams/${encodeURIComponent(name)}`))).data;
      queryClient.setQueryData(streamQueryKey, updated);
      setMessage(null);
      autoLoadedRef.current = false;
      seededRef.current = false;
      setConfirmAction(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("streams.purgeFailed"));
      setConfirmAction(null);
    } finally {
      setConfirmBusy(false);
    }
  }

  async function deleteStream() {
    if (!id) return;
    setConfirmBusy(true);
    try {
      await api(clusterPath(id, `/streams/${encodeURIComponent(name)}`), { method: "DELETE" });
      await invalidateJetStreamTopology(id);
      setConfirmAction(null);
      navigate(
        fromZombies
          ? ZOMBIES_TOPOLOGY_HREF
          : fromNaming
            ? NAMING_TOPOLOGY_HREF
            : fromTopology
              ? "/admin/topology"
              : hubHref,
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : t("streams.deleteFailed"));
      setConfirmAction(null);
    } finally {
      setConfirmBusy(false);
    }
  }

  async function saveStreamConfig(body: StreamConfigPayload) {
    if (!id) return;
    setEditBusy(true);
    setEditError("");
    try {
      const updated = (
        await api<StreamInfo>(clusterPath(id, `/streams/${encodeURIComponent(name)}`), {
          method: "PUT",
          body: JSON.stringify({ ...body, name }),
        })
      ).data;
      queryClient.setQueryData(streamQueryKey, updated);
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
    setPublishPayloadTouched(true);
    if (publishPayloadError) {
      setError(publishPayloadError);
      return;
    }
    try {
      const encoded = encodePublishPayload({
        format: publishFormat,
        jsonText: publishPayload,
        binaryText: publishBinaryPayload,
      });
      if (publishFormat === "json") {
        const { parsed, isJSON } = tryParseJSON(publishPayload);
        if (isJSON) {
          const formatted = JSON.stringify(parsed, null, 2);
          if (formatted !== publishPayload) setPublishPayload(formatted);
        }
      }
      const result = (
        await api<{ seq: number }>(
          clusterPath(id, `/streams/${encodeURIComponent(name)}/messages`),
          {
            method: "POST",
            body: JSON.stringify({
              subject: publishSubject,
              data: encoded.data,
              headers: { "Content-Type": encoded.contentType },
            }),
          },
        )
      ).data;
      const updated = (await api<StreamInfo>(clusterPath(id, `/streams/${encodeURIComponent(name)}`))).data;
      queryClient.setQueryData(streamQueryKey, updated);
      setSeq(String(result.seq));
      await loadMessage(String(result.seq));
      setError("");
    } catch (err) {
      if (err instanceof Error && err.message === "invalid-json") {
        setError(t("streams.publishInvalidJson"));
        return;
      }
      if (err instanceof Error && /base64|empty/i.test(err.message)) {
        setError(t("streams.publishInvalidBase64"));
        return;
      }
      setError(err instanceof Error ? err.message : t("streams.publishFailed"));
    }
  }

  const publishPayloadError = useMemo(() => {
    if (publishFormat === "msgpack" || publishFormat === "cbor" || publishFormat === "protobuf") {
      const trimmed = publishBinaryPayload.trim();
      if (!trimmed) return t("streams.publishInvalidBase64");
      try {
        encodePublishPayload({
          format: publishFormat,
          binaryText: publishBinaryPayload,
        });
        return "";
      } catch {
        return t("streams.publishInvalidBase64");
      }
    }

    const trimmed = publishPayload.trim();
    if (!trimmed) return t("streams.publishInvalidJson");
    const location = locateJsonError(publishPayload);
    if (!location) return "";
    return t("streams.publishInvalidJsonLocated", {
      line: location.line,
      column: location.column,
      detail: location.message,
    });
  }, [publishBinaryPayload, publishFormat, publishPayload, t]);

  const publishJsonErrorLocation = useMemo(() => {
    if (publishFormat !== "json" || !publishPayload.trim()) return null;
    return locateJsonError(publishPayload);
  }, [publishFormat, publishPayload]);

  const streamTemplates = useMemo(
    () => templatesForStream(id, name, templates),
    [id, name, templates],
  );

  function applyTemplate(template: PayloadTemplate) {
    setPublishFormat(template.format);
    setPublishSubject(template.subject ?? publishSubject);
    setPublishPayload(template.payload);
    setPublishBinaryPayload(template.binaryPayload ?? "");
    setPublishPayloadTouched(false);
    setTemplateHint("");
  }

  function onSaveTemplate() {
    const next = savePayloadTemplate({
      name: templateName || t("streams.templateNamePlaceholder"),
      subject: publishSubject,
      format: publishFormat,
      payload: publishPayload,
      binaryPayload: publishBinaryPayload,
      clusterId: id ?? undefined,
      stream: name || undefined,
    });
    setTemplates(next);
    setTemplateName("");
    setTemplateHint(t("streams.templateSaved"));
  }

  function onDeleteTemplate(templateId: string) {
    setTemplates(deletePayloadTemplate(templateId));
  }

  async function importMessages(items: MessageImportItem[]) {
    if (!id) return;
    let ok = 0;
    let failed = 0;
    let lastSeq: number | undefined;
    for (const item of items) {
      try {
        const result = (
          await api<{ seq: number }>(
            clusterPath(id, `/streams/${encodeURIComponent(name)}/messages`),
            {
              method: "POST",
              body: JSON.stringify({
                subject: item.subject,
                data: item.data,
                headers: item.headers,
              }),
            },
          )
        ).data;
        ok += 1;
        lastSeq = result.seq;
      } catch {
        failed += 1;
      }
    }
    const updated = (await api<StreamInfo>(clusterPath(id, `/streams/${encodeURIComponent(name)}`))).data;
    queryClient.setQueryData(streamQueryKey, updated);
    if (lastSeq != null) {
      setSeq(String(lastSeq));
      await loadMessage(String(lastSeq));
    }
    if (failed > 0) {
      setError(t("streams.importPartial", { ok, failed }));
    } else {
      setError("");
      setTemplateHint(t("streams.importSuccess", { count: ok }));
    }
    if (ok === 0 && failed > 0) {
      throw new Error(t("streams.importFailed"));
    }
  }

  async function publishQuickSample() {
    if (!id || !stream) return;
    const subject = publishSubject || stream.config.subjects?.[0] || "";
    if (!subject) {
      setError(t("streams.publishFailed"));
      return;
    }
    try {
      const sample = '{\n  "hello": "world"\n}';
      setPublishFormat("json");
      setPublishSubject(subject);
      setPublishPayload(sample);
      const encoded = encodePublishPayload({ format: "json", jsonText: sample });
      const result = (
        await api<{ seq: number }>(
          clusterPath(id, `/streams/${encodeURIComponent(name)}/messages`),
          {
            method: "POST",
            body: JSON.stringify({
              subject,
              data: encoded.data,
              headers: { "Content-Type": encoded.contentType },
            }),
          },
        )
      ).data;
      const updated = (await api<StreamInfo>(clusterPath(id, `/streams/${encodeURIComponent(name)}`))).data;
      queryClient.setQueryData(streamQueryKey, updated);
      setSeq(String(result.seq));
      await loadMessage(String(result.seq));
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : t("streams.publishFailed"));
    }
  }

  function jumpToJsonError() {
    if (!publishJsonErrorLocation) return;
    const el = document.getElementById("publish-payload-input");
    if (!(el instanceof HTMLTextAreaElement)) return;
    const pos = publishJsonErrorLocation.position;
    el.focus();
    el.setSelectionRange(pos, Math.min(pos + 1, el.value.length));
  }

  function onPublishPayloadKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (publishFormat !== "json") return;
    const edit = applyJsonTextareaKey(
      publishPayload,
      event.key,
      event.shiftKey,
      event.currentTarget.selectionStart,
      event.currentTarget.selectionEnd,
    );
    if (!edit) return;
    event.preventDefault();
    setPublishPayload(edit.value);
    setPublishPayloadTouched(true);
    const el = event.currentTarget;
    requestAnimationFrame(() => {
      el.selectionStart = edit.selectionStart;
      el.selectionEnd = edit.selectionEnd;
    });
  }

  if (!id) {
    return <p className="text-muted">{t("streams.selectCluster")}</p>;
  }

  if (!stream) {
    if (streamQuery.isError) {
      return <QueryErrorState error={streamQuery.error} onRetry={() => void streamQuery.refetch()} empty />;
    }
    if (error) return <Alert variant="error">{error}</Alert>;
    return <PageLoader />;
  }

  const firstSeq = stream.state.firstSeq;
  const lastSeq = stream.state.lastSeq;
  const hasMessages = lastSeq > 0 && stream.state.messages > 0;
  const sizeBytes = message ? payloadByteLength(message.message.data) : 0;
  const exportRow = message
    ? rowFromMessage({
        seq: message.message.seq,
        subject: message.message.subject,
        time: message.message.time,
        data: message.message.data,
        headers: message.message.headers,
      })
    : null;

  const publishMetric = streamMetric(name, StreamMetricKind.LastSeq);
  const deliverMetric = streamMetric(name, StreamMetricKind.DeliveredSeq);
  const ackMetric = streamMetric(name, StreamMetricKind.AckFloorSeq);
  const bytesMetric = streamMetric(name, StreamMetricKind.Bytes);
  const seriesFor = (metric: string) =>
    ratesHistory.data?.series.find((item) => item.metric === metric)?.points ?? [];
  const lastRate = (points: { v: number }[]) => (points.length ? points[points.length - 1].v : 0);
  const formatRate = (value: number) => {
    if (!Number.isFinite(value)) return "0";
    if (value >= 100) return Math.round(value).toLocaleString();
    if (value >= 10) return value.toFixed(1);
    return value.toFixed(2);
  };
  const formatBytesRate = (value: number) => {
    if (value < 1024) return `${formatRate(value)} B/s`;
    if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB/s`;
    return `${(value / (1024 * 1024)).toFixed(2)} MB/s`;
  };
  const publishPoints = seriesFor(publishMetric);
  const deliverPoints = seriesFor(deliverMetric);
  const ackPoints = seriesFor(ackMetric);
  const bytesPoints = seriesFor(bytesMetric);
  const rateCards = [
    {
      key: "publish",
      title: t("streams.publishPerSec"),
      value: `${formatRate(lastRate(publishPoints))}/s`,
      points: publishPoints,
    },
    {
      key: "deliver",
      title: t("streams.deliverPerSec"),
      value: `${formatRate(lastRate(deliverPoints))}/s`,
      points: deliverPoints,
    },
    {
      key: "ack",
      title: t("streams.ackPerSec"),
      value: `${formatRate(lastRate(ackPoints))}/s`,
      points: ackPoints,
    },
    {
      key: "bytes",
      title: t("streams.bytesPerSec"),
      value: formatBytesRate(lastRate(bytesPoints)),
      points: bytesPoints,
    },
  ];

  async function copyText(label: string, value: string) {
    try {
      await navigator.clipboard.writeText(value);
      setCopyHint(label);
      window.setTimeout(() => setCopyHint(""), 1500);
    } catch {
      setError(t("streams.copyFailed"));
    }
  }

  return (
    <div className="page">
      {backLabel ? (
        <p className="mb-12">
          <Link to={backHref} className="link-back" state={backState}>
            {backLabel}
          </Link>
        </p>
      ) : (
        jsBase && <JetStreamSectionTabs base={jsBase} active="streams" />
      )}

      <PageHeader
        eyebrow={t("streams.detailEyebrow")}
        title={stream.config.name}
        subtitle={stream.config.description || stream.config.subjects?.join(", ")}
        actions={
          <div className="actions">
            {id && <StreamFavoriteButton clusterId={id} streamName={stream.config.name} />}
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
                <button className="btn secondary" type="button" onClick={() => setConfirmAction("purge")}>
                  {t("streams.purgeStream")}
                </button>
                <button className="btn danger" type="button" onClick={() => setConfirmAction("delete")}>
                  {t("streams.deleteStream")}
                </button>
              </>
            )}
          </div>
        }
      />

      {(streamQuery.isError || consumersQuery.isError) && (
        <QueryErrorState
          error={streamQuery.error ?? consumersQuery.error}
          onRetry={() => {
            void streamQuery.refetch();
            void consumersQuery.refetch();
          }}
        />
      )}
      {error && <Alert variant="error">{error}</Alert>}

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

      <ConfirmDialog
        open={confirmAction === "delete"}
        title={t("streams.confirmDeleteTitle")}
        description={
          <>
            <p>{t("streams.confirmDelete", { name })}</p>
            <BlastRadiusPanel
              data={impactQuery.data}
              loading={impactQuery.isFetching}
              error={impactQuery.error instanceof Error ? impactQuery.error.message : impactQuery.isError ? "error" : null}
            />
          </>
        }
        busy={confirmBusy}
        onCancel={() => {
          if (!confirmBusy) setConfirmAction(null);
        }}
        onConfirm={() => void deleteStream()}
      />

      <ConfirmDialog
        open={confirmAction === "purge"}
        title={t("streams.confirmPurgeTitle")}
        description={t("streams.confirmPurge", { name })}
        confirmLabel={t("streams.purge")}
        busy={confirmBusy}
        onCancel={() => {
          if (!confirmBusy) setConfirmAction(null);
        }}
        onConfirm={() => void purgeStream()}
      />

      <nav className="nc-tabs stream-tabs" aria-label={t("streams.detailSectionsAria")}>
        <button
          type="button"
          className={`nc-tab${tab === "overview" ? " active" : ""}`}
          aria-current={tab === "overview" ? "page" : undefined}
          onClick={() => setTab("overview")}
        >
          {t("streams.tabDetails")}
        </button>
        <button
          type="button"
          className={`nc-tab${tab === "consumers" ? " active" : ""}`}
          aria-current={tab === "consumers" ? "page" : undefined}
          onClick={() => setTab("consumers")}
        >
          {t("streams.tabConsumers")}
        </button>
        <button
          type="button"
          className={`nc-tab${tab === "messages" ? " active" : ""}`}
          aria-current={tab === "messages" ? "page" : undefined}
          onClick={() => setTab("messages")}
        >
          {t("streams.tabMessages")}
        </button>
        {showDlqTab && (
          <button
            type="button"
            className={`nc-tab${tab === "dlq" ? " active" : ""}`}
            aria-current={tab === "dlq" ? "page" : undefined}
            onClick={() => setTab("dlq")}
          >
            {t("streams.tabDlq")}
          </button>
        )}
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
            {!showDlqTab && jsBase && (
              <div className="stream-meta-list__row">
                <dt>{t("streams.dlqLinked")}</dt>
                <dd>
                  <Link to={`${jsBase}/streams/${encodeURIComponent(`${name}_DLQ`)}?tab=dlq`}>
                    {`${name}_DLQ`}
                  </Link>
                </dd>
              </div>
            )}
          </dl>

          <div className="section-header mt-32">
            <div>
              <h2>{t("streams.ratesTitle")}</h2>
              <p className="text-muted">{t("streams.ratesSubtitle")}</p>
            </div>
            <TimeRangeSelector value={metricsRange} onChange={setMetricsRange} />
          </div>
          {ratesHistory.error instanceof Error && (
            <Alert variant="error">{ratesHistory.error.message}</Alert>
          )}
          <div className="nc-metrics-grid">
            {rateCards.map((card) => (
              <div className="nc-metric-card" key={card.key}>
                <h3>{card.title}</h3>
                <p className="nc-metric-card__value">{card.value}</p>
                <Suspense fallback={null}>
                  <MetricsTimeSeriesChart
                    title={card.title}
                    emptyMessage={t("streams.ratesCollecting")}
                    series={[
                      {
                        key: card.key,
                        label: card.title,
                        color: "var(--accent)",
                        points: card.points,
                      },
                    ]}
                  />
                </Suspense>
              </div>
            ))}
          </div>
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
                { id: "name", header: "Name", width: "minmax(120px, 1.2fr)" },
                { id: "deliver", header: t("streams.deliverPolicy"), width: "minmax(100px, 0.9fr)" },
                { id: "ack", header: t("streams.ackPolicy"), width: "minmax(90px, 0.8fr)" },
                { id: "lag", header: t("streams.lag"), width: "72px", align: "right" },
                { id: "pending", header: t("streams.pending"), width: "88px", align: "right" },
                { id: "ackPending", header: t("streams.ackPending"), width: "104px", align: "right" },
                { id: "waiting", header: t("streams.waiting"), width: "88px", align: "right" },
                { id: "redelivered", header: t("streams.redelivered"), width: "104px", align: "right" },
                { id: "delivered", header: t("streams.deliveredSeq"), width: "112px", align: "right" },
                { id: "ackFloor", header: t("streams.ackFloor"), width: "104px", align: "right" },
              ]}
              items={consumers}
              empty={t("streams.noConsumers")}
              getKey={(consumer) => consumer.name}
              renderCell={(consumer, columnId) => {
                switch (columnId) {
                  case "name":
                    return (
                      <span style={{ display: "inline-flex", gap: "0.5rem", alignItems: "center" }}>
                        <Link to={`${streamHref}/consumers/${encodeURIComponent(consumer.name)}`}>
                          {consumer.name}
                        </Link>
                        {consumer.slowConsumer ? (
                          <span className="topology-detail__chip topology-detail__chip--warn">
                            {t("consumers.slowConsumer")}
                          </span>
                        ) : null}
                      </span>
                    );
                  case "deliver":
                    return consumer.config.deliverPolicy;
                  case "ack":
                    return consumer.config.ackPolicy;
                  case "lag":
                    return consumerLag(lastSeq, consumer.delivered?.streamSeq);
                  case "pending":
                    return consumer.numPending;
                  case "ackPending":
                    return consumer.numAckPending;
                  case "waiting":
                    return consumer.numWaiting ?? 0;
                  case "redelivered":
                    return consumer.numRedelivered ?? 0;
                  case "delivered":
                    return consumer.delivered?.streamSeq ?? 0;
                  case "ackFloor":
                    return consumer.ackFloor?.streamSeq ?? 0;
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
        <div className={canManageJetStream ? "messages-layout" : "messages-layout messages-layout--browse-only"}>
          {canManageJetStream && (
            <form className="form-grid card messages-layout__publish" onSubmit={publishMessage}>
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
              <div className="form-grid__full publish-format" role="group" aria-label={t("streams.publishFormat")}>
                {(
                  [
                    ["json", t("streams.publishFormatJson")],
                    ["msgpack", t("streams.publishFormatMsgpack")],
                    ["cbor", t("streams.publishFormatCbor")],
                    ["protobuf", t("streams.publishFormatProtobuf")],
                  ] as const
                ).map(([format, label]) => (
                  <button
                    key={format}
                    type="button"
                    className={`publish-format__btn${publishFormat === format ? " publish-format__btn--active" : ""}`}
                    aria-pressed={publishFormat === format}
                    onClick={() => {
                      setPublishFormat(format);
                      setPublishPayloadTouched(false);
                    }}
                  >
                    {label}
                  </button>
                ))}
              </div>
              {publishFormat === "json" ? (
                <label className="form-grid__full messages-layout__payload">
                  {t("streams.payload")}
                  <LinedTextarea
                    id="publish-payload-input"
                    rows={10}
                    value={publishPayload}
                    onChange={(e) => {
                      setPublishPayload(e.target.value);
                      setPublishPayloadTouched(true);
                    }}
                    onKeyDown={onPublishPayloadKeyDown}
                    onBlur={() => {
                      setPublishPayloadTouched(true);
                      if (publishPayloadError) return;
                      const formatted = formatMessagePayload(publishPayload);
                      if (formatted !== publishPayload) setPublishPayload(formatted);
                    }}
                    placeholder={'{\n  "hello": "world"\n}'}
                    spellCheck={false}
                    required
                    errorLine={
                      publishPayloadTouched && publishJsonErrorLocation
                        ? publishJsonErrorLocation.line
                        : null
                    }
                    aria-invalid={publishPayloadTouched && Boolean(publishPayloadError)}
                    aria-describedby={
                      publishPayloadTouched && publishPayloadError ? "publish-payload-error" : undefined
                    }
                    className={
                      publishPayloadTouched && publishPayloadError ? "input-invalid" : undefined
                    }
                  />
                  {publishPayloadTouched && publishPayloadError && (
                    <div id="publish-payload-error" className="field-error field-error--json" role="alert">
                      <button type="button" className="field-error__jump" onClick={jumpToJsonError}>
                        {publishPayloadError}
                      </button>
                      {publishJsonErrorLocation && (
                        <pre className="field-error__snippet" aria-hidden="true">
                          {`${publishJsonErrorLocation.snippet}\n${publishJsonErrorLocation.caret}`}
                        </pre>
                      )}
                    </div>
                  )}
                </label>
              ) : (
                <label className="form-grid__full messages-layout__payload">
                  {publishFormat === "msgpack"
                    ? t("streams.payloadMsgpackBase64")
                    : publishFormat === "cbor"
                      ? t("streams.payloadCborBase64")
                      : t("streams.payloadProtobufBase64")}
                  <textarea
                    rows={10}
                    value={publishBinaryPayload}
                    onChange={(e) => {
                      setPublishBinaryPayload(e.target.value);
                      setPublishPayloadTouched(true);
                    }}
                    onBlur={() => setPublishPayloadTouched(true)}
                    placeholder={t("streams.payloadBinaryBase64Placeholder")}
                    spellCheck={false}
                    required
                    aria-invalid={publishPayloadTouched && Boolean(publishPayloadError)}
                    aria-describedby={
                      publishPayloadTouched && publishPayloadError ? "publish-payload-error" : undefined
                    }
                    className={
                      publishPayloadTouched && publishPayloadError ? "input-invalid" : undefined
                    }
                  />
                  {publishPayloadTouched && publishPayloadError && (
                    <span id="publish-payload-error" className="field-error" role="alert">
                      {publishPayloadError}
                    </span>
                  )}
                </label>
              )}
              <div className="form-grid__full messages-layout__publish-actions">
                <button className="btn" type="submit" disabled={Boolean(publishPayloadError)}>
                  {t("streams.publish")}
                </button>
                <button className="btn secondary" type="button" onClick={() => void publishQuickSample()}>
                  {t("streams.quickSample")}
                </button>
              </div>
              <div className="form-grid__full publish-templates">
                <div className="publish-templates__header">{t("streams.templatesLabel")}</div>
                <div className="publish-templates__save">
                  <input
                    value={templateName}
                    onChange={(e) => setTemplateName(e.target.value)}
                    placeholder={t("streams.templateNamePlaceholder")}
                    aria-label={t("streams.templateName")}
                  />
                  <button className="btn secondary" type="button" onClick={onSaveTemplate}>
                    {t("streams.saveTemplate")}
                  </button>
                </div>
                {streamTemplates.length === 0 ? (
                  <p className="text-muted">{t("streams.templatesEmpty")}</p>
                ) : (
                  <ul className="publish-templates__list">
                    {streamTemplates.map((template) => (
                      <li key={template.id}>
                        <button type="button" className="btn secondary btn--small" onClick={() => applyTemplate(template)}>
                          {template.name}
                        </button>
                        <button
                          type="button"
                          className="btn danger btn--small"
                          onClick={() => onDeleteTemplate(template.id)}
                        >
                          {t("streams.deleteTemplate")}
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
                {templateHint && (
                  <p className="text-muted" role="status">
                    {templateHint}
                  </p>
                )}
              </div>
            </form>
          )}

          <div className="messages-layout__browse">
            {canManageJetStream && (
              <div className="actions mb-12">
                <MessageImportButton onImport={importMessages} />
              </div>
            )}
            {!hasMessages ? (
              <EmptyState title={t("streams.noMessagesTitle")} description={t("streams.noMessagesDescription")} />
            ) : (
              <>
                {messageLoading && !message && <p className="text-muted">{t("streams.messageLoading")}</p>}

                {!message && !messageLoading && (
                  <EmptyState title={t("streams.loadMessageHint")} />
                )}

                {message && exportRow && (
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
                    <div className="message-actions">
                      <MessageDownloadMenu rows={[exportRow]} stream={name} mode="single" />
                      <button
                        type="button"
                        className="btn secondary"
                        onClick={() =>
                          void decodeMessagePayload(
                            message.message.data,
                            message.message.headers,
                          ).then((decoded) =>
                            copyText(t("streams.copyPayload"), decoded.text),
                          )
                        }
                      >
                        {t("streams.copyPayload")}
                      </button>
                      {copyHint && (
                        <span className="message-actions__hint" role="status">
                          {t("streams.copied")}: {copyHint}
                        </span>
                      )}
                    </div>
                    <MessagePayloadViewer data={message.message.data} headers={message.message.headers} />
                  </article>
                )}

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
              </>
            )}
          </div>
        </div>
      )}

      {tab === "dlq" && showDlqTab && id && (
        <DlqPanel clusterId={id} streamName={name} canManage={canManageJetStream} />
      )}
    </div>
  );
}
