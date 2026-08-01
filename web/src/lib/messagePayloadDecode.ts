import { formatMessagePayload } from "./api";
import { bytesToBase64 } from "./publishEncode";

export type PayloadWireFormat = "json" | "msgpack" | "cbor" | "protobuf" | "bytes";

export type DecodedMessagePayload = {
  format: PayloadWireFormat;
  /** Human-readable representation for text/JSON/CSV/PDF exports. */
  text: string;
  /** Raw message bytes. */
  bytes: Uint8Array;
  /** Preferred native file extension without dot. */
  nativeExt: string;
  mimeType: string;
};

type MsgpackDecode = (bytes: Uint8Array) => unknown;
type CborDecoderLike = {
  decodeMultiple: (bytes: Uint8Array, callback: (value: unknown) => void) => void;
};

let decodeMsgpackFn: MsgpackDecode | null = null;
let cborDecoder: CborDecoderLike | null = null;
let codecsPromise: Promise<void> | null = null;

/** Dynamically load msgpack/cbor codecs (once). Call before sniffing/decoding binary formats. */
export function loadBinaryCodecs(): Promise<void> {
  if (decodeMsgpackFn && cborDecoder) return Promise.resolve();
  codecsPromise ??= (async () => {
    const [mp, cx] = await Promise.all([import("@msgpack/msgpack"), import("cbor-x")]);
    decodeMsgpackFn = mp.decode as MsgpackDecode;
    cborDecoder = new cx.Decoder({ mapsAsObjects: true, useRecords: false });
  })();
  return codecsPromise;
}

const WIRE_VARINT = 0;
const WIRE_64BIT = 1;
const WIRE_LEN = 2;
const WIRE_32BIT = 5;

const PREVIEW_CHARS = 160;

function contentTypeOf(headers?: Record<string, string>): string {
  if (!headers) return "";
  const raw =
    headers["Content-Type"] ??
    headers["content-type"] ??
    headers["Nats-Content-Type"] ??
    "";
  return raw.split(";")[0]?.trim().toLowerCase() ?? "";
}

/** Header-declared format, or null when Content-Type is absent/unrecognized. */
export function detectPayloadFormatFromHeaders(
  headers?: Record<string, string>,
): PayloadWireFormat | null {
  const ct = contentTypeOf(headers);
  if (!ct) return null;
  if (ct === "application/msgpack" || ct === "application/x-msgpack" || ct === "msgpack") {
    return "msgpack";
  }
  if (ct === "application/cbor" || ct === "application/cbor-seq") return "cbor";
  if (
    ct === "application/protobuf" ||
    ct === "application/x-protobuf" ||
    ct === "application/vnd.google.protobuf" ||
    ct === "protobuf"
  ) {
    return "protobuf";
  }
  if (ct === "application/json" || ct.endsWith("+json") || ct === "json") return "json";
  return null;
}

function isMostlyPrintableAscii(bytes: Uint8Array): boolean {
  if (bytes.length === 0) return true;
  let printable = 0;
  for (let i = 0; i < bytes.length; i += 1) {
    const b = bytes[i]!;
    if (b === 0x09 || b === 0x0a || b === 0x0d || (b >= 0x20 && b < 0x7f)) {
      printable += 1;
    }
  }
  return printable / bytes.length >= 0.85;
}

function tryDecodeUtf8(bytes: Uint8Array): string | null {
  try {
    return new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  } catch {
    return null;
  }
}

function looksLikeJsonText(text: string): boolean {
  const trimmed = text.trimStart();
  if (!trimmed.startsWith("{") && !trimmed.startsWith("[")) return false;
  try {
    JSON.parse(text);
    return true;
  } catch {
    return false;
  }
}

function isStructuredValue(value: unknown): boolean {
  return value !== null && typeof value === "object";
}

function tryDecodeMsgpackFull(bytes: Uint8Array): unknown | null {
  if (bytes.length === 0 || !decodeMsgpackFn) return null;
  try {
    const value = decodeMsgpackFn(bytes);
    if (isStructuredValue(value)) return value;
    if (isMostlyPrintableAscii(bytes)) return null;
    return value;
  } catch {
    return null;
  }
}

function decodeCborValues(bytes: Uint8Array): unknown[] {
  if (!cborDecoder) return [];
  const values: unknown[] = [];
  cborDecoder.decodeMultiple(bytes, (value) => {
    values.push(value);
  });
  return values;
}

function tryDecodeCborFull(bytes: Uint8Array): unknown | null {
  if (bytes.length === 0 || !cborDecoder) return null;
  try {
    const values = decodeCborValues(bytes);
    if (values.length !== 1) return null;
    const value = values[0];
    if (isStructuredValue(value)) return value;
    if (isMostlyPrintableAscii(bytes)) return null;
    return value;
  } catch {
    return null;
  }
}

function readVarint(
  bytes: Uint8Array,
  offset: number,
): { value: number; next: number } | null {
  let result = 0;
  let shift = 0;
  let pos = offset;
  while (pos < bytes.length && shift <= 35) {
    const b = bytes[pos]!;
    pos += 1;
    result |= (b & 0x7f) << shift;
    if ((b & 0x80) === 0) {
      return { value: result >>> 0, next: pos };
    }
    shift += 7;
  }
  return null;
}

function wireTypeName(wireType: number): string {
  switch (wireType) {
    case WIRE_VARINT:
      return "varint";
    case WIRE_64BIT:
      return "64bit";
    case WIRE_LEN:
      return "len";
    case WIRE_32BIT:
      return "32bit";
    default:
      return `wire${wireType}`;
  }
}

function formatLenPayload(slice: Uint8Array): string {
  const text = tryDecodeUtf8(slice);
  if (text !== null && isMostlyPrintableAscii(slice)) {
    return JSON.stringify(text);
  }
  return [...slice].map((b) => b.toString(16).padStart(2, "0")).join(" ");
}

/** Walk protobuf wire format; returns pretty lines or null if invalid. */
export function parseProtobufWire(bytes: Uint8Array): string[] | null {
  if (bytes.length === 0) return null;
  if (isMostlyPrintableAscii(bytes) && tryDecodeUtf8(bytes) !== null) return null;

  const lines: string[] = [];
  let pos = 0;
  let fields = 0;

  while (pos < bytes.length) {
    const tag = readVarint(bytes, pos);
    if (!tag) return null;
    pos = tag.next;
    const fieldNumber = tag.value >>> 3;
    const wireType = tag.value & 0x7;
    if (fieldNumber === 0) return null;

    switch (wireType) {
      case WIRE_VARINT: {
        const v = readVarint(bytes, pos);
        if (!v) return null;
        pos = v.next;
        lines.push(`${fieldNumber}: ${wireTypeName(wireType)} = ${v.value}`);
        break;
      }
      case WIRE_64BIT: {
        if (pos + 8 > bytes.length) return null;
        const hex = [...bytes.subarray(pos, pos + 8)]
          .map((b) => b.toString(16).padStart(2, "0"))
          .join(" ");
        pos += 8;
        lines.push(`${fieldNumber}: ${wireTypeName(wireType)} = ${hex}`);
        break;
      }
      case WIRE_32BIT: {
        if (pos + 4 > bytes.length) return null;
        const hex = [...bytes.subarray(pos, pos + 4)]
          .map((b) => b.toString(16).padStart(2, "0"))
          .join(" ");
        pos += 4;
        lines.push(`${fieldNumber}: ${wireTypeName(wireType)} = ${hex}`);
        break;
      }
      case WIRE_LEN: {
        const len = readVarint(bytes, pos);
        if (!len) return null;
        pos = len.next;
        if (pos + len.value > bytes.length) return null;
        const slice = bytes.subarray(pos, pos + len.value);
        pos += len.value;
        lines.push(`${fieldNumber}: ${wireTypeName(wireType)}(${len.value}) = ${formatLenPayload(slice)}`);
        break;
      }
      default:
        return null;
    }
    fields += 1;
  }

  return fields > 0 ? lines : null;
}

export async function detectPayloadFormatFromBytes(bytes: Uint8Array): Promise<PayloadWireFormat> {
  if (bytes.length === 0) return "bytes";

  const text = tryDecodeUtf8(bytes);
  if (text !== null && looksLikeJsonText(text)) return "json";

  await loadBinaryCodecs();
  if (tryDecodeCborFull(bytes) !== null) return "cbor";
  if (tryDecodeMsgpackFull(bytes) !== null) return "msgpack";
  if (parseProtobufWire(bytes) !== null) return "protobuf";

  return "bytes";
}

/**
 * Prefer Content-Type / Nats-Content-Type when present; otherwise sniff bytes.
 * When headers are absent/unrecognized and bytes are omitted, returns "bytes".
 */
export async function detectPayloadFormat(
  headers?: Record<string, string>,
  bytes?: Uint8Array,
): Promise<PayloadWireFormat> {
  const fromHeader = detectPayloadFormatFromHeaders(headers);
  if (fromHeader) return fromHeader;
  if (bytes) return detectPayloadFormatFromBytes(bytes);
  return "bytes";
}

/**
 * Cheap row preview: truncated UTF-8 when printable, otherwise truncated base64.
 * Does not sniff or load msgpack/cbor.
 */
export function compactPayloadPreview(data: string, maxChars = PREVIEW_CHARS): string {
  if (!data) return "";
  const bytes = messageDataToBytes(data);
  const text = tryDecodeUtf8(bytes);
  if (text !== null && isMostlyPrintableAscii(bytes)) {
    return text.length > maxChars ? `${text.slice(0, maxChars)}…` : text;
  }
  const b64 = data.replace(/\s+/g, "");
  return b64.length > maxChars ? `${b64.slice(0, maxChars)}…` : b64;
}

export function bytesToHexDump(bytes: Uint8Array, bytesPerLine = 16): string {
  if (bytes.length === 0) return "";
  const lines: string[] = [];
  for (let i = 0; i < bytes.length; i += bytesPerLine) {
    const slice = bytes.subarray(i, i + bytesPerLine);
    const hex = [...slice].map((b) => b.toString(16).padStart(2, "0")).join(" ");
    lines.push(hex);
  }
  return lines.join("\n");
}

function prettyJson(value: unknown): string {
  return JSON.stringify(value, (_key, v) => (typeof v === "bigint" ? v.toString() : v), 2);
}

function metaForFormat(format: PayloadWireFormat): { nativeExt: string; mimeType: string } {
  switch (format) {
    case "json":
      return { nativeExt: "json", mimeType: "application/json" };
    case "msgpack":
      return { nativeExt: "msgpack", mimeType: "application/msgpack" };
    case "cbor":
      return { nativeExt: "cbor", mimeType: "application/cbor" };
    case "protobuf":
      return { nativeExt: "pb", mimeType: "application/protobuf" };
    case "bytes":
    default:
      return { nativeExt: "bin", mimeType: "application/octet-stream" };
  }
}

/** Prefer true base64 → bytes without UTF-8 reinterpretation. */
export function messageDataToBytes(data: string): Uint8Array {
  if (!data) return new Uint8Array();
  const normalized = data.replace(/\s+/g, "");
  try {
    const binary = atob(normalized.replace(/-/g, "+").replace(/_/g, "/"));
    const out = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) out[i] = binary.charCodeAt(i);
    return out;
  } catch {
    try {
      return new TextEncoder().encode(data);
    } catch {
      return new Uint8Array();
    }
  }
}

async function prettyPrint(format: PayloadWireFormat, bytes: Uint8Array): Promise<string> {
  if (format === "msgpack" || format === "cbor") {
    await loadBinaryCodecs();
  }

  if (format === "msgpack") {
    try {
      return prettyJson(decodeMsgpackFn!(bytes));
    } catch {
      return bytesToHexDump(bytes);
    }
  }

  if (format === "cbor") {
    try {
      const values = decodeCborValues(bytes);
      if (values.length === 1) {
        return prettyJson(values[0]);
      }
      if (values.length > 1) {
        return prettyJson(values);
      }
    } catch {
      // fall through to hex
    }
    return bytesToHexDump(bytes);
  }

  if (format === "protobuf") {
    const lines = parseProtobufWire(bytes);
    if (lines) return lines.join("\n");
    return bytesToHexDump(bytes);
  }

  if (format === "json") {
    try {
      return formatMessagePayload(new TextDecoder().decode(bytes));
    } catch {
      return bytesToHexDump(bytes) || bytesToBase64(bytes);
    }
  }

  const text = tryDecodeUtf8(bytes);
  if (text !== null && isMostlyPrintableAscii(bytes)) {
    return formatMessagePayload(text);
  }
  return bytesToHexDump(bytes) || bytesToBase64(bytes);
}

const decodeCache = new WeakMap<object, DecodedMessagePayload>();
const decodeCacheByKey = new Map<string, DecodedMessagePayload>();

function cacheKey(data: string, headers?: Record<string, string>): string {
  if (!headers || Object.keys(headers).length === 0) return data;
  return `${data}\0${JSON.stringify(headers)}`;
}

export async function decodeMessagePayload(
  data: string,
  headers?: Record<string, string>,
  cacheHost?: object,
): Promise<DecodedMessagePayload> {
  if (cacheHost) {
    const hit = decodeCache.get(cacheHost);
    if (hit) return hit;
  } else {
    const key = cacheKey(data, headers);
    const hit = decodeCacheByKey.get(key);
    if (hit) return hit;
  }

  const bytes = messageDataToBytes(data);
  const format = await detectPayloadFormat(headers, bytes);
  const meta = metaForFormat(format);
  const decoded: DecodedMessagePayload = {
    format,
    text: await prettyPrint(format, bytes),
    bytes,
    nativeExt: meta.nativeExt,
    mimeType: meta.mimeType,
  };

  if (cacheHost) {
    decodeCache.set(cacheHost, decoded);
  } else if (decodeCacheByKey.size < 64) {
    decodeCacheByKey.set(cacheKey(data, headers), decoded);
  }

  return decoded;
}

export function payloadFormatLabel(format: PayloadWireFormat): string {
  switch (format) {
    case "json":
      return "JSON";
    case "msgpack":
      return "MessagePack";
    case "cbor":
      return "CBOR";
    case "protobuf":
      return "Protobuf";
    case "bytes":
    default:
      return "Binary";
  }
}
