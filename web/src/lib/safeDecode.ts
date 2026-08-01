/** Safely decode a URI component; returns input unchanged if already decoded or invalid. */
export function safeDecodeURIComponent(value: string): string {
  if (!value) return value;
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}
