import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useSearchParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import QueryErrorState from "../components/ui/QueryErrorState";
import { useAccount } from "../lib/account";
import { useCluster } from "../lib/cluster";
import { MONITORING_POLL_MS } from "../lib/constants";
import {
  eventCatalogConsumerHref,
  eventCatalogStreamHref,
  formatEventCatalogSchema,
} from "../lib/eventCatalog";
import {
  eventWikipediaCatalogHref,
  eventWikipediaIncidentHref,
  eventWikipediaSubjectHref,
  fetchEventWikipedia,
  filterEventWikipediaPages,
  sortEventWikipediaPages,
  type EventWikipediaPage,
} from "../lib/eventWikipedia";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";

function formatWhen(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

export default function EventWikipediaPage() {
  const { t } = useTranslation();
  const { clusterId, cluster } = useCluster();
  const { accountName } = useAccount();
  const [searchParams, setSearchParams] = useSearchParams();
  const [search, setSearch] = useState(searchParams.get("q") ?? "");
  const [selectedSubject, setSelectedSubject] = useState<string | null>(
    searchParams.get("subject"),
  );

  const wikiQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "event-wikipedia"),
    queryFn: () => fetchEventWikipedia(clusterId!, { fresh: true }),
    enabled: Boolean(clusterId),
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS),
  });

  const pages = useMemo(
    () => sortEventWikipediaPages(wikiQuery.data?.pages ?? []),
    [wikiQuery.data?.pages],
  );
  const filtered = useMemo(() => filterEventWikipediaPages(pages, search), [pages, search]);
  const totals = wikiQuery.data?.totals;

  const selected: EventWikipediaPage | null = useMemo(() => {
    if (!selectedSubject) return filtered[0] ?? pages[0] ?? null;
    return pages.find((p) => p.subject === selectedSubject) ?? filtered[0] ?? null;
  }, [selectedSubject, pages, filtered]);

  useEffect(() => {
    const q = searchParams.get("q") ?? "";
    const subject = searchParams.get("subject");
    setSearch(q);
    if (subject) setSelectedSubject(subject);
  }, [searchParams]);

  useEffect(() => {
    if (selected && !filtered.some((p) => p.subject === selected.subject) && filtered[0]) {
      setSelectedSubject(filtered[0].subject);
    }
  }, [filtered, selected]);

  const selectSubject = (subject: string) => {
    setSelectedSubject(subject);
    const next = new URLSearchParams(searchParams);
    next.set("subject", subject);
    setSearchParams(next, { replace: true });
  };

  if (!clusterId) {
    return (
      <div className="nc-page">
        <PageHeader title={t("wikipedia.title")} subtitle={t("wikipedia.needSystem")} />
        <EmptyState title={t("wikipedia.needSystem")} />
      </div>
    );
  }

  return (
    <div className="nc-page">
      <PageHeader
        title={t("wikipedia.title")}
        subtitle={t("wikipedia.subtitle", { system: cluster?.name ?? clusterId })}
        actions={
          <button
            type="button"
            className="btn btn--ghost btn--small"
            onClick={() => void wikiQuery.refetch()}
            disabled={wikiQuery.isFetching}
          >
            {t("common.refresh")}
          </button>
        }
      />

      {wikiQuery.isError && (
        <QueryErrorState error={wikiQuery.error} onRetry={() => void wikiQuery.refetch()} />
      )}

      {totals && (
        <div className="nc-catalog-stats" aria-label={t("wikipedia.statsAria")}>
          <span>
            {t("wikipedia.statTotal")}: <strong>{totals.total}</strong>
          </span>
          <span>
            {t("wikipedia.statDocumented")}: <strong>{totals.documented}</strong>
          </span>
          <span>
            {t("wikipedia.statDeprecated")}: <strong>{totals.deprecated}</strong>
          </span>
          <span>
            {t("wikipedia.statOrphan")}: <strong>{totals.orphan}</strong>
          </span>
        </div>
      )}

      <div className="nc-catalog-layout">
        <aside className="nc-catalog-list" aria-label={t("wikipedia.listAria")}>
          <input
            className="nc-catalog-search"
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("wikipedia.searchPlaceholder")}
            aria-label={t("wikipedia.searchPlaceholder")}
          />
          {filtered.length === 0 ? (
            <EmptyState title={t("wikipedia.empty")} />
          ) : (
            <ul className="nc-catalog-entries">
              {filtered.map((page) => {
                const active = selected?.subject === page.subject;
                return (
                  <li key={page.subject}>
                    <button
                      type="button"
                      className={`nc-catalog-entry${active ? " is-active" : ""}`}
                      onClick={() => selectSubject(page.subject)}
                    >
                      <span className="nc-catalog-entry__subject">{page.subject}</span>
                      <span className="nc-catalog-entry__meta">
                        {page.owner || t("wikipedia.noOwner")}
                      </span>
                      <span className="nc-catalog-entry__badges">
                        {page.deprecation.deprecated && (
                          <span className="badge">{t("wikipedia.badgeDeprecated")}</span>
                        )}
                        {!page.documented && (
                          <span className="badge">{t("wikipedia.badgeUndocumented")}</span>
                        )}
                        {page.orphan && (
                          <span className="badge badge--muted">{t("wikipedia.badgeOrphan")}</span>
                        )}
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </aside>

        <section className="nc-catalog-detail" aria-label={t("wikipedia.detailAria")}>
          {!selected ? (
            <EmptyState title={t("wikipedia.selectEvent")} />
          ) : (
            <WikipediaArticle
              page={selected}
              clusterId={clusterId}
              accountName={accountName}
              onSelectRelated={selectSubject}
            />
          )}
        </section>
      </div>
    </div>
  );
}

function WikipediaArticle({
  page,
  clusterId,
  accountName,
  onSelectRelated,
}: {
  page: EventWikipediaPage;
  clusterId: string;
  accountName: string;
  onSelectRelated: (subject: string) => void;
}) {
  const { t } = useTranslation();
  const schemaText = formatEventCatalogSchema(page.schema);
  const exampleText = formatEventCatalogSchema(page.example);
  const created = formatWhen(page.history.createdAt);
  const updated = formatWhen(page.history.updatedAt);

  return (
    <>
      <header className="nc-catalog-detail__header">
        <h2 className="nc-catalog-detail__subject">{page.subject}</h2>
        {page.deprecation.deprecated && (
          <span className="badge">{t("wikipedia.badgeDeprecated")}</span>
        )}
        {page.orphan && <span className="badge badge--muted">{t("wikipedia.badgeOrphan")}</span>}
        <Link className="btn btn--ghost btn--small" to={eventWikipediaCatalogHref(page.subject)}>
          {t("wikipedia.editInCatalog")}
        </Link>
      </header>

      <WikiSection title={t("wikipedia.purpose")}>
        {page.purpose ? <p>{page.purpose}</p> : <p className="text-muted">{t("wikipedia.noPurpose")}</p>}
      </WikiSection>

      <WikiSection title={t("wikipedia.history")}>
        {!created && !updated && page.history.streams.length === 0 ? (
          <p className="text-muted">{t("wikipedia.noHistory")}</p>
        ) : (
          <dl className="nc-wiki-dl">
            {created && (
              <>
                <dt>{t("wikipedia.createdAt")}</dt>
                <dd>{created}</dd>
              </>
            )}
            {updated && (
              <>
                <dt>{t("wikipedia.updatedAt")}</dt>
                <dd>{updated}</dd>
              </>
            )}
            {page.history.updatedBy && (
              <>
                <dt>{t("wikipedia.updatedBy")}</dt>
                <dd>{page.history.updatedBy}</dd>
              </>
            )}
            <dt>{t("wikipedia.streams")}</dt>
            <dd>
              {page.history.streams.length === 0 ? (
                t("wikipedia.noStreams")
              ) : (
                <ul className="nc-catalog-links">
                  {page.history.streams.map((stream) => (
                    <li key={stream}>
                      <Link to={eventCatalogStreamHref(stream, clusterId, accountName)}>
                        {stream}
                      </Link>
                    </li>
                  ))}
                </ul>
              )}
            </dd>
          </dl>
        )}
      </WikiSection>

      <WikiSection title={t("wikipedia.owner")}>
        {page.owner ? <p>{page.owner}</p> : <p className="text-muted">{t("wikipedia.noOwner")}</p>}
      </WikiSection>

      <WikiSection title={t("wikipedia.consumers")}>
        {page.consumers.length === 0 ? (
          <p className="text-muted">{t("wikipedia.noConsumers")}</p>
        ) : (
          <table className="data-table nc-catalog-consumers">
            <thead>
              <tr>
                <th>{t("wikipedia.consumerName")}</th>
                <th>{t("wikipedia.consumerStream")}</th>
                <th>{t("wikipedia.consumerService")}</th>
              </tr>
            </thead>
            <tbody>
              {page.consumers.map((c) => (
                <tr key={`${c.stream}:${c.name}`}>
                  <td>
                    <Link to={eventCatalogConsumerHref(c, clusterId, accountName)}>{c.name}</Link>
                  </td>
                  <td>{c.stream}</td>
                  <td>{c.service || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </WikiSection>

      <WikiSection title={t("wikipedia.examples")}>
        {exampleText ? (
          <pre className="nc-catalog-schema">{exampleText}</pre>
        ) : (
          <p className="text-muted">{t("wikipedia.noExample")}</p>
        )}
      </WikiSection>

      <WikiSection title={t("wikipedia.schema")}>
        {schemaText ? (
          <pre className="nc-catalog-schema">{schemaText}</pre>
        ) : (
          <p className="text-muted">{t("wikipedia.noSchema")}</p>
        )}
      </WikiSection>

      <WikiSection title={t("wikipedia.relatedEvents")}>
        {page.relatedEvents.length === 0 ? (
          <p className="text-muted">{t("wikipedia.noRelated")}</p>
        ) : (
          <ul className="nc-catalog-links">
            {page.relatedEvents.map((subj) => (
              <li key={subj}>
                <Link to={eventWikipediaSubjectHref(subj)} onClick={() => onSelectRelated(subj)}>
                  {subj}
                </Link>
              </li>
            ))}
          </ul>
        )}
      </WikiSection>

      <WikiSection title={t("wikipedia.knownIncidents")}>
        {page.knownIncidents.length === 0 ? (
          <p className="text-muted">{t("wikipedia.noIncidents")}</p>
        ) : (
          <ul className="nc-catalog-links">
            {page.knownIncidents.map((inc) => (
              <li key={`${inc.stream}:${inc.consumer}`}>
                <Link to={eventWikipediaIncidentHref(clusterId, inc)}>
                  {inc.stream} / {inc.consumer}
                  {inc.service ? ` (${inc.service})` : ""}
                </Link>
              </li>
            ))}
          </ul>
        )}
      </WikiSection>

      <WikiSection title={t("wikipedia.deprecationStatus")}>
        {!page.deprecation.deprecated ? (
          <p>{t("wikipedia.notDeprecated")}</p>
        ) : (
          <dl className="nc-wiki-dl">
            <dt>{t("wikipedia.badgeDeprecated")}</dt>
            <dd>{t("wikipedia.deprecatedYes")}</dd>
            {page.deprecation.successorSubject && (
              <>
                <dt>{t("wikipedia.successor")}</dt>
                <dd>
                  <Link
                    to={eventWikipediaSubjectHref(page.deprecation.successorSubject)}
                    onClick={() => onSelectRelated(page.deprecation.successorSubject!)}
                  >
                    {page.deprecation.successorSubject}
                  </Link>
                </dd>
              </>
            )}
            {page.deprecation.note && (
              <>
                <dt>{t("wikipedia.deprecationNote")}</dt>
                <dd>{page.deprecation.note}</dd>
              </>
            )}
          </dl>
        )}
      </WikiSection>
    </>
  );
}

function WikiSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="nc-catalog-section">
      <h3>{title}</h3>
      {children}
    </div>
  );
}
