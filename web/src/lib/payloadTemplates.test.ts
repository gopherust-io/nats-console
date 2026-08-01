import { describe, expect, it, beforeEach } from "vitest";
import { STORAGE_KEYS } from "./constants";
import {
  deletePayloadTemplate,
  readPayloadTemplates,
  savePayloadTemplate,
  templatesForStream,
} from "./payloadTemplates";

describe("payloadTemplates", () => {
  beforeEach(() => {
    localStorage.removeItem(STORAGE_KEYS.payloadTemplates);
  });

  it("saves and filters templates by stream", () => {
    savePayloadTemplate({
      name: "ping",
      format: "json",
      payload: '{"ok":true}',
      clusterId: "c1",
      stream: "ORDERS",
    });
    savePayloadTemplate({
      name: "global",
      format: "json",
      payload: "{}",
    });

    const forOrders = templatesForStream("c1", "ORDERS");
    expect(forOrders.map((t) => t.name).sort()).toEqual(["global", "ping"]);
    expect(templatesForStream("c1", "OTHER").map((t) => t.name)).toEqual(["global"]);
  });

  it("deletes templates by id", () => {
    const [saved] = savePayloadTemplate({
      name: "tmp",
      format: "json",
      payload: "{}",
    });
    expect(readPayloadTemplates()).toHaveLength(1);
    deletePayloadTemplate(saved!.id);
    expect(readPayloadTemplates()).toHaveLength(0);
  });
});
