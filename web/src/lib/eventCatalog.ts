import { api, clusterPath, jetStreamUIBase } from "./api";

export type EventCatalogConsumer = {
  name: string;
  stream: string;
  service?: string;
};

export type EventCatalogEntry = {
  subject: string;
  owner?: string;
  description?: string;
  schema?: Record<string, unknown> | null;
  example?: Record<string, unknown> | null;
  deprecated?: boolean;
  successorSubject?: string;
  deprecationNote?: string;
  updatedBy?: string;
  createdAt?: string;
  updatedAt?: string;
  streams: string[];
  consumers: EventCatalogConsumer[];
  documented: boolean;
  orphan: boolean;
};

export type EventCatalogTotals = {
  total: number;
  documented: number;
  undocumented: number;
  orphan: number;
};

export type EventCatalogSnapshot = {
  capturedAt?: string;
  entries: EventCatalogEntry[];
  totals: EventCatalogTotals;
};

export type EventCatalogUpsert = {
  owner: string;
  description: string;
  schema?: Record<string, unknown> | null;
  example?: Record<string, unknown> | null;
  deprecated?: boolean;
  successorSubject?: string;
  deprecationNote?: string;
};

export type EventCatalogDoc = {
  subject: string;
  owner?: string;
  description?: string;
  schema?: Record<string, unknown> | null;
  example?: Record<string, unknown> | null;
  deprecated?: boolean;
  successorSubject?: string;
  deprecationNote?: string;
  updatedBy?: string;
  createdAt?: string;
  updatedAt?: string;
};

const EMPTY_TOTALS: EventCatalogTotals = {
  total: 0,
  documented: 0,
  undocumented: 0,
  orphan: 0,
};

export const EVENT_CATALOG_HREF = "/docs/event-catalog";
export const EVENT_WIKIPEDIA_FROM_CATALOG_HREF = "/docs/event-wikipedia";

export function eventCatalogWikipediaHref(subject: string): string {
  return `${EVENT_WIKIPEDIA_FROM_CATALOG_HREF}?subject=${encodeURIComponent(subject)}`;
}

export async function fetchEventCatalog(
  clusterId: string,
  options?: { fresh?: boolean },
): Promise<EventCatalogSnapshot> {
  const fresh = options?.fresh ? "?fresh=1" : "";
  const snap = await api<EventCatalogSnapshot>(clusterPath(clusterId, `/event-catalog${fresh}`));
  const data = snap.data;
  return {
    capturedAt: data?.capturedAt,
    entries: Array.isArray(data?.entries) ? data.entries : [],
    totals: data?.totals ?? EMPTY_TOTALS,
  };
}

export async function upsertEventCatalogEntry(
  clusterId: string,
  subject: string,
  body: EventCatalogUpsert,
): Promise<EventCatalogDoc> {
  return (
    await api<EventCatalogDoc>(
      clusterPath(clusterId, `/event-catalog/${encodeURIComponent(subject)}`),
      { method: "PUT", body: JSON.stringify(body) },
    )
  ).data;
}

export async function deleteEventCatalogEntry(clusterId: string, subject: string): Promise<void> {
  await api(clusterPath(clusterId, `/event-catalog/${encodeURIComponent(subject)}`), {
    method: "DELETE",
  });
}

export function filterEventCatalogEntries(
  entries: EventCatalogEntry[],
  query: string,
): EventCatalogEntry[] {
  const q = query.trim().toLowerCase();
  if (!q) return entries;
  return entries.filter((e) => {
    const hay = [e.subject, e.owner ?? "", e.description ?? "", ...(e.streams ?? [])]
      .join(" ")
      .toLowerCase();
    return hay.includes(q);
  });
}

export function sortEventCatalogEntries(entries: EventCatalogEntry[]): EventCatalogEntry[] {
  return [...entries].sort((a, b) => a.subject.localeCompare(b.subject));
}

export function eventCatalogConsumerHref(
  consumer: EventCatalogConsumer,
  clusterId: string,
  accountName = "Default",
): string {
  const base = jetStreamUIBase(clusterId, accountName);
  return `${base}/streams/${encodeURIComponent(consumer.stream)}/consumers/${encodeURIComponent(consumer.name)}`;
}

export function eventCatalogStreamHref(
  stream: string,
  clusterId: string,
  accountName = "Default",
): string {
  return `${jetStreamUIBase(clusterId, accountName)}/streams/${encodeURIComponent(stream)}`;
}

export function formatEventCatalogSchema(schema: Record<string, unknown> | null | undefined): string {
  if (!schema || typeof schema !== "object") return "";
  try {
    return JSON.stringify(schema, null, 2);
  } catch {
    return "";
  }
}

export function parseEventCatalogSchema(text: string): {
  schema: Record<string, unknown> | null;
  error?: string;
} {
  const trimmed = text.trim();
  if (!trimmed) return { schema: null };
  try {
    const parsed = JSON.parse(trimmed) as unknown;
    if (parsed === null) return { schema: null };
    if (typeof parsed !== "object" || Array.isArray(parsed)) {
      return { schema: null, error: "Schema must be a JSON object" };
    }
    return { schema: parsed as Record<string, unknown> };
  } catch {
    return { schema: null, error: "Schema must be valid JSON" };
  }
}
