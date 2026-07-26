import { decodeBase64 } from "./api";

/** Byte length helpers for message payloads (kept separate for React Fast Refresh). */
export function payloadByteLength(data?: string, payload?: string): number {
  if (payload !== undefined) return new TextEncoder().encode(payload).length;
  if (data !== undefined) return new TextEncoder().encode(decodeBase64(data)).length;
  return 0;
}
