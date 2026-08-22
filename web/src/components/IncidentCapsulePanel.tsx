import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import MessagePayloadViewer from "./MessagePayloadViewer";
import EmptyState from "./ui/EmptyState";
import QueryErrorState from "./ui/QueryErrorState";
import {
  api,
  clusterPath,
  type IncidentCapsuleDetail,
  type IncidentCapsuleDryRun,
  type IncidentCapsuleSummary,
} from "../lib/api";
import { formatDateTime } from "../lib/datetime";
import { clusterQueryKey } from "../lib/query";
import { useToast } from "../lib/toast";

export type IncidentCapsulePanelProps = {
  clusterId: string;
  streamName: string;
  consumer: string;
  canManage: boolean;
};

export default function IncidentCapsulePanel({
  clusterId,
  streamName,
  consumer,
  canManage,
}: IncidentCapsulePanelProps) {
  const { t } = useTranslation();
  const toast = useToast();
  const queryClient = useQueryClient();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [dryRun, setDryRun] = useState<IncidentCapsuleDryRun | null>(null);

  const listKey = clusterQueryKey(clusterId, `capsules:${streamName}:${consumer}`);

  const listQuery = useQuery({
    queryKey: listKey,
    queryFn: async () =>
      (
        await api<IncidentCapsuleSummary[]>(
          clusterPath(
            clusterId,
            `/streams/${encodeURIComponent(streamName)}/consumers/${encodeURIComponent(consumer)}/incident-capsules`,
          ),
        )
      ).data ?? [],
  });

  const detailQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `capsule:${selectedId ?? ""}`),
    enabled: Boolean(selectedId),
    queryFn: async () =>
      (
        await api<IncidentCapsuleDetail>(
          clusterPath(clusterId, `/incident-capsules/${encodeURIComponent(selectedId!)}`),
        )
      ).data,
  });

  const invalidate = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: listKey });
  }, [listKey, queryClient]);

  const dryRunMutation = useMutation({
    mutationFn: async (id: string) =>
      (
        await api<IncidentCapsuleDryRun>(
          clusterPath(clusterId, `/incident-capsules/${encodeURIComponent(id)}/replay/dry-run`),
          { method: "POST" },
        )
      ).data,
    onSuccess: (result) => {
      if (!result) {
        toast.error(t("capsules.dryRunFailed"));
        return;
      }
      setDryRun(result);
      toast.success(t("capsules.dryRunSuccess", { count: result.invoked ?? 0 }));
    },
    onError: (err: Error) => {
      toast.error(err.message || t("capsules.dryRunFailed"));
    },
  });

  const items = listQuery.data ?? [];

  return (
    <div className="nc-blast-radius nc-incident-capsules">
      <h3 className="nc-blast-radius__title">{t("capsules.title")}</h3>
      <p className="nc-blast-radius__subtitle">{t("capsules.subtitle")}</p>

      <div className="dlq-panel__toolbar">
        <button
          type="button"
          className="btn secondary btn--small"
          disabled={listQuery.isFetching}
          onClick={() => void listQuery.refetch()}
        >
          {t("common.refresh")}
        </button>
      </div>

      {listQuery.isLoading ? (
        <p className="nc-blast-radius__status">{t("capsules.loading")}</p>
      ) : null}

      {listQuery.isError ? (
        <QueryErrorState error={listQuery.error} onRetry={() => void listQuery.refetch()} />
      ) : null}

      {!listQuery.isLoading && !listQuery.isError && items.length === 0 ? (
        <EmptyState title={t("capsules.emptyTitle")} description={t("capsules.emptyDescription")} />
      ) : null}

      {items.length > 0 ? (
        <ul className="nc-incident-capsules__list">
          {items.map((c) => (
            <li key={c.id}>
              <button
                type="button"
                className={
                  selectedId === c.id
                    ? "btn secondary btn--small nc-incident-capsules__item is-active"
                    : "btn secondary btn--small nc-incident-capsules__item"
                }
                onClick={() => {
                  setSelectedId(c.id);
                  setDryRun(null);
                }}
              >
                <span className="mono">{c.id}</span>
                {c.failingSeq ? <span> · #{c.failingSeq}</span> : null}
                {c.trigger ? <span> · {c.trigger}</span> : null}
                {c.createdAt ? <span> · {formatDateTime(c.createdAt)}</span> : null}
              </button>
            </li>
          ))}
        </ul>
      ) : null}

      {selectedId && detailQuery.isLoading ? (
        <p className="nc-blast-radius__status">{t("capsules.loadingDetail")}</p>
      ) : null}

      {selectedId && detailQuery.isError ? (
        <QueryErrorState error={detailQuery.error} onRetry={() => void detailQuery.refetch()} />
      ) : null}

      {detailQuery.data ? (
        <article className="card message-viewer mt-16">
          <header className="message-meta">
            <div className="message-meta__item message-meta__item--grow">
              <span className="message-meta__label">{t("capsules.id")}</span>
              <span className="message-meta__value mono">{detailQuery.data.id}</span>
            </div>
            <div className="message-meta__item">
              <span className="message-meta__label">{t("capsules.messages")}</span>
              <span className="message-meta__value">{detailQuery.data.messageCount}</span>
            </div>
            <div className="message-meta__item">
              <span className="message-meta__label">{t("capsules.trigger")}</span>
              <span className="message-meta__value">{detailQuery.data.trigger || "—"}</span>
            </div>
            <button type="button" className="btn secondary btn--small" onClick={() => setSelectedId(null)}>
              {t("common.close")}
            </button>
          </header>
          {detailQuery.data.reason ? (
            <p className="text-muted">
              {t("capsules.reason")}: {detailQuery.data.reason}
            </p>
          ) : null}
          {canManage ? (
            <div className="message-actions">
              <button
                type="button"
                className="btn"
                disabled={dryRunMutation.isPending}
                onClick={() => dryRunMutation.mutate(detailQuery.data!.id)}
              >
                {t("capsules.dryRun")}
              </button>
            </div>
          ) : null}
          {(detailQuery.data.messages ?? []).slice(0, 5).map((m) => (
            <div key={m.sequence} className="mt-16">
              <div className="message-meta">
                <span className="mono">
                  #{m.sequence} {m.subject}
                </span>
              </div>
              <MessagePayloadViewer data={m.data} headers={m.headers} />
            </div>
          ))}
        </article>
      ) : null}

      {dryRun ? (
        <div className="nc-blast-radius__section mt-16">
          <div className="nc-blast-radius__section-label">{t("capsules.dryRunResult")}</div>
          <dl className="nc-replay-dry-run__stats">
            <div className="nc-replay-dry-run__row">
              <dt>{t("capsules.invoked")}</dt>
              <dd>{dryRun.invoked ?? 0}</dd>
            </div>
            <div className="nc-replay-dry-run__row">
              <dt>{t("capsules.subjects")}</dt>
              <dd className="mono">{(dryRun.subjects ?? []).join(", ") || "—"}</dd>
            </div>
          </dl>
          <button type="button" className="btn secondary btn--small" onClick={() => void invalidate()}>
            {t("common.refresh")}
          </button>
        </div>
      ) : null}
    </div>
  );
}
