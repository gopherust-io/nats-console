import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useSearchParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "../components/ui/Alert";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import QueryErrorState from "../components/ui/QueryErrorState";
import { useAccount } from "../lib/account";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { MONITORING_POLL_MS } from "../lib/constants";
import {
  deleteEventCatalogEntry,
  eventCatalogConsumerHref,
  eventCatalogStreamHref,
  eventCatalogWikipediaHref,
  fetchEventCatalog,
  filterEventCatalogEntries,
  formatEventCatalogSchema,
  parseEventCatalogSchema,
  sortEventCatalogEntries,
  upsertEventCatalogEntry,
  type EventCatalogEntry,
} from "../lib/eventCatalog";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";

export default function EventCatalogPage() {
  const { t } = useTranslation();
  const { clusterId, cluster } = useCluster();
  const { accountName } = useAccount();
  const { canManageJetStream } = useAuth();
  const canManageJS = canManageJetStream(clusterId);
  const qc = useQueryClient();

  const [searchParams] = useSearchParams();
  const [search, setSearch] = useState(() => searchParams.get("q")?.trim() ?? "");
  const [selectedSubject, setSelectedSubject] = useState<string | null>(() => {
    const q = searchParams.get("q")?.trim();
    return q || null;
  });
  const [owner, setOwner] = useState("");
  const [description, setDescription] = useState("");
  const [schemaText, setSchemaText] = useState("");
  const [exampleText, setExampleText] = useState("");
  const [deprecated, setDeprecated] = useState(false);
  const [successorSubject, setSuccessorSubject] = useState("");
  const [deprecationNote, setDeprecationNote] = useState("");
  const [formError, setFormError] = useState("");

  const catalogQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "event-catalog"),
    queryFn: () => fetchEventCatalog(clusterId!, { fresh: true }),
    enabled: Boolean(clusterId),
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS),
  });

  const entries = useMemo(
    () => sortEventCatalogEntries(catalogQuery.data?.entries ?? []),
    [catalogQuery.data?.entries],
  );
  const filtered = useMemo(() => filterEventCatalogEntries(entries, search), [entries, search]);
  const totals = catalogQuery.data?.totals;

  const selected: EventCatalogEntry | null = useMemo(() => {
    if (!selectedSubject) return filtered[0] ?? entries[0] ?? null;
    return entries.find((e) => e.subject === selectedSubject) ?? filtered[0] ?? null;
  }, [selectedSubject, entries, filtered]);

  useEffect(() => {
    setSelectedSubject(null);
    setFormError("");
  }, [clusterId]);

  useEffect(() => {
    if (!selected) {
      setOwner("");
      setDescription("");
      setSchemaText("");
      setExampleText("");
      setDeprecated(false);
      setSuccessorSubject("");
      setDeprecationNote("");
      setFormError("");
      return;
    }
    setOwner(selected.owner ?? "");
    setDescription(selected.description ?? "");
    setSchemaText(formatEventCatalogSchema(selected.schema));
    setExampleText(formatEventCatalogSchema(selected.example));
    setDeprecated(Boolean(selected.deprecated));
    setSuccessorSubject(selected.successorSubject ?? "");
    setDeprecationNote(selected.deprecationNote ?? "");
    setFormError("");
    // Reseed when subject or cluster changes; skip poll-only refreshes of the same entry.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentional: identity only
  }, [selected?.subject, clusterId]);

  useEffect(() => {
    if (selected && !filtered.some((e) => e.subject === selected.subject) && filtered[0]) {
      setSelectedSubject(filtered[0].subject);
    }
  }, [filtered, selected]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      if (!clusterId || !selected) throw new Error("missing selection");
      const parsed = parseEventCatalogSchema(schemaText);
      if (parsed.error) throw new Error(parsed.error);
      const exampleParsed = parseEventCatalogSchema(exampleText);
      if (exampleParsed.error) throw new Error(exampleParsed.error.replace("Schema", "Example"));
      return upsertEventCatalogEntry(clusterId, selected.subject, {
        owner,
        description,
        schema: parsed.schema,
        example: exampleParsed.schema,
        deprecated,
        successorSubject,
        deprecationNote,
      });
    },
    onSuccess: async () => {
      setFormError("");
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "event-catalog") });
    },
    onError: (e: Error) => setFormError(e.message),
  });

  const clearMutation = useMutation({
    mutationFn: async () => {
      if (!clusterId || !selected) throw new Error("missing selection");
      await deleteEventCatalogEntry(clusterId, selected.subject);
    },
    onSuccess: async () => {
      setFormError("");
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "event-catalog") });
    },
    onError: (e: Error) => setFormError(e.message),
  });

  if (!clusterId) {
    return (
      <div className="nc-page">
        <PageHeader title={t("catalog.title")} subtitle={t("catalog.needSystem")} />
        <EmptyState title={t("catalog.needSystem")} />
      </div>
    );
  }

  return (
    <div className="nc-page">
      <PageHeader
        title={t("catalog.title")}
        subtitle={t("catalog.subtitle", { system: cluster?.name ?? clusterId })}
        actions={
          <button
            type="button"
            className="btn btn--ghost btn--small"
            onClick={() => void catalogQuery.refetch()}
            disabled={catalogQuery.isFetching}
          >
            {t("common.refresh")}
          </button>
        }
      />

      {catalogQuery.isError && (
        <QueryErrorState error={catalogQuery.error} onRetry={() => void catalogQuery.refetch()} />
      )}

      {totals && (
        <div className="nc-catalog-stats" aria-label={t("catalog.statsAria")}>
          <span>
            {t("catalog.statTotal")}: <strong>{totals.total}</strong>
          </span>
          <span>
            {t("catalog.statDocumented")}: <strong>{totals.documented}</strong>
          </span>
          <span>
            {t("catalog.statUndocumented")}: <strong>{totals.undocumented}</strong>
          </span>
          <span>
            {t("catalog.statOrphan")}: <strong>{totals.orphan}</strong>
          </span>
        </div>
      )}

      <div className="nc-catalog-layout">
        <aside className="nc-catalog-list" aria-label={t("catalog.listAria")}>
          <input
            className="nc-catalog-search"
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("catalog.searchPlaceholder")}
            aria-label={t("catalog.searchPlaceholder")}
          />
          {filtered.length === 0 ? (
            <EmptyState title={t("catalog.empty")} />
          ) : (
            <ul className="nc-catalog-entries">
              {filtered.map((entry) => {
                const active = selected?.subject === entry.subject;
                return (
                  <li key={entry.subject}>
                    <button
                      type="button"
                      className={`nc-catalog-entry${active ? " is-active" : ""}`}
                      onClick={() => setSelectedSubject(entry.subject)}
                    >
                      <span className="nc-catalog-entry__subject">{entry.subject}</span>
                      <span className="nc-catalog-entry__meta">
                        {entry.owner || t("catalog.noOwner")}
                      </span>
                      <span className="nc-catalog-entry__badges">
                        {entry.deprecated && (
                          <span className="badge">{t("catalog.badgeDeprecated")}</span>
                        )}
                        {!entry.documented && (
                          <span className="badge">{t("catalog.badgeUndocumented")}</span>
                        )}
                        {entry.orphan && (
                          <span className="badge badge--muted">{t("catalog.badgeOrphan")}</span>
                        )}
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </aside>

        <section className="nc-catalog-detail" aria-label={t("catalog.detailAria")}>
          {!selected ? (
            <EmptyState title={t("catalog.selectEvent")} />
          ) : (
            <>
              <header className="nc-catalog-detail__header">
                <h2 className="nc-catalog-detail__subject">{selected.subject}</h2>
                {selected.orphan && (
                  <span className="badge badge--muted">{t("catalog.badgeOrphan")}</span>
                )}
                {selected.deprecated && (
                  <span className="badge">{t("catalog.badgeDeprecated")}</span>
                )}
                <Link
                  className="btn btn--ghost btn--small"
                  to={eventCatalogWikipediaHref(selected.subject)}
                >
                  {t("catalog.viewWikipedia")}
                </Link>
              </header>

              {formError && <Alert variant="error">{formError}</Alert>}

              <div className="nc-form-row">
                <label htmlFor="catalog-owner">{t("catalog.owner")}</label>
                <input
                  id="catalog-owner"
                  value={owner}
                  onChange={(e) => setOwner(e.target.value)}
                  disabled={!canManageJS}
                  placeholder={t("catalog.ownerPlaceholder")}
                />
              </div>
              <div className="nc-form-row">
                <label htmlFor="catalog-description">{t("catalog.description")}</label>
                <textarea
                  id="catalog-description"
                  rows={3}
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  disabled={!canManageJS}
                  placeholder={t("catalog.descriptionPlaceholder")}
                />
              </div>
              <div className="nc-form-row">
                <label htmlFor="catalog-schema">{t("catalog.schema")}</label>
                <textarea
                  id="catalog-schema"
                  className="nc-catalog-schema"
                  rows={12}
                  value={schemaText}
                  onChange={(e) => setSchemaText(e.target.value)}
                  disabled={!canManageJS}
                  placeholder='{"type":"object","properties":{...}}'
                  spellCheck={false}
                />
              </div>
              <div className="nc-form-row">
                <label htmlFor="catalog-example">{t("catalog.example")}</label>
                <textarea
                  id="catalog-example"
                  className="nc-catalog-schema"
                  rows={8}
                  value={exampleText}
                  onChange={(e) => setExampleText(e.target.value)}
                  disabled={!canManageJS}
                  placeholder='{"id":"ord_1"}'
                  spellCheck={false}
                />
              </div>
              <div className="nc-form-row">
                <label htmlFor="catalog-deprecated">
                  <input
                    id="catalog-deprecated"
                    type="checkbox"
                    checked={deprecated}
                    onChange={(e) => setDeprecated(e.target.checked)}
                    disabled={!canManageJS}
                  />{" "}
                  {t("catalog.deprecated")}
                </label>
              </div>
              {deprecated && (
                <>
                  <div className="nc-form-row">
                    <label htmlFor="catalog-successor">{t("catalog.successor")}</label>
                    <input
                      id="catalog-successor"
                      value={successorSubject}
                      onChange={(e) => setSuccessorSubject(e.target.value)}
                      disabled={!canManageJS}
                      placeholder="orders.created"
                    />
                  </div>
                  <div className="nc-form-row">
                    <label htmlFor="catalog-deprecation-note">{t("catalog.deprecationNote")}</label>
                    <textarea
                      id="catalog-deprecation-note"
                      rows={2}
                      value={deprecationNote}
                      onChange={(e) => setDeprecationNote(e.target.value)}
                      disabled={!canManageJS}
                      placeholder={t("catalog.deprecationNotePlaceholder")}
                    />
                  </div>
                </>
              )}

              {canManageJS && (
                <div className="nc-form-actions">
                  <button
                    type="button"
                    className="btn btn--primary"
                    disabled={saveMutation.isPending}
                    onClick={() => saveMutation.mutate()}
                  >
                    {t("catalog.save")}
                  </button>
                  {selected.documented && (
                    <button
                      type="button"
                      className="btn btn--ghost"
                      disabled={clearMutation.isPending}
                      onClick={() => clearMutation.mutate()}
                    >
                      {t("catalog.clearDocs")}
                    </button>
                  )}
                </div>
              )}

              <div className="nc-catalog-section">
                <h3>{t("catalog.streams")}</h3>
                {selected.streams.length === 0 ? (
                  <p className="text-muted">{t("catalog.noStreams")}</p>
                ) : (
                  <ul className="nc-catalog-links">
                    {selected.streams.map((stream) => (
                      <li key={stream}>
                        <Link to={eventCatalogStreamHref(stream, clusterId, accountName)}>
                          {stream}
                        </Link>
                      </li>
                    ))}
                  </ul>
                )}
              </div>

              <div className="nc-catalog-section">
                <h3>{t("catalog.consumers")}</h3>
                {selected.consumers.length === 0 ? (
                  <p className="text-muted">{t("catalog.noConsumers")}</p>
                ) : (
                  <table className="data-table nc-catalog-consumers">
                    <thead>
                      <tr>
                        <th>{t("catalog.consumerName")}</th>
                        <th>{t("catalog.consumerStream")}</th>
                        <th>{t("catalog.consumerService")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {selected.consumers.map((c) => (
                        <tr key={`${c.stream}:${c.name}`}>
                          <td>
                            <Link to={eventCatalogConsumerHref(c, clusterId, accountName)}>
                              {c.name}
                            </Link>
                          </td>
                          <td>{c.stream}</td>
                          <td>{c.service || "—"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </>
          )}
        </section>
      </div>
    </div>
  );
}
