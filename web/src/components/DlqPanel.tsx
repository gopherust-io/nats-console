import { useCallback, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import MessagePayloadViewer from "./MessagePayloadViewer";
import VirtualTable, { type VirtualTableColumn } from "./VirtualTable";
import EmptyState from "./ui/EmptyState";
import QueryErrorState from "./ui/QueryErrorState";
import { useConfirmDialog } from "../hooks/useConfirmDialog";
import {
  api,
  clusterPath,
  type DLQListResult,
  type DLQMessage,
  type DLQRetryRequest,
  type DLQRetryResult,
  type IncidentCapsuleDetail,
} from "../lib/api";
import { formatDateTime } from "../lib/datetime";
import { clusterQueryKey } from "../lib/query";
import { useToast } from "../lib/toast";

type DlqPanelProps = {
  clusterId: string;
  streamName: string;
  canManage: boolean;
};

function displayError(msg: DLQMessage): string {
  return msg.autopsyError || msg.reason || "—";
}

export default function DlqPanel({ clusterId, streamName, canManage }: DlqPanelProps) {
  const { t } = useTranslation();
  const { askConfirm, confirmDialog } = useConfirmDialog();
  const toast = useToast();
  const queryClient = useQueryClient();
  const [selected, setSelected] = useState<Set<number>>(() => new Set());
  const [detail, setDetail] = useState<DLQMessage | null>(null);
  const [startSeq, setStartSeq] = useState<number | undefined>(undefined);

  const listKey = clusterQueryKey(clusterId, `dlq:${streamName}:${startSeq ?? 0}`);

  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: async () => {
      const params = new URLSearchParams({ limit: "100" });
      if (startSeq && startSeq > 0) params.set("startSeq", String(startSeq));
      return (
        await api<DLQListResult>(
          clusterPath(clusterId, `/streams/${encodeURIComponent(streamName)}/dlq/messages?${params}`),
        )
      ).data;
    },
  });

  const invalidate = useCallback(async () => {
    await queryClient.invalidateQueries({
      predicate: (q) => {
        const key = q.queryKey;
        return key[0] === "cluster" && key[1] === clusterId && typeof key[2] === "string" && (
          (key[2] as string).startsWith(`dlq:${streamName}`) ||
          key[2] === `stream:${streamName}` ||
          key[2] === "streams"
        );
      },
    });
  }, [clusterId, queryClient, streamName]);

  const captureMutation = useMutation({
    mutationFn: async (seq: number) =>
      (
        await api<IncidentCapsuleDetail>(
          clusterPath(clusterId, `/streams/${encodeURIComponent(streamName)}/dlq/messages/${seq}/capsule`),
          { method: "POST" },
        )
      ).data,
    onSuccess: (capsule) => {
      toast.success(t("capsules.captureSuccess", { id: capsule.id }));
    },
    onError: (err: Error) => {
      toast.error(err.message || t("capsules.captureFailed"));
    },
  });

  const retryMutation = useMutation({
    mutationFn: async (body: DLQRetryRequest) =>
      (
        await api<DLQRetryResult>(clusterPath(clusterId, `/streams/${encodeURIComponent(streamName)}/dlq/retry`), {
          method: "POST",
          body: JSON.stringify(body),
        })
      ).data,
    onSuccess: async (result, vars) => {
      const retriedSeqs = new Set(vars.seqs ?? []);
      if (vars.all) {
        setDetail(null);
      } else if (detail && retriedSeqs.has(detail.seq) && !(result.failed ?? []).some((f) => f.seq === detail.seq)) {
        setDetail(null);
      }
      setSelected(new Set());
      await invalidate();
      const failedCount = result.failed?.length ?? 0;
      if (result.truncated) {
        toast.error(
          t("streams.dlqRetryTruncated", {
            retried: result.retried,
            remaining: result.remaining ?? "?",
          }),
        );
      } else if (failedCount > 0) {
        toast.error(t("streams.dlqRetryPartial", { retried: result.retried, failed: failedCount }));
      } else {
        toast.success(t("streams.dlqRetrySuccess", { count: result.retried }));
      }
    },
    onError: (err: Error) => {
      toast.error(err.message || t("streams.dlqRetryFailed"));
    },
  });

  const messages = useMemo(() => listQuery.data?.messages ?? [], [listQuery.data?.messages]);

  const toggleSeq = useCallback((seq: number) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(seq)) next.delete(seq);
      else next.add(seq);
      return next;
    });
  }, []);

  const allSelected = messages.length > 0 && messages.every((m) => selected.has(m.seq));

  const toggleAll = useCallback(() => {
    setSelected((prev) => {
      if (messages.length === 0) return prev;
      if (messages.every((m) => prev.has(m.seq))) return new Set();
      return new Set(messages.map((m) => m.seq));
    });
  }, [messages]);

  const columns = useMemo<VirtualTableColumn[]>(() => {
    const cols: VirtualTableColumn[] = [
      {
        id: "select",
        header: canManage ? (
          <input
            type="checkbox"
            checked={allSelected}
            onChange={toggleAll}
            aria-label={t("streams.dlqSelectAll")}
          />
        ) : (
          ""
        ),
        width: "44px",
        align: "center",
      },
      { id: "seq", header: t("streams.seq"), width: "88px" },
      { id: "time", header: t("streams.time"), width: "minmax(140px, 1fr)" },
      { id: "original", header: t("streams.dlqOriginalSubject"), width: "minmax(160px, 1.4fr)" },
      { id: "error", header: t("streams.dlqErrorReason"), width: "minmax(180px, 1.6fr)" },
      { id: "consumer", header: t("streams.dlqConsumer"), width: "minmax(100px, 1fr)" },
    ];
    if (canManage) {
      cols.push({ id: "actions", header: "", width: "96px", align: "right" });
    }
    return cols;
  }, [allSelected, canManage, t, toggleAll]);

  const renderCell = useCallback(
    (msg: DLQMessage, columnId: string) => {
      switch (columnId) {
        case "select":
          return canManage ? (
            <input
              type="checkbox"
              checked={selected.has(msg.seq)}
              onChange={() => toggleSeq(msg.seq)}
              aria-label={t("streams.dlqSelectRow", { seq: msg.seq })}
            />
          ) : null;
        case "seq":
          return (
            <button type="button" className="link-btn mono" onClick={() => setDetail(msg)}>
              #{msg.seq}
            </button>
          );
        case "time":
          return (
            <time dateTime={msg.time} title={msg.time}>
              {formatDateTime(msg.time)}
            </time>
          );
        case "original":
          return (
            <button
              type="button"
              className="link-btn mono virtual-table__truncate"
              title={msg.originalSubject || msg.subject}
              onClick={() => setDetail(msg)}
            >
              {msg.originalSubject || msg.subject || "—"}
            </button>
          );
        case "error":
          return (
            <span className="virtual-table__truncate" title={displayError(msg)}>
              {displayError(msg)}
            </span>
          );
        case "consumer":
          return <span className="mono">{msg.consumer || "—"}</span>;
        case "actions":
          return canManage ? (
            <button
              type="button"
              className="btn secondary btn--small"
              disabled={retryMutation.isPending}
              onClick={() => retryMutation.mutate({ seqs: [msg.seq] })}
            >
              {t("streams.dlqRetry")}
            </button>
          ) : null;
        default:
          return null;
      }
    },
    [canManage, retryMutation, selected, t, toggleSeq],
  );

  function retrySelected() {
    const seqs = Array.from(selected).sort((a, b) => a - b);
    if (seqs.length === 0) return;
    retryMutation.mutate({ seqs });
  }

  function retryAll() {
    askConfirm({
      title: t("streams.dlqConfirmRetryAllTitle"),
      description: t("streams.dlqConfirmRetryAll"),
      confirmLabel: t("streams.dlqRetryAll"),
      action: () => retryMutation.mutate({ all: true }),
    });
  }

  if (listQuery.isLoading) {
    return <p className="text-muted">{t("streams.loading")}</p>;
  }

  if (listQuery.isError) {
    return <QueryErrorState error={listQuery.error} onRetry={() => void listQuery.refetch()} />;
  }

  return (
    <div className="dlq-panel">
      {confirmDialog}
      <div className="dlq-panel__toolbar">
        <div className="dlq-panel__actions">
          {canManage && (
            <>
              <button
                type="button"
                className="btn secondary"
                disabled={selected.size === 0 || retryMutation.isPending}
                onClick={retrySelected}
              >
                {t("streams.dlqRetrySelected", { count: selected.size })}
              </button>
              <button
                type="button"
                className="btn"
                disabled={messages.length === 0 || retryMutation.isPending}
                onClick={retryAll}
              >
                {t("streams.dlqRetryAll")}
              </button>
            </>
          )}
          <button
            type="button"
            className="btn secondary"
            disabled={listQuery.isFetching}
            onClick={() => void listQuery.refetch()}
          >
            {t("common.refresh")}
          </button>
        </div>
        {listQuery.data?.nextSeq != null && (
          <button
            type="button"
            className="btn secondary btn--small"
            onClick={() => setStartSeq(listQuery.data!.nextSeq)}
          >
            {t("streams.dlqLoadMore")}
          </button>
        )}
        {startSeq != null && startSeq > 0 && (
          <button type="button" className="btn secondary btn--small" onClick={() => setStartSeq(undefined)}>
            {t("streams.dlqFromStart")}
          </button>
        )}
      </div>

      {messages.length === 0 ? (
        <EmptyState title={t("streams.dlqEmptyTitle")} description={t("streams.dlqEmptyDescription")} />
      ) : (
        <VirtualTable
          columns={columns}
          items={messages}
          getKey={(m) => String(m.seq)}
          renderCell={renderCell}
        />
      )}

      {detail && (
        <article className="card message-viewer dlq-panel__detail mt-16">
          <header className="message-meta">
            <div className="message-meta__item">
              <span className="message-meta__label">{t("streams.seq")}</span>
              <span className="message-meta__value mono">#{detail.seq}</span>
            </div>
            <div className="message-meta__item message-meta__item--grow">
              <span className="message-meta__label">{t("streams.dlqOriginalSubject")}</span>
              <span className="message-meta__value mono">{detail.originalSubject || "—"}</span>
            </div>
            <div className="message-meta__item">
              <span className="message-meta__label">{t("streams.dlqErrorReason")}</span>
              <span className="message-meta__value">{displayError(detail)}</span>
            </div>
            <button type="button" className="btn secondary btn--small" onClick={() => setDetail(null)}>
              {t("common.close")}
            </button>
          </header>
          {(detail.autopsyStack || detail.sourceStream || detail.numDelivered) && (
            <dl className="dlq-panel__meta">
              {detail.sourceStream && (
                <>
                  <dt>{t("streams.dlqSourceStream")}</dt>
                  <dd className="mono">{detail.sourceStream}</dd>
                </>
              )}
              {detail.sourceSeq != null && detail.sourceSeq > 0 && (
                <>
                  <dt>{t("streams.dlqSourceSeq")}</dt>
                  <dd className="mono">#{detail.sourceSeq}</dd>
                </>
              )}
              {detail.numDelivered != null && detail.numDelivered > 0 && (
                <>
                  <dt>{t("streams.dlqNumDelivered")}</dt>
                  <dd>{detail.numDelivered}</dd>
                </>
              )}
              {detail.autopsyHash && (
                <>
                  <dt>{t("streams.dlqAutopsyHash")}</dt>
                  <dd className="mono">{detail.autopsyHash}</dd>
                </>
              )}
            </dl>
          )}
          {detail.autopsyStack && <pre className="dlq-panel__stack mono">{detail.autopsyStack}</pre>}
          <MessagePayloadViewer data={detail.data} headers={detail.headers} />
          {canManage && (
            <div className="message-actions">
              <button
                type="button"
                className="btn"
                disabled={retryMutation.isPending}
                onClick={() => retryMutation.mutate({ seqs: [detail.seq] })}
              >
                {t("streams.dlqRetry")}
              </button>
              <button
                type="button"
                className="btn secondary"
                disabled={
                  captureMutation.isPending ||
                  !detail.sourceStream ||
                  !detail.sourceSeq ||
                  !detail.consumer
                }
                title={
                  !detail.sourceStream || !detail.sourceSeq || !detail.consumer
                    ? t("capsules.captureNeedsHeaders")
                    : undefined
                }
                onClick={() => captureMutation.mutate(detail.seq)}
              >
                {t("capsules.captureFromDlq")}
              </button>
            </div>
          )}
        </article>
      )}
    </div>
  );
}
