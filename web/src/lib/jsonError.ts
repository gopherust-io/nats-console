export type JsonParseLocation = {
  line: number;
  column: number;
  position: number;
  snippet: string;
  caret: string;
  message: string;
};

/** Parse JSON and, on failure, locate the error with line/column and a caret snippet. */
export function locateJsonError(text: string): JsonParseLocation | null {
  try {
    JSON.parse(text);
    return null;
  } catch (err) {
    const message = err instanceof Error ? err.message : "Invalid JSON";
    const position = extractJsonErrorPosition(message, text);
    const { line, column } = offsetToLineColumn(text, position);
    const { snippet, caret } = buildErrorSnippet(text, position, column);
    return { line, column, position, snippet, caret, message: cleanJsonErrorMessage(message) };
  }
}

export function formatJsonError(location: JsonParseLocation): string {
  return `line ${location.line}, column ${location.column}: ${location.message}\n${location.snippet}\n${location.caret}`;
}

function extractJsonErrorPosition(message: string, text: string): number {
  const atPosition = /(?:at position|at offset)\s+(\d+)/i.exec(message);
  if (atPosition) {
    return clamp(Number(atPosition[1]), 0, text.length);
  }

  const lineCol = /line\s+(\d+)\s+column\s+(\d+)/i.exec(message);
  if (lineCol) {
    return lineColumnToOffset(text, Number(lineCol[1]), Number(lineCol[2]));
  }

  // Fallback: scan for first likely break (unbalanced / trailing junk heuristics are weak;
  // point at end of text so the caret is still useful).
  return Math.max(0, text.trimEnd().length - 1);
}

function offsetToLineColumn(text: string, position: number): { line: number; column: number } {
  const safe = clamp(position, 0, text.length);
  let line = 1;
  let column = 1;
  for (let i = 0; i < safe; i += 1) {
    if (text[i] === "\n") {
      line += 1;
      column = 1;
    } else {
      column += 1;
    }
  }
  return { line, column };
}

function lineColumnToOffset(text: string, line: number, column: number): number {
  let currentLine = 1;
  let i = 0;
  while (i < text.length && currentLine < line) {
    if (text[i] === "\n") currentLine += 1;
    i += 1;
  }
  return clamp(i + Math.max(column, 1) - 1, 0, text.length);
}

function buildErrorSnippet(
  text: string,
  position: number,
  column: number,
): { snippet: string; caret: string } {
  const safe = clamp(position, 0, Math.max(0, text.length));
  const lineStart = text.lastIndexOf("\n", safe - 1) + 1;
  const lineEndIdx = text.indexOf("\n", safe);
  const lineEnd = lineEndIdx === -1 ? text.length : lineEndIdx;
  let snippet = text.slice(lineStart, lineEnd);
  let caretCol = column;

  // Keep the snippet readable for long lines.
  const max = 72;
  if (snippet.length > max) {
    const start = Math.max(0, caretCol - 24);
    const end = Math.min(snippet.length, start + max);
    snippet = `${start > 0 ? "…" : ""}${snippet.slice(start, end)}${end < text.slice(lineStart, lineEnd).length ? "…" : ""}`;
    caretCol = caretCol - start + (start > 0 ? 1 : 0);
  }

  const caret = `${" ".repeat(Math.max(0, caretCol - 1))}^`;
  return { snippet: snippet || " ", caret };
}

function cleanJsonErrorMessage(message: string): string {
  return message
    .replace(/\s+in JSON at position \d+/i, "")
    .replace(/\s+at line \d+ column \d+ of the JSON data/i, "")
    .replace(/^JSON\.parse:\s*/i, "")
    .trim();
}

function clamp(n: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, n));
}
