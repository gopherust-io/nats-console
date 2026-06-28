import { FormEvent, useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Alert from "../components/ui/Alert";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import { api, AuditEntry } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { AUDIT_PAGE_LIMIT } from "../lib/constants";

type AuditListResponse = {
  entries: AuditEntry[];
  total: number;
};

function formatClusterId(clusterId: string) {
  if (!clusterId) return "—";
  if (clusterId.length <= 13) return clusterId;
  return `${clusterId.slice(0, 8)}…${clusterId.slice(-4)}`;
}

function formatResource(entry: AuditEntry) {
  if (!entry.resourceType) return "—";
  if (!entry.resourceName) return entry.resourceType;
  return `${entry.resourceType} / ${entry.resourceName}`;
}

export default function AuditPage() {
  const { clusterId } = useCluster();
  const [filterInput, setFilterInput] = useState("");
  const [appliedClusterFilter, setAppliedClusterFilter] = useState("");
  const [expandedEntryId, setExpandedEntryId] = useState<string | null>(null);

  useEffect(() => {
    const initial = clusterId ?? "";
    setFilterInput(initial);
    setAppliedClusterFilter(initial);
  }, [clusterId]);

  const auditQuery = useQuery({
    queryKey: ["audit", appliedClusterFilter, AUDIT_PAGE_LIMIT],
    queryFn: async () => {
      const params = new URLSearchParams({ limit: String(AUDIT_PAGE_LIMIT) });
      if (appliedClusterFilter) params.set("clusterId", appliedClusterFilter);
      return api<AuditListResponse>(`/api/v1/audit?${params}`);
    },
  });

  const entries = auditQuery.data?.entries ?? [];
  const total = auditQuery.data?.total ?? 0;
  const error = auditQuery.error instanceof Error ? auditQuery.error.message : "";

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
        eyebrow="Administration"
        title="Audit Log"
        subtitle="Review operator actions across clusters — who did what, when, and from where."
        badge={<span className="badge">{total} entries</span>}
        actions={
          <button
            className="btn btn--secondary"
            type="button"
            onClick={() => auditQuery.refetch()}
            disabled={auditQuery.isFetching}
          >
            Refresh
          </button>
        }
      />

      <Alert variant="error">{error}</Alert>

      <form className="audit-toolbar panel" onSubmit={onFilter}>
        <label className="audit-toolbar__field">
          <span className="audit-toolbar__label">Cluster ID filter</span>
          <input
            value={filterInput}
            onChange={(event) => setFilterInput(event.target.value)}
            placeholder="Optional cluster UUID"
            aria-label="Cluster ID filter"
          />
        </label>
        <button className="btn" type="submit">
          Apply filter
        </button>
      </form>

      {auditQuery.isLoading && <div className="skeleton skeleton--table" />}

      {!auditQuery.isLoading && !auditQuery.isError && entries.length === 0 && (
        <EmptyState
          title="No audit entries yet"
          description="Actions such as stream creation, login events, and assistant requests will appear here."
        />
      )}

      {!auditQuery.isLoading && entries.length > 0 && (
        <div className="table-wrap audit-table">
          <div className="audit-table__header" role="row">
            <div className="audit-table__cell audit-table__cell--time" role="columnheader">
              Time
            </div>
            <div className="audit-table__cell" role="columnheader">
              Actor
            </div>
            <div className="audit-table__cell audit-table__cell--action" role="columnheader">
              Action
            </div>
            <div className="audit-table__cell audit-table__cell--cluster" role="columnheader">
              Cluster
            </div>
            <div className="audit-table__cell audit-table__cell--resource" role="columnheader">
              Resource
            </div>
            <div className="audit-table__cell audit-table__cell--ip" role="columnheader">
              IP
            </div>
            <div className="audit-table__cell audit-table__cell--details" role="columnheader">
              Details
            </div>
          </div>

          <div className="audit-table__body">
            {entries.map((entry) => {
              const isExpanded = expandedEntryId === entry.id;
              return (
                <article key={entry.id} className={`audit-entry${isExpanded ? " audit-entry--expanded" : ""}`}>
                  <div className="audit-entry__row" role="row">
                    <div className="audit-table__cell audit-table__cell--time" role="cell">
                      <time dateTime={entry.timestamp}>{new Date(entry.timestamp).toLocaleString()}</time>
                    </div>
                    <div className="audit-table__cell" role="cell">
                      {entry.actor || "—"}
                    </div>
                    <div className="audit-table__cell audit-table__cell--action" role="cell">
                      <span className="audit-action">{entry.action}</span>
                    </div>
                    <div className="audit-table__cell audit-table__cell--cluster" role="cell">
                      {entry.clusterId ? (
                        <span className="mono virtual-table__truncate" title={entry.clusterId}>
                          {formatClusterId(entry.clusterId)}
                        </span>
                      ) : (
                        "—"
                      )}
                    </div>
                    <div className="audit-table__cell audit-table__cell--resource" role="cell">
                      <span className="virtual-table__truncate" title={formatResource(entry)}>
                        {formatResource(entry)}
                      </span>
                    </div>
                    <div className="audit-table__cell audit-table__cell--ip" role="cell">
                      <span className="mono">{entry.ip || "—"}</span>
                    </div>
                    <div className="audit-table__cell audit-table__cell--details" role="cell">
                      <button
                        className="btn btn--ghost btn--small"
                        type="button"
                        aria-expanded={isExpanded}
                        onClick={() => toggleDetails(entry.id)}
                      >
                        {isExpanded ? "Hide" : "Show"}
                      </button>
                    </div>
                  </div>

                  {isExpanded && (
                    <div className="audit-entry__details">
                      <div className="audit-entry__details-head">
                        <span className="audit-entry__details-label">Request details</span>
                        {entry.requestId && (
                          <span className="audit-entry__request-id mono">req {entry.requestId}</span>
                        )}
                      </div>
                      <pre className="audit-entry__json mono">{JSON.stringify(entry.details, null, 2)}</pre>
                    </div>
                  )}
                </article>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
