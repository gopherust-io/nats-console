import { describe, expect, it } from "vitest";
import { formatJsonError, locateJsonError } from "./jsonError";

describe("locateJsonError", () => {
  it("returns null for valid JSON", () => {
    expect(locateJsonError('{"ok":true}')).toBeNull();
  });

  it("points at a trailing comma with line and column", () => {
    const text = '{\n  "a": 1,\n}';
    const loc = locateJsonError(text);
    expect(loc).not.toBeNull();
    expect(loc!.line).toBeGreaterThanOrEqual(2);
    expect(loc!.snippet.length).toBeGreaterThan(0);
    expect(loc!.caret).toContain("^");
    expect(formatJsonError(loc!)).toMatch(/line \d+, column \d+/);
  });

  it("points near missing quotes", () => {
    const text = '{\n  hello: "world"\n}';
    const loc = locateJsonError(text);
    expect(loc).not.toBeNull();
    expect(loc!.line).toBe(2);
    expect(formatJsonError(loc!)).toContain("^");
  });
});
