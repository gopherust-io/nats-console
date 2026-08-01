import { api, clusterPath } from "./api";
import {
  EVENT_CATALOG_HREF,
  eventCatalogConsumerHref,
  eventCatalogStreamHref,
  formatEventCatalogSchema,
  parseEventCatalogSchema,
  type EventCatalogConsumer,
} from "./eventCatalog";

export type EventWikipediaHistory = {
  createdAt?: string;
  updatedAt?: string;
  updatedBy?: string;
  streams: string[];
};

export type EventWikipediaDeprecation = {
  deprecated: boolean;
  successorSubject?: string;
  note?: string;
};

export type EventWikipediaIncidentLink = {
  stream: string;
  consumer: string;
  service?: string;
};

export type EventWikipediaPage = {
  subject: string;
  purpose?: string;
  history: EventWikipediaHistory;
  owner?: string;
  consumers: EventCatalogConsumer[];
  example?: Record<string, unknown> | null;
  schema?: Record<string, unknown> | null;
  relatedEvents: string[];
  knownIncidents: EventWikipediaIncidentLink[];
  deprecation: EventWikipediaDeprecation;
  documented: boolean;
  orphan: boolean;
};

export type EventWikipediaTotals = {
  total: number;
  documented: number;
  deprecated: number;
  orphan: number;
};

export type EventWikipediaSnapshot = {
  capturedAt?: string;
  pages: EventWikipediaPage[];
  totals: EventWikipediaTotals;
};

const EMPTY_TOTALS: EventWikipediaTotals = {
  total: 0,
  documented: 0,
  deprecated: 0,
  orphan: 0,
};

export const EVENT_WIKIPEDIA_HREF = "/docs/event-wikipedia";

export async function fetchEventWikipedia(
  clusterId: string,
  options?: { fresh?: boolean; subject?: string },
): Promise<EventWikipediaSnapshot> {
  const params = new URLSearchParams();
  if (options?.fresh) params.set("fresh", "1");
  if (options?.subject?.trim()) params.set("subject", options.subject.trim());
  const qs = params.toString() ? `?${params.toString()}` : "";
  const snap = await api<EventWikipediaSnapshot>(clusterPath(clusterId, `/event-wikipedia${qs}`));
  const data = snap.data;
  return {
    capturedAt: data?.capturedAt,
    pages: Array.isArray(data?.pages) ? data.pages : [],
    totals: data?.totals ?? EMPTY_TOTALS,
  };
}

export function filterEventWikipediaPages(
  pages: EventWikipediaPage[],
  query: string,
): EventWikipediaPage[] {
  const q = query.trim().toLowerCase();
  if (!q) return pages;
  return pages.filter((p) => {
    const hay = [
      p.subject,
      p.owner ?? "",
      p.purpose ?? "",
      p.deprecation.successorSubject ?? "",
      ...(p.history.streams ?? []),
      ...(p.relatedEvents ?? []),
    ]
      .join(" ")
      .toLowerCase();
    return hay.includes(q);
  });
}

export function sortEventWikipediaPages(pages: EventWikipediaPage[]): EventWikipediaPage[] {
  return [...pages].sort((a, b) => a.subject.localeCompare(b.subject));
}

export function eventWikipediaCatalogHref(subject: string): string {
  return `${EVENT_CATALOG_HREF}?q=${encodeURIComponent(subject)}`;
}

export function eventWikipediaSubjectHref(subject: string): string {
  return `${EVENT_WIKIPEDIA_HREF}?subject=${encodeURIComponent(subject)}`;
}

export function eventWikipediaIncidentHref(
  clusterId: string,
  incident: EventWikipediaIncidentLink,
): string {
  const params = new URLSearchParams({
    cluster: clusterId,
    stream: incident.stream,
    consumer: incident.consumer,
  });
  return `/admin/audit?${params.toString()}`;
}

export {
  EVENT_CATALOG_HREF,
  eventCatalogConsumerHref,
  eventCatalogStreamHref,
  formatEventCatalogSchema,
  parseEventCatalogSchema,
};
