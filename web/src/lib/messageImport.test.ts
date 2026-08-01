import { describe, expect, it } from "vitest";
import { toJSON, type MessageExportRow } from "./messageDownload";
import { parseMessageImportJSON } from "./messageImport";

describe("messageImport", () => {
  it("round-trips JSON export from toJSON", async () => {
    const rows: MessageExportRow[] = [
      {
        seq: 1,
        subject: "orders.created",
        time: "2026-01-01T00:00:00Z",
        data: btoa(JSON.stringify({ id: 1 })),
        headers: { "Content-Type": "application/json" },
      },
    ];
    const exported = await toJSON(rows);
    const parsed = parseMessageImportJSON(exported);
    expect(parsed.items).toHaveLength(1);
    expect(parsed.items[0]?.subject).toBe("orders.created");
    expect(parsed.items[0]?.data.length).toBeGreaterThan(0);
    expect(parsed.items[0]?.headers?.["Content-Type"]).toBe("application/json");
  });

  it("accepts an array of subject+payload objects", () => {
    const parsed = parseMessageImportJSON(
      JSON.stringify([
        { subject: "a", payload: '{"x":1}' },
        { subject: "b", payload: '{"y":2}' },
      ]),
    );
    expect(parsed.items.map((i) => i.subject)).toEqual(["a", "b"]);
  });

  it("rejects oversized batches", () => {
    const items = Array.from({ length: 101 }, (_, i) => ({
      subject: `s.${i}`,
      payload: "{}",
    }));
    expect(() => parseMessageImportJSON(JSON.stringify(items))).toThrow("too-many");
  });
});
