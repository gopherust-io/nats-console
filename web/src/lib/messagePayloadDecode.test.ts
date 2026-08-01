import { encode as encodeMsgpack } from "@msgpack/msgpack";
import { encode as encodeCbor } from "cbor-x";
import { describe, expect, it } from "vitest";
import { bytesToBase64 } from "./publishEncode";
import {
  bytesToHexDump,
  compactPayloadPreview,
  decodeMessagePayload,
  detectPayloadFormat,
  detectPayloadFormatFromBytes,
  parseProtobufWire,
  payloadFormatLabel,
} from "./messagePayloadDecode";

describe("messagePayloadDecode", () => {
  it("detects content types from headers", async () => {
    expect(await detectPayloadFormat({ "Content-Type": "application/msgpack" })).toBe("msgpack");
    expect(await detectPayloadFormat({ "Content-Type": "application/cbor" })).toBe("cbor");
    expect(await detectPayloadFormat({ "Content-Type": "application/x-protobuf" })).toBe("protobuf");
    expect(await detectPayloadFormat({ "Content-Type": "application/json" })).toBe("json");
    expect(await detectPayloadFormat({})).toBe("bytes");
  });

  it("sniffs JSON without headers", async () => {
    const bytes = new TextEncoder().encode('{"hello":"world"}');
    expect(await detectPayloadFormatFromBytes(bytes)).toBe("json");
    expect(await detectPayloadFormat(undefined, bytes)).toBe("json");
  });

  it("sniffs MessagePack without headers", async () => {
    const bytes = encodeMsgpack({ hello: "world", n: 1 });
    expect(await detectPayloadFormatFromBytes(bytes)).toBe("msgpack");
  });

  it("sniffs CBOR without headers", async () => {
    const bytes = new Uint8Array(encodeCbor({ hello: "world", n: 1 }));
    expect(await detectPayloadFormatFromBytes(bytes)).toBe("cbor");
  });

  it("sniffs Protobuf wire without headers", async () => {
    // field 1 varint = 1
    const bytes = new Uint8Array([0x08, 0x01]);
    expect(await detectPayloadFormatFromBytes(bytes)).toBe("protobuf");
  });

  it("does not claim plain ASCII text as msgpack/cbor/protobuf", async () => {
    const bytes = new TextEncoder().encode("hello world");
    expect(await detectPayloadFormatFromBytes(bytes)).toBe("bytes");
  });

  it("headers win over byte sniffing", async () => {
    const jsonBytes = new TextEncoder().encode('{"a":1}');
    expect(await detectPayloadFormat({ "Content-Type": "application/msgpack" }, jsonBytes)).toBe(
      "msgpack",
    );
  });

  it("decodes MessagePack to pretty JSON", async () => {
    const bytes = encodeMsgpack({ hello: "world", n: 1 });
    const decoded = await decodeMessagePayload(bytesToBase64(bytes), {
      "Content-Type": "application/msgpack",
    });
    expect(decoded.format).toBe("msgpack");
    expect(decoded.nativeExt).toBe("msgpack");
    expect(JSON.parse(decoded.text)).toEqual({ hello: "world", n: 1 });
  });

  it("decodes CBOR to pretty JSON", async () => {
    const bytes = new Uint8Array(encodeCbor({ hello: "world", n: 1 }));
    const decoded = await decodeMessagePayload(bytesToBase64(bytes), {
      "Content-Type": "application/cbor",
    });
    expect(decoded.format).toBe("cbor");
    expect(decoded.nativeExt).toBe("cbor");
    expect(JSON.parse(decoded.text)).toEqual({ hello: "world", n: 1 });
  });

  it("renders Protobuf as wire listing when valid", async () => {
    const bytes = new Uint8Array([0x08, 0x01, 0x12, 0x03, 0x61, 0x62, 0x63]);
    const decoded = await decodeMessagePayload(bytesToBase64(bytes), {
      "Content-Type": "application/protobuf",
    });
    expect(decoded.format).toBe("protobuf");
    expect(decoded.nativeExt).toBe("pb");
    expect(decoded.text).toContain("1: varint = 1");
    expect(decoded.text).toContain('2: len(3) = "abc"');
    expect(decoded.bytes).toEqual(bytes);
  });

  it("falls back to hex for invalid protobuf declared by header", async () => {
    const bytes = new Uint8Array([0xff, 0xff, 0xff]);
    const decoded = await decodeMessagePayload(bytesToBase64(bytes), {
      "Content-Type": "application/protobuf",
    });
    expect(decoded.format).toBe("protobuf");
    expect(decoded.text).toBe(bytesToHexDump(bytes));
  });

  it("parseProtobufWire rejects empty and text-like buffers", () => {
    expect(parseProtobufWire(new Uint8Array())).toBeNull();
    expect(parseProtobufWire(new TextEncoder().encode("not protobuf"))).toBeNull();
  });

  it("labels formats for the UI badge", () => {
    expect(payloadFormatLabel("json")).toBe("JSON");
    expect(payloadFormatLabel("msgpack")).toBe("MessagePack");
    expect(payloadFormatLabel("cbor")).toBe("CBOR");
    expect(payloadFormatLabel("protobuf")).toBe("Protobuf");
    expect(payloadFormatLabel("bytes")).toBe("Binary");
  });

  it("compact preview skips binary sniffing", () => {
    const bytes = encodeMsgpack({ hello: "world", n: 1 });
    const preview = compactPayloadPreview(bytesToBase64(bytes));
    expect(preview.length).toBeGreaterThan(0);
    expect(preview).not.toContain("hello");
  });
});
