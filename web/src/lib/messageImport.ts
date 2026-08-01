import { MESSAGE_IMPORT_MAX_BATCH } from "./constants";
import { contentTypeFor, encodePublishPayload, type PublishFormat } from "./publishEncode";

export type MessageImportItem = {
  subject: string;
  data: string;
  headers?: Record<string, string>;
};

export type MessageImportResult = {
  items: MessageImportItem[];
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function asString(value: unknown): string | null {
  return typeof value === "string" ? value : null;
}

function asHeaders(value: unknown): Record<string, string> | undefined {
  if (!isRecord(value)) return undefined;
  const out: Record<string, string> = {};
  for (const [key, raw] of Object.entries(value)) {
    if (typeof raw === "string") out[key] = raw;
  }
  return Object.keys(out).length > 0 ? out : undefined;
}

function parseFormat(value: unknown): PublishFormat {
  if (value === "msgpack" || value === "cbor" || value === "protobuf" || value === "json") {
    return value;
  }
  return "json";
}

function itemFromObject(raw: Record<string, unknown>): MessageImportItem {
  const subject = asString(raw.subject)?.trim();
  if (!subject) {
    throw new Error("missing-subject");
  }

  const headers = asHeaders(raw.headers);
  const dataField = asString(raw.data);
  if (dataField != null && dataField.length > 0) {
    return { subject, data: dataField, headers };
  }

  const payload = asString(raw.payload);
  if (payload == null) {
    throw new Error("missing-payload");
  }

  const format = parseFormat(raw.format);
  if (format === "json") {
    const encoded = encodePublishPayload({ format: "json", jsonText: payload });
    return {
      subject,
      data: encoded.data,
      headers: { ...(headers ?? {}), "Content-Type": encoded.contentType },
    };
  }

  const encoded = encodePublishPayload({ format, binaryText: payload });
  return {
    subject,
    data: encoded.data,
    headers: { ...(headers ?? {}), "Content-Type": encoded.contentType || contentTypeFor(format) },
  };
}

/** Parse JSON produced by message export (`toJSON`) or a publish-ready array. */
export function parseMessageImportJSON(text: string, maxBatch = MESSAGE_IMPORT_MAX_BATCH): MessageImportResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    throw new Error("invalid-json");
  }

  const list = Array.isArray(parsed) ? parsed : [parsed];
  if (list.length === 0) {
    throw new Error("empty");
  }
  if (list.length > maxBatch) {
    throw new Error("too-many");
  }

  const items: MessageImportItem[] = [];
  for (const entry of list) {
    if (!isRecord(entry)) {
      throw new Error("invalid-item");
    }
    items.push(itemFromObject(entry));
  }
  return { items };
}

export async function parseMessageImportFile(file: File, maxBatch = MESSAGE_IMPORT_MAX_BATCH): Promise<MessageImportResult> {
  const text = await file.text();
  return parseMessageImportJSON(text, maxBatch);
}
