import { Fragment, memo, useMemo, type ReactNode } from "react";

type StatItem = { label: string; value: string };

type Block =
  | { type: "paragraph"; text: string }
  | { type: "heading"; text: string }
  | { type: "stats"; items: StatItem[] }
  | { type: "list"; items: string[] };

function stripMarkdown(text: string): string {
  return text.replace(/\*\*/g, "").replace(/`/g, "").trim();
}

function parseStatLine(line: string): StatItem | null {
  const cleaned = line.replace(/^[*\-•]\s+/, "").trim();
  const match = cleaned.match(/^\*{0,2}([^*:]+)\*{0,2}:\s*(.+)$/);
  if (!match) return null;
  return { label: match[1].trim(), value: match[2].trim() };
}

function isHeadingLine(line: string): boolean {
  const trimmed = line.trim();
  return /^\*\*[^*]+\*\*:?\s*$/.test(trimmed);
}

function isStatLine(line: string): boolean {
  const cleaned = line.replace(/^[*\-•]\s+/, "").trim();
  return /^\*{0,2}[^*:]+?\*{0,2}:\s*.+$/.test(cleaned);
}

function parseBlocks(content: string): Block[] {
  const blocks: Block[] = [];
  const sections = content.trim().split(/\n\s*\n/);

  for (const section of sections) {
    const lines = section
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean);
    if (lines.length === 0) continue;

    if (lines.length === 1 && (isHeadingLine(lines[0]) || (/:$/.test(lines[0]) && !isStatLine(lines[0])))) {
      blocks.push({ type: "heading", text: stripMarkdown(lines[0]).replace(/:$/, "") });
      continue;
    }

    if (lines.every(isStatLine)) {
      const stats = lines.map((line) => parseStatLine(line.replace(/^[*\-•]\s+/, "")));
      if (stats.every(Boolean)) {
        blocks.push({ type: "stats", items: stats as StatItem[] });
        continue;
      }
    }

    const listLines = lines.filter((line) => /^[*\-•]\s+/.test(line));
    if (listLines.length === lines.length) {
      const items = listLines.map((line) => line.replace(/^[*\-•]\s+/, "").trim());
      const stats = items.map(parseStatLine);
      if (stats.every(Boolean)) {
        blocks.push({ type: "stats", items: stats as StatItem[] });
        continue;
      }
      blocks.push({ type: "list", items: items.map(stripMarkdown) });
      continue;
    }

    blocks.push({ type: "paragraph", text: lines.map(stripMarkdown).join(" ") });
  }

  return blocks;
}

function renderInline(text: string): ReactNode[] {
  const parts = text.split(/(\*\*[^*]+\*\*)/g);
  return parts.map((part, index) => {
    if (part.startsWith("**") && part.endsWith("**")) {
      const label = part.slice(2, -2);
      return (
        <span key={index} className="assistant-inline-label">
          {label}
        </span>
      );
    }
    return <Fragment key={index}>{part}</Fragment>;
  });
}

function AssistantMessageBody({ content }: { content: string }) {
  const blocks = useMemo(() => parseBlocks(content), [content]);

  return (
    <div className="assistant-body">
      {blocks.map((block, index) => {
        switch (block.type) {
          case "heading":
            return (
              <div key={index} className="assistant-body__heading">
                {block.text}
              </div>
            );
          case "stats":
            return (
              <dl key={index} className="assistant-stats">
                {block.items.map((item) => (
                  <div key={item.label} className="assistant-stat">
                    <dt>{item.label}</dt>
                    <dd>{item.value}</dd>
                  </div>
                ))}
              </dl>
            );
          case "list":
            return (
              <ul key={index} className="assistant-list">
                {block.items.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            );
          default:
            return (
              <p key={index} className="assistant-body__paragraph">
                {renderInline(block.text)}
              </p>
            );
        }
      })}
    </div>
  );
}

export default memo(AssistantMessageBody);
