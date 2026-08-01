import { describe, expect, it } from "vitest";
import {
  escapeCSVField,
  liveBufferFilename,
  rowFromMessage,
  sanitizeFilenamePart,
  singleMessageFilename,
  toCSV,
  toJSON,
  toText,
} from "./messageDownload";

function b64(text: string): string {
  return btoa(unescape(encodeURIComponent(text)));
}

describe("messageDownload", () => {
  const sample = rowFromMessage({
    seq: 42,
    subject: "orders.created",
    time: "2026-07-27T12:00:00Z",
    data: b64('{"ok":true}'),
    headers: { "Nats-Msg-Id": "abc,def" },
  });

  it("sanitizes filename parts", () => {
    expect(sanitizeFilenamePart("ORDERS/foo")).toBe("ORDERS_foo");
    expect(sanitizeFilenamePart("!!!")).toBe("message");
  });

  it("builds filenames", () => {
    expect(singleMessageFilename("ORDERS", 7, "json")).toBe("ORDERS-seq-7.json");
    expect(liveBufferFilename("ORDERS", "csv", new Date("2026-07-27T01:02:03.004Z"))).toBe(
      "ORDERS-live-2026-07-27T01-02-03-004Z.csv",
    );
  });

  it("escapes CSV fields", () => {
    expect(escapeCSVField("plain")).toBe("plain");
    expect(escapeCSVField('a"b')).toBe('"a""b"');
    expect(escapeCSVField("a,b")).toBe('"a,b"');
    expect(escapeCSVField("a\nb")).toBe('"a\nb"');
  });

  it("builds JSON envelope for a single message", async () => {
    const parsed = JSON.parse(await toJSON([sample]));
    expect(parsed).toEqual({
      seq: 42,
      subject: "orders.created",
      time: "2026-07-27T12:00:00Z",
      headers: { "Nats-Msg-Id": "abc,def" },
      format: "json",
      payload: '{\n  "ok": true\n}',
    });
  });

  it("builds JSON array for multiple messages", async () => {
    const parsed = JSON.parse(await toJSON([sample, { ...sample, seq: 43 }]));
    expect(Array.isArray(parsed)).toBe(true);
    expect(parsed).toHaveLength(2);
    expect(parsed[1].seq).toBe(43);
  });

  it("builds CSV with escaped headers and payload", async () => {
    const csv = await toCSV([sample]);
    expect(csv.startsWith("seq,subject,time,headers,payload\n")).toBe(true);
    expect(csv).toContain("42,orders.created,2026-07-27T12:00:00Z,");
    expect(csv).toContain('"{""Nats-Msg-Id"":""abc,def""}"');
    expect(csv).toContain('""ok"": true');
  });

  it("builds text with metadata and payload", async () => {
    const text = await toText([sample, { ...sample, seq: 43, data: b64("second"), headers: undefined }]);
    expect(text).toBe(
      [
        "seq: 42",
        "subject: orders.created",
        "time: 2026-07-27T12:00:00Z",
        "format: json",
        'headers: {"Nats-Msg-Id":"abc,def"}',
        "payload:",
        '{\n  "ok": true\n}',
        "---",
        "seq: 43",
        "subject: orders.created",
        "time: 2026-07-27T12:00:00Z",
        "format: bytes",
        "payload:",
        "second",
        "",
      ].join("\n"),
    );
  });

  it("decodes MessagePack payloads in JSON export", async () => {
    const { encode } = await import("@msgpack/msgpack");
    const bytes = encode({ ok: true });
    const data = btoa(String.fromCharCode(...bytes));
    const row = rowFromMessage({
      seq: 1,
      subject: "mp",
      time: "2026-07-27T12:00:00Z",
      data,
      headers: { "Content-Type": "application/msgpack" },
    });
    const parsed = JSON.parse(await toJSON([row]));
    expect(parsed.format).toBe("msgpack");
    expect(JSON.parse(parsed.payload)).toEqual({ ok: true });
  });

  it("decodes CBOR payloads in JSON export via sniffing", async () => {
    const { encode } = await import("cbor-x");
    const bytes = new Uint8Array(encode({ ok: true }));
    const data = btoa(String.fromCharCode(...bytes));
    const row = rowFromMessage({
      seq: 2,
      subject: "cbor",
      time: "2026-07-27T12:00:00Z",
      data,
    });
    const parsed = JSON.parse(await toJSON([row]));
    expect(parsed.format).toBe("cbor");
    expect(JSON.parse(parsed.payload)).toEqual({ ok: true });
  });
});
