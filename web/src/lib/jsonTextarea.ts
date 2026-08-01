const INDENT = "  ";

export type TextareaEdit = {
  value: string;
  selectionStart: number;
  selectionEnd: number;
};

/** Apply Tab / Shift+Tab / Enter editing helpers for JSON construction in a textarea. */
export function applyJsonTextareaKey(
  value: string,
  key: string,
  shiftKey: boolean,
  selectionStart: number,
  selectionEnd: number,
): TextareaEdit | null {
  if (key === "Tab") {
    return shiftKey
      ? outdentSelection(value, selectionStart, selectionEnd)
      : indentSelection(value, selectionStart, selectionEnd);
  }
  if (key === "Enter" && !shiftKey) {
    return insertNewlineWithIndent(value, selectionStart, selectionEnd);
  }
  return null;
}

function indentSelection(value: string, start: number, end: number): TextareaEdit {
  if (start === end) {
    const next = value.slice(0, start) + INDENT + value.slice(end);
    const cursor = start + INDENT.length;
    return { value: next, selectionStart: cursor, selectionEnd: cursor };
  }

  const lineStart = value.lastIndexOf("\n", start - 1) + 1;
  const block = value.slice(lineStart, end);
  const indented = block
    .split("\n")
    .map((line) => INDENT + line)
    .join("\n");
  const next = value.slice(0, lineStart) + indented + value.slice(end);
  return {
    value: next,
    selectionStart: start + INDENT.length,
    selectionEnd: end + (indented.length - block.length),
  };
}

function outdentSelection(value: string, start: number, end: number): TextareaEdit {
  const lineStart = value.lastIndexOf("\n", start - 1) + 1;
  const rangeEnd = start === end ? findLineEnd(value, end) : end;
  const block = value.slice(lineStart, rangeEnd);
  const lines = block.split("\n");
  let removedBeforeCursor = 0;
  let removedTotal = 0;

  const outdented = lines
    .map((line, index) => {
      const strip = line.startsWith(INDENT) ? INDENT.length : line.startsWith("\t") ? 1 : line.startsWith(" ") ? 1 : 0;
      if (strip === 0) return line;
      removedTotal += strip;
      if (index === 0) removedBeforeCursor = strip;
      return line.slice(strip);
    })
    .join("\n");

  const next = value.slice(0, lineStart) + outdented + value.slice(rangeEnd);
  const nextStart = Math.max(lineStart, start - removedBeforeCursor);
  const nextEnd = start === end ? nextStart : Math.max(nextStart, end - removedTotal);
  return { value: next, selectionStart: nextStart, selectionEnd: nextEnd };
}

function insertNewlineWithIndent(value: string, start: number, end: number): TextareaEdit {
  const lineStart = value.lastIndexOf("\n", start - 1) + 1;
  const currentLine = value.slice(lineStart, start);
  const indentMatch = /^[\t ]*/.exec(currentLine);
  let indent = indentMatch?.[0] ?? "";

  const trimmedLeft = currentLine.trimEnd();
  const opened = trimmedLeft.endsWith("{") || trimmedLeft.endsWith("[");
  if (opened) indent += INDENT;

  const before = value.slice(0, start);
  const after = value.slice(end);
  const prev = trimmedLeft.slice(-1);
  const next = after.trimStart().slice(0, 1);
  const pair = prev === "{" ? "}" : prev === "[" ? "]" : "";

  let insert = `\n${indent}`;
  let cursor = start + insert.length;

  if (pair && (next === pair || next === "")) {
    const closerIndent = indent.slice(0, Math.max(0, indent.length - INDENT.length));
    if (next === pair) {
      // Keep existing closer on the following line.
      insert = `\n${indent}\n${closerIndent}`;
      cursor = start + 1 + indent.length;
    } else {
      // Auto-close empty object/array.
      insert = `\n${indent}\n${closerIndent}${pair}`;
      cursor = start + 1 + indent.length;
    }
  }

  return {
    value: before + insert + after,
    selectionStart: cursor,
    selectionEnd: cursor,
  };
}

function findLineEnd(value: string, from: number): number {
  const idx = value.indexOf("\n", from);
  return idx === -1 ? value.length : idx;
}
