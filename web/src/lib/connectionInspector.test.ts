import { describe, expect, it } from "vitest";
import {
  formatConnectedSince,
  formatTLSVersion,
  isSlowConsumerConnection,
  parseRttMs,
  connectionUsername,
} from "./connectionInspector";

describe("isSlowConsumerConnection", () => {
  it("detects explicit booleans and reason text", () => {
    expect(isSlowConsumerConnection({ slow_consumer: true })).toBe(true);
    expect(isSlowConsumerConnection({ is_slow_consumer: true })).toBe(true);
    expect(isSlowConsumerConnection({ reason: "Slow Consumer" })).toBe(true);
    expect(isSlowConsumerConnection({ reason: "ok" })).toBe(false);
  });

  it("detects stalls and high pending bytes", () => {
    expect(isSlowConsumerConnection({ stalls: 1 })).toBe(true);
    expect(isSlowConsumerConnection({ pending_bytes: 2 * 1024 * 1024 })).toBe(true);
    expect(isSlowConsumerConnection({ stalls: 0, pending_bytes: 100 })).toBe(false);
  });
});

describe("formatTLSVersion", () => {
  it("formats tls fields including cipher_suite", () => {
    expect(formatTLSVersion({})).toBe("—");
    expect(formatTLSVersion({ tls_version: "1.3" })).toBe("1.3");
    expect(formatTLSVersion({ tls_version: "1.3", tls_cipher: "TLS_AES_128_GCM_SHA256" })).toBe(
      "1.3 (TLS_AES_128_GCM_SHA256)",
    );
    expect(
      formatTLSVersion({ tls_version: "1.3", tls_cipher_suite: "TLS_AES_256_GCM_SHA384" }),
    ).toBe("1.3 (TLS_AES_256_GCM_SHA384)");
  });
});

describe("formatConnectedSince", () => {
  it("handles empty, invalid and valid timestamps", () => {
    expect(formatConnectedSince(undefined)).toBe("—");
    expect(formatConnectedSince("not-a-date")).toBe("not-a-date");
    const formatted = formatConnectedSince("2026-01-01T00:00:00Z");
    expect(formatted.length).toBeGreaterThan(0);
    expect(formatted).not.toMatch(/\d:\d{2}:\d{2}/); // no seconds
  });
});

describe("parseRttMs", () => {
  it("parses units into milliseconds", () => {
    expect(parseRttMs("9ms")).toBe(9);
    expect(parseRttMs("100ms")).toBe(100);
    expect(parseRttMs("500µs")).toBe(0.5);
    expect(parseRttMs("1s")).toBe(1000);
    expect(parseRttMs(undefined)).toBeNull();
    expect(parseRttMs("bogus")).toBeNull();
  });

  it("orders numerically unlike string sort", () => {
    const values = ["100ms", "9ms", "1s", "500µs"];
    const sorted = [...values].sort((a, b) => (parseRttMs(a) ?? 0) - (parseRttMs(b) ?? 0));
    expect(sorted).toEqual(["500µs", "9ms", "100ms", "1s"]);
  });
});

describe("connectionUsername", () => {
  it("prefers authorized_user", () => {
    expect(connectionUsername({ authorized_user: "a", user: "b" })).toBe("a");
    expect(connectionUsername({ user: "b" })).toBe("b");
    expect(connectionUsername({})).toBeUndefined();
  });
});
