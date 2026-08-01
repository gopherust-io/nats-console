export type PublishFormat = "json" | "msgpack" | "cbor" | "protobuf";

export function contentTypeFor(format: PublishFormat): string {
  switch (format) {
    case "msgpack":
      return "application/msgpack";
    case "cbor":
      return "application/cbor";
    case "protobuf":
      return "application/protobuf";
    case "json":
    default:
      return "application/json";
  }
}

export function encodeJsonPayload(parsed: unknown): Uint8Array {
  return new TextEncoder().encode(JSON.stringify(parsed));
}

/** Strict base64 decode (standard + URL-safe). Rejects bad alphabet, padding, and length. */
export function decodeBase64Bytes(raw: string): Uint8Array {
  const normalized = raw.replace(/\s+/g, "");
  if (!normalized) throw new Error("empty");
  if (normalized.length % 4 === 1) throw new Error("invalid base64");
  if (!/^[A-Za-z0-9+/_-]+={0,2}$/.test(normalized)) throw new Error("invalid base64");
  if (/=/.test(normalized.slice(0, -2))) throw new Error("invalid base64");
  if ((normalized.match(/=/g) ?? []).length > 2) throw new Error("invalid base64");

  const standard = normalized.replace(/-/g, "+").replace(/_/g, "/");
  try {
    const binary = atob(standard);
    const out = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) out[i] = binary.charCodeAt(i);
    return out;
  } catch {
    throw new Error("invalid base64");
  }
}

export function bytesToBase64(bytes: Uint8Array): string {
  let binary = "";
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return btoa(binary);
}

export function encodePublishPayload(input: {
  format: PublishFormat;
  jsonText?: string;
  binaryText?: string;
}): { data: string; contentType: string } {
  const contentType = contentTypeFor(input.format);

  if (input.format === "msgpack" || input.format === "cbor" || input.format === "protobuf") {
    const bytes = decodeBase64Bytes(input.binaryText ?? "");
    return { data: bytesToBase64(bytes), contentType };
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(input.jsonText ?? "");
  } catch {
    throw new Error("invalid-json");
  }

  return { data: bytesToBase64(encodeJsonPayload(parsed)), contentType };
}
