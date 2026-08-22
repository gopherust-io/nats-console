import { FormEvent, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router";
import IncidentReconstructionPanel from "../components/IncidentReconstructionPanel";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import QueryErrorState from "../components/ui/QueryErrorState";
import VirtualTable from "../components/VirtualTable";
import {
  api,
  AuditEntry,
  clusterPath,
  ConsumerInfo,
  IncidentReconstruction,
  StreamInfo,
} from "../lib/api";
import { useCluster } from "../lib/cluster";
import { AUDIT_PAGE_LIMIT } from "../lib/constants";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";


function formatClusterId(clusterId: string, emDash: string) {
  if (!clusterId) return emDash;
  if (clusterId.length <= 13) return clusterId;
  return `${clusterId.slice(0, 8)}…${clusterId.slice(-4)}`;
}

function formatResource(entry: AuditEntry, emDash: string) {
  if (!entry.resourceType) return emDash;
  if (!entry.resourceName) return entry.resourceType;
  return `${entry.resourceType} / ${entry.resourceName}`;
}

export default function AuditPage() {
  const { t } = useTranslation();
  const { clusterId, clusters } = useCluster();
  const [searchParams] = useSearchParams();
  const [filterInput, setFilterInput] = useState("");
  const [appliedClusterFilter, setAppliedClusterFilter] = useState("");
  const [expandedEntryId, setExpandedEntryId] = useState<string | null>(null);
  const [incidentClusterId, setIncidentClusterId] = useState(
    () => searchParams.get("cluster") || clusterId || "",
  );
  const [incidentStream, setIncidentStream] = useState(() => searchParams.get("stream") || "");
  const [incidentConsumer, setIncidentConsumer] = useState(
    () => searchParams.get("consumer") || "",
  );
  const emDash = t("common.emDash");

  useEffect(() => {
    const fromQuery = searchParams.get("cluster");
    if (fromQuery) {
      setIncidentClusterId(fromQuery);
      return;
    }
    const initial = clusterId ?? "";
    setFilterInput(initial);
    setAppliedClusterFilter(initial);
  }, [clusterId, searchParams]);

  useEffect(() => {
    const stream = searchParams.get("stream");
    const consumer = searchParams.get("consumer");
    if (stream) setIncidentStream(stream);
    if (consumer) setIncidentConsumer(consumer);
  }, [searchParams]);

  useEffect(() => {
    if (searchParams.get("cluster") || searchParams.get("stream") || searchParams.get("consumer")) {
      return;
    }
    const initial = clusterId ?? clusters[0]?.id ?? "";
    setIncidentClusterId(initial);
    setIncidentStream("");
    setIncidentConsumer("");
  }, [clusterId, clusters, searchParams]);

  const auditQuery = useQuery({
    queryKey: ["audit", appliedClusterFilter, AUDIT_PAGE_LIMIT],
    queryFn: async () => {
      const params = new URLSearchParams({ limit: String(AUDIT_PAGE_LIMIT) });
      if (appliedClusterFilter) params.set("clusterId", appliedClusterFilter);
      const r = await api<AuditEntry[]>(`/api/v1/audit?${params}`);
      return { entries: r.data ?? [], total: r.meta?.total ?? 0 };
    },
  });

  const streamsQuery = useQuery({
    queryKey: clusterQueryKey(incidentClusterId, "audit-incident-streams"),
    queryFn: async () =>
      (await api<StreamInfo[]>(clusterPath(incidentClusterId, "/streams?offset=0&limit=500"))).data ?? [],
    enabled: Boolean(incidentClusterId),
    staleTime: 30_000,
  });

  useEffect(() => {
    const streams = streamsQuery.data ?? [];
    if (streams.some((stream) => stream.config.name === incidentStream)) return;
    setIncidentStream(streams[0]?.config.name ?? "");
    setIncidentConsumer("");
  }, [streamsQuery.data, incidentStream]);

  const consumersQuery = useQuery({
    queryKey: clusterQueryKey(incidentClusterId, `audit-incident-consumers:${incidentStream}`),
    queryFn: async () =>
      (
        await api<ConsumerInfo[]>(
          clusterPath(
            incidentClusterId,
            `/streams/${encodeURIComponent(incidentStream)}/consumers?offset=0&limit=500`,
          ),
        )
      ).data ?? [],
    enabled: Boolean(incidentClusterId && incidentStream),
    staleTime: 30_000,
  });

  useEffect(() => {
    const consumers = consumersQuery.data ?? [];
    if (consumers.some((consumer) => consumer.name === incidentConsumer)) return;
    setIncidentConsumer(consumers[0]?.name ?? "");
  }, [consumersQuery.data, incidentConsumer]);

  const incidentQuery = useQuery({
    queryKey: clusterQueryKey(
      incidentClusterId,
      `audit-incident-reconstruction:${incidentStream}:${incidentConsumer}`,
    ),
    queryFn: async () =>
      (
        await api<IncidentReconstruction>(
          clusterPath(
            incidentClusterId,
            `/streams/${encodeURIComponent(incidentStream)}/consumers/${encodeURIComponent(incidentConsumer)}/incident-reconstruction`,
          ),
        )
      ).data,
    enabled: Boolean(incidentClusterId && incidentStream && incidentConsumer),
    refetchInterval: visibilityAwareInterval(30_000),
  });

  const entries = auditQuery.data?.entries ?? [];
  const total = auditQuery.data?.total ?? 0;
  const error = auditQuery.isError;

  function onFilter(event: FormEvent) {
    event.preventDefault();
    setAppliedClusterFilter(filterInput.trim());
    setExpandedEntryId(null);
  }

  function toggleDetails(entryId: string) {
    setExpandedEntryId((current) => (current === entryId ? null : entryId));
  }

  return (
    <div className="page">
      <PageHeader
        eyebrow={t("audit.eyebrow")}
        title={t("audit.title")}
        subtitle={t("audit.subtitle")}
        badge={<span className="badge">{t("audit.entriesCount", { count: total })}</span>}
        actions={
          <button
            className="btn btn--secondary"
            type="button"
            onClick={() => auditQuery.refetch()}
            disabled={auditQuery.isFetching}
          >
            {t("common.refresh")}
          </button>
        }
      />

      {error && <QueryErrorState error={auditQuery.error} onRetry={() => void auditQuery.refetch()} />}

      <section className="panel audit-incident">
        <div className="audit-incident__selectors">
          <label className="audit-toolbar__field">
            <span className="audit-toolbar__label">{t("audit.incidentCluster")}</span>
            <select
              value={incidentClusterId}
              onChange={(event) => {
                setIncidentClusterId(event.target.value);
                setIncidentStream("");
                setIncidentConsumer("");
              }}
              aria-label={t("audit.incidentCluster")}
            >
              <option value="">{t("audit.selectCluster")}</option>
              {clusters.map((cluster) => (
                <option key={cluster.id} value={cluster.id}>
                  {cluster.name}
                </option>
              ))}
            </select>
          </label>

          <label className="audit-toolbar__field">
            <span className="audit-toolbar__label">{t("audit.incidentStream")}</span>
            <select
              value={incidentStream}
              onChange={(event) => {
                setIncidentStream(event.target.value);
                setIncidentConsumer("");
              }}
              disabled={!incidentClusterId || streamsQuery.isLoading}
              aria-label={t("audit.incidentStream")}
            >
              <option value="">{t("audit.selectStream")}</option>
              {(streamsQuery.data ?? []).map((stream) => (
                <option key={stream.config.name} value={stream.config.name}>
                  {stream.config.name}
                </option>
              ))}
            </select>
          </label>

          <label className="audit-toolbar__field">
            <span className="audit-toolbar__label">{t("audit.incidentConsumer")}</span>
            <select
              value={incidentConsumer}
              onChange={(event) => setIncidentConsumer(event.target.value)}
              disabled={!incidentStream || consumersQuery.isLoading}
              aria-label={t("audit.incidentConsumer")}
            >
              <option value="">{t("audit.selectConsumer")}</option>
              {(consumersQuery.data ?? []).map((consumer) => (
                <option key={consumer.name} value={consumer.name}>
                  {consumer.name}
                </option>
              ))}
            </select>
          </label>
        </div>

        {!incidentClusterId || !incidentStream || !incidentConsumer ? (
          <p className="audit-incident__hint">{t("audit.incidentSelectionHint")}</p>
        ) : (
          <IncidentReconstructionPanel
            data={incidentQuery.data ?? null}
            loading={incidentQuery.isLoading}
            error={
              incidentQuery.isError
                ? incidentQuery.error instanceof Error
                  ? incidentQuery.error.message
                  : "error"
                : null
            }
          />
        )}
      </section>

      <form className="audit-toolbar panel" onSubmit={onFilter}>
        <label className="audit-toolbar__field">
          <span className="audit-toolbar__label">{t("audit.clusterFilter")}</span>
          <input
            value={filterInput}
            onChange={(event) => setFilterInput(event.target.value)}
            placeholder={t("audit.clusterPlaceholder")}
            aria-label={t("audit.clusterFilter")}
          />
        </label>
        <button className="btn" type="submit">
          {t("audit.applyFilter")}
        </button>
      </form>

      {auditQuery.isLoading && <div className="skeleton skeleton--table" />}

      {!auditQuery.isLoading && !auditQuery.isError && entries.length === 0 && (
        <EmptyState title={t("audit.emptyTitle")} description={t("audit.emptyDescription")} />
      )}

      {!auditQuery.isLoading && entries.length > 0 && (
        <div className="table-wrap audit-table">
          <VirtualTable
            columns={[
              { id: "time", header: t("audit.time"), width: "minmax(140px, 1.1fr)" },
              { id: "actor", header: t("audit.actor"), width: "minmax(100px, 1fr)" },
              { id: "action", header: t("audit.action"), width: "minmax(120px, 1fr)" },
              { id: "cluster", header: t("audit.cluster"), width: "minmax(100px, 0.9fr)" },
              { id: "resource", header: t("audit.resource"), width: "minmax(140px, 1.2fr)" },
              { id: "ip", header: t("audit.ip"), width: "minmax(90px, 0.8fr)" },
              { id: "details", header: t("audit.details"), width: "100px" },
            ]}
            items={entries}
            rowHeight={52}
            detailHeight={220}
            maxHeight={640}
            empty={t("audit.emptyTitle")}
            getKey={(entry) => entry.id}
            isDetailOpen={(entry) => expandedEntryId === entry.id}
            getRowClassName={(entry) =>
              expandedEntryId === entry.id ? "audit-entry audit-entry--expanded" : "audit-entry"
            }
            renderCell={(entry, columnId) => {
              const isExpanded = expandedEntryId === entry.id;
              switch (columnId) {
                case "time":
                  return <time dateTime={entry.timestamp}>{new Date(entry.timestamp).toLocaleString()}</time>;
                case "actor":
                  return entry.actor || emDash;
                case "action":
                  return <span className="audit-action">{entry.action}</span>;
                case "cluster":
                  return entry.clusterId ? (
                    <span className="mono virtual-table__truncate" title={entry.clusterId}>
                      {formatClusterId(entry.clusterId, emDash)}
                    </span>
                  ) : (
                    emDash
                  );
                case "resource":
                  return (
                    <span className="virtual-table__truncate" title={formatResource(entry, emDash)}>
                      {formatResource(entry, emDash)}
                    </span>
                  );
                case "ip":
                  return <span className="mono">{entry.ip || emDash}</span>;
                case "details":
                  return (
                    <button
                      className="btn btn--ghost btn--small"
                      type="button"
                      aria-expanded={isExpanded}
                      onClick={() => toggleDetails(entry.id)}
                    >
                      {isExpanded ? t("audit.hide") : t("audit.show")}
                    </button>
                  );
                default:
                  return null;
              }
            }}
            renderDetail={(entry) =>
              expandedEntryId === entry.id ? (
                <div className="audit-entry__details">
                  <div className="audit-entry__details-head">
                    <span className="audit-entry__details-label">{t("audit.requestDetails")}</span>
                    {entry.requestId && (
                      <span className="audit-entry__request-id mono">req {entry.requestId}</span>
                    )}
                  </div>
                  <pre className="audit-entry__json mono">{JSON.stringify(entry.details, null, 2)}</pre>
                </div>
              ) : null
            }
          />
        </div>
      )}
    </div>
  );
}
