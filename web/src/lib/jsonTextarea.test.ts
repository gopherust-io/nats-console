import { describe, expect, it } from "vitest";
import { applyJsonTextareaKey } from "./jsonTextarea";

describe("applyJsonTextareaKey", () => {
  it("inserts two-space indent on Tab at caret", () => {
    const result = applyJsonTextareaKey('{\n"', "Tab", false, 2, 2);
    expect(result).toEqual({
      value: '{\n  "',
      selectionStart: 4,
      selectionEnd: 4,
    });
  });

  it("indents each selected line on Tab", () => {
    const value = "{\n\"a\": 1\n}";
    const result = applyJsonTextareaKey(value, "Tab", false, 2, 8);
    expect(result?.value).toBe("{\n  \"a\": 1\n}");
    expect(result?.selectionStart).toBe(4);
    expect(result?.selectionEnd).toBe(10);
  });

  it("outdents on Shift+Tab", () => {
    const value = '{\n  "a": 1\n}';
    const result = applyJsonTextareaKey(value, "Tab", true, 4, 4);
    expect(result?.value).toBe('{\n"a": 1\n}');
    expect(result?.selectionStart).toBe(2);
  });

  it("continues indent on Enter", () => {
    const value = '{\n  "a": 1';
    const result = applyJsonTextareaKey(value, "Enter", false, value.length, value.length);
    expect(result?.value).toBe('{\n  "a": 1\n  ');
    expect(result?.selectionStart).toBe(value.length + 3);
  });

  it("adds an extra indent level after an opening brace", () => {
    const value = "{";
    const result = applyJsonTextareaKey(value, "Enter", false, 1, 1);
    expect(result?.value).toBe("{\n  \n}");
    expect(result?.selectionStart).toBe(4);
  });

  it("ignores unrelated keys", () => {
    expect(applyJsonTextareaKey("{}", "a", false, 1, 1)).toBeNull();
  });
});
