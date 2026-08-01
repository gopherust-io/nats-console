import { encode as encodeMsgpack } from "@msgpack/msgpack";
import { describe, expect, it } from "vitest";
import {
  bytesToBase64,
  contentTypeFor,
  decodeBase64Bytes,
  encodeJsonPayload,
  encodePublishPayload,
} from "./publishEncode";

describe("publishEncode", () => {
  it("maps content types", () => {
    expect(contentTypeFor("json")).toBe("application/json");
    expect(contentTypeFor("msgpack")).toBe("application/msgpack");
    expect(contentTypeFor("cbor")).toBe("application/cbor");
    expect(contentTypeFor("protobuf")).toBe("application/protobuf");
  });

  it("encodes JSON as UTF-8 bytes", () => {
    const bytes = encodeJsonPayload({ hello: "world" });
    expect(new TextDecoder().decode(bytes)).toBe('{"hello":"world"}');
  });

  it("decodes strict base64 including URL-safe", () => {
    const bytes = new Uint8Array([0x0a, 0xff, 0x01]);
    const standard = bytesToBase64(bytes);
    expect(decodeBase64Bytes(standard)).toEqual(bytes);
    expect(decodeBase64Bytes(standard.replace(/\+/g, "-").replace(/\//g, "_"))).toEqual(bytes);
    expect(decodeBase64Bytes(` ${standard} \n`)).toEqual(bytes);
  });

  it("rejects invalid base64 syntax", () => {
    expect(() => decodeBase64Bytes("")).toThrow(/empty/);
    expect(() => decodeBase64Bytes("!!!")).toThrow(/invalid/);
    expect(() => decodeBase64Bytes("a")).toThrow(/invalid/); // length % 4 === 1
    expect(() => decodeBase64Bytes("ab=c")).toThrow(/invalid/); // = not only at end
    expect(() => decodeBase64Bytes("====")).toThrow(/invalid/);
    expect(() => decodeBase64Bytes("YWJj====")).toThrow(/invalid/); // more than 2 padding chars
    expect(() => decodeBase64Bytes("YW=j")).toThrow(/invalid/); // = in the middle
  });

  it("builds a publishable JSON envelope", () => {
    const result = encodePublishPayload({
      format: "json",
      jsonText: '{\n  "ok": true\n}',
    });
    expect(result.contentType).toBe("application/json");
    expect(atob(result.data)).toBe('{"ok":true}');
  });

  it("builds MessagePack, CBOR, and Protobuf envelopes from base64 binary", () => {
    const raw = encodeMsgpack({ ok: true });
    const b64 = bytesToBase64(raw);

    const msgpack = encodePublishPayload({ format: "msgpack", binaryText: b64 });
    expect(msgpack.contentType).toBe("application/msgpack");
    expect(msgpack.data).toBe(b64);

    const cbor = encodePublishPayload({ format: "cbor", binaryText: b64 });
    expect(cbor.contentType).toBe("application/cbor");
    expect(cbor.data).toBe(b64);

    const protoBytes = new Uint8Array([0x08, 0x01]);
    const proto = encodePublishPayload({
      format: "protobuf",
      binaryText: bytesToBase64(protoBytes),
    });
    expect(proto.contentType).toBe("application/protobuf");
    expect(proto.data).toBe(bytesToBase64(protoBytes));
  });
});
