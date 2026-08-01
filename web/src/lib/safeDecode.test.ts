import { describe, expect, it } from "vitest";
import { safeDecodeURIComponent } from "./safeDecode";

describe("safeDecodeURIComponent", () => {
  it("decodes a valid percent-encoded value", () => {
    expect(safeDecodeURIComponent("hello%20world")).toBe("hello world");
  });

  it("returns the input unchanged when already decoded", () => {
    expect(safeDecodeURIComponent("plain-value")).toBe("plain-value");
  });

  it("returns the input unchanged for empty strings", () => {
    expect(safeDecodeURIComponent("")).toBe("");
  });

  it("returns the input unchanged instead of throwing on malformed sequences", () => {
    expect(safeDecodeURIComponent("100%")).toBe("100%");
    expect(safeDecodeURIComponent("%E0%A4%A")).toBe("%E0%A4%A");
  });
});
