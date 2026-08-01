import type { PublishFormat } from "./publishEncode";
import { STORAGE_KEYS } from "./constants";

export type PayloadTemplate = {
  id: string;
  name: string;
  subject?: string;
  format: PublishFormat;
  payload: string;
  binaryPayload?: string;
  clusterId?: string;
  stream?: string;
  updatedAt: string;
};

function isPublishFormat(value: unknown): value is PublishFormat {
  return value === "json" || value === "msgpack" || value === "cbor" || value === "protobuf";
}

export function readPayloadTemplates(): PayloadTemplate[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEYS.payloadTemplates);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((item): item is PayloadTemplate => {
      if (!item || typeof item !== "object") return false;
      const t = item as PayloadTemplate;
      return (
        typeof t.id === "string" &&
        typeof t.name === "string" &&
        isPublishFormat(t.format) &&
        typeof t.payload === "string" &&
        typeof t.updatedAt === "string"
      );
    });
  } catch {
    return [];
  }
}

export function writePayloadTemplates(templates: PayloadTemplate[]): void {
  localStorage.setItem(STORAGE_KEYS.payloadTemplates, JSON.stringify(templates));
}

export function templatesForStream(
  clusterId: string | null | undefined,
  stream: string | null | undefined,
  templates = readPayloadTemplates(),
): PayloadTemplate[] {
  return templates.filter((t) => {
    if (t.clusterId && clusterId && t.clusterId !== clusterId) return false;
    if (t.stream && stream && t.stream !== stream) return false;
    return true;
  });
}

export function savePayloadTemplate(
  input: Omit<PayloadTemplate, "id" | "updatedAt"> & { id?: string },
): PayloadTemplate[] {
  const current = readPayloadTemplates();
  const updatedAt = new Date().toISOString();
  const id = input.id ?? crypto.randomUUID();
  const nextItem: PayloadTemplate = {
    id,
    name: input.name.trim() || "Untitled",
    subject: input.subject,
    format: input.format,
    payload: input.payload,
    binaryPayload: input.binaryPayload,
    clusterId: input.clusterId,
    stream: input.stream,
    updatedAt,
  };
  const idx = current.findIndex((t) => t.id === id);
  const next =
    idx >= 0 ? current.map((t, i) => (i === idx ? nextItem : t)) : [...current, nextItem];
  writePayloadTemplates(next);
  return next;
}

export function deletePayloadTemplate(id: string): PayloadTemplate[] {
  const next = readPayloadTemplates().filter((t) => t.id !== id);
  writePayloadTemplates(next);
  return next;
}
