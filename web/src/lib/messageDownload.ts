import { decodeMessagePayload } from "./messagePayloadDecode";

export type MessageExportRow = {
  seq: number;
  subject: string;
  time: string;
  data: string;
  headers?: Record<string, string>;
};

export type MessageDownloadFormat = "json" | "csv" | "xlsx" | "pdf" | "txt" | "msgpack" | "cbor" | "protobuf";

const CSV_COLUMNS = ["seq", "subject", "time", "headers", "payload"] as const;

export function sanitizeFilenamePart(value: string): string {
  return value.replace(/[^\w.-]+/g, "_").replace(/^_+|_+$/g, "") || "message";
}

export function singleMessageFilename(stream: string, seq: number, ext: string): string {
  return `${sanitizeFilenamePart(stream)}-seq-${seq}.${ext}`;
}

export function liveBufferFilename(stream: string, ext: string, now = new Date()): string {
  const stamp = now.toISOString().replace(/[:.]/g, "-");
  return `${sanitizeFilenamePart(stream)}-live-${stamp}.${ext}`;
}

export function rowFromMessage(input: {
  seq: number;
  subject: string;
  time: string;
  data?: string;
  headers?: Record<string, string>;
}): MessageExportRow {
  return {
    seq: input.seq,
    subject: input.subject,
    time: input.time,
    data: input.data ?? "",
    headers: input.headers,
  };
}

async function decodedPayload(row: MessageExportRow): Promise<string> {
  return (await decodeMessagePayload(row.data, row.headers)).text;
}

function headersJSON(row: MessageExportRow): string {
  if (!row.headers || Object.keys(row.headers).length === 0) return "";
  return JSON.stringify(row.headers);
}

export function escapeCSVField(value: string): string {
  if (/[",\n\r]/.test(value)) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}

export async function toJSON(rows: MessageExportRow[]): Promise<string> {
  const body = await Promise.all(
    rows.map(async (row) => {
      const decoded = await decodeMessagePayload(row.data, row.headers);
      return {
        seq: row.seq,
        subject: row.subject,
        time: row.time,
        headers: row.headers ?? {},
        format: decoded.format,
        payload: decoded.text,
      };
    }),
  );
  return `${JSON.stringify(rows.length === 1 ? body[0] : body, null, 2)}\n`;
}

export async function toCSV(rows: MessageExportRow[]): Promise<string> {
  const lines = [CSV_COLUMNS.join(",")];
  for (const row of rows) {
    lines.push(
      [
        String(row.seq),
        escapeCSVField(row.subject),
        escapeCSVField(row.time),
        escapeCSVField(headersJSON(row)),
        escapeCSVField(await decodedPayload(row)),
      ].join(","),
    );
  }
  return `${lines.join("\n")}\n`;
}

export async function toText(rows: MessageExportRow[]): Promise<string> {
  const blocks = await Promise.all(
    rows.map(async (row) => {
      const decoded = await decodeMessagePayload(row.data, row.headers);
      const lines = [
        `seq: ${row.seq}`,
        `subject: ${row.subject}`,
        `time: ${row.time}`,
        `format: ${decoded.format}`,
      ];
      const headers = headersJSON(row);
      if (headers) lines.push(`headers: ${headers}`);
      lines.push("payload:", decoded.text || "(empty)");
      return lines.join("\n");
    }),
  );
  return `${blocks.join("\n---\n")}\n`;
}

export async function toXlsx(rows: MessageExportRow[]): Promise<Blob> {
  const { default: ExcelJS } = await import("exceljs");
  const workbook = new ExcelJS.Workbook();
  const sheet = workbook.addWorksheet("Messages");
  sheet.columns = CSV_COLUMNS.map((key) => ({ header: key, key, width: key === "payload" ? 60 : 20 }));
  for (const row of rows) {
    sheet.addRow({
      seq: row.seq,
      subject: row.subject,
      time: row.time,
      headers: headersJSON(row),
      payload: await decodedPayload(row),
    });
  }
  const buffer = await workbook.xlsx.writeBuffer();
  return new Blob([buffer as BlobPart], {
    type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  });
}

function wrapPdfLines(
  doc: { splitTextToSize: (text: string, maxWidth: number) => string[] | string },
  text: string,
  maxWidth: number,
): string[] {
  const lines: string[] = [];
  for (const paragraph of text.split(/\r?\n/)) {
    if (!paragraph) {
      lines.push("");
      continue;
    }
    const wrapped = doc.splitTextToSize(paragraph, maxWidth);
    if (Array.isArray(wrapped)) lines.push(...wrapped);
    else lines.push(String(wrapped));
  }
  return lines;
}

export async function toPdf(rows: MessageExportRow[], title = "Messages"): Promise<Blob> {
  const { jsPDF } = await import("jspdf");
  const doc = new jsPDF({ unit: "pt", format: "a4" });
  const margin = 40;
  const pageWidth = doc.internal.pageSize.getWidth();
  const pageHeight = doc.internal.pageSize.getHeight();
  const maxWidth = pageWidth - margin * 2;
  let y = margin;

  const ensureSpace = (needed: number) => {
    if (y + needed > pageHeight - margin) {
      doc.addPage();
      y = margin;
    }
  };

  doc.setFont("helvetica", "bold");
  doc.setFontSize(14);
  doc.text(title, margin, y);
  y += 22;

  for (let i = 0; i < rows.length; i += 1) {
    const row = rows[i];
    const decoded = await decodeMessagePayload(row.data, row.headers);
    ensureSpace(80);
    doc.setFont("helvetica", "bold");
    doc.setFontSize(11);
    doc.text(`Message #${row.seq}`, margin, y);
    y += 16;

    doc.setFont("helvetica", "normal");
    doc.setFontSize(10);
    const meta = [
      `Subject: ${row.subject}`,
      `Time: ${row.time}`,
      `Format: ${decoded.format}`,
      headersJSON(row) ? `Headers: ${headersJSON(row)}` : "",
    ].filter(Boolean);
    for (const line of meta) {
      ensureSpace(14);
      doc.text(line, margin, y);
      y += 14;
    }

    y += 4;
    doc.setFont("courier", "normal");
    doc.setFontSize(9);
    const payloadLines = wrapPdfLines(doc, decoded.text || "(empty)", maxWidth);
    for (const line of payloadLines) {
      ensureSpace(12);
      doc.text(line, margin, y);
      y += 12;
    }

    if (i < rows.length - 1) {
      y += 10;
      ensureSpace(10);
      doc.setDrawColor(180);
      doc.line(margin, y, pageWidth - margin, y);
      y += 16;
    }
  }

  return doc.output("blob");
}

export function downloadBlob(filename: string, blob: Blob): void {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.rel = "noopener";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1_000);
}

async function nativeBinaryForRows(
  rows: MessageExportRow[],
  want: "msgpack" | "cbor" | "protobuf",
): Promise<{
  bytes: Uint8Array;
  ext: string;
  mimeType: string;
} | null> {
  const row = rows[0];
  if (!row) return null;
  const decoded = await decodeMessagePayload(row.data, row.headers);
  if (decoded.format !== want) return null;
  return { bytes: decoded.bytes, ext: decoded.nativeExt, mimeType: decoded.mimeType };
}

export async function downloadMessages(
  rows: MessageExportRow[],
  format: MessageDownloadFormat,
  filename: string,
  pdfTitle?: string,
): Promise<void> {
  if (rows.length === 0) return;

  switch (format) {
    case "json":
      downloadBlob(filename, new Blob([await toJSON(rows)], { type: "application/json;charset=utf-8" }));
      return;
    case "csv":
      downloadBlob(filename, new Blob([await toCSV(rows)], { type: "text/csv;charset=utf-8" }));
      return;
    case "txt":
      downloadBlob(filename, new Blob([await toText(rows)], { type: "text/plain;charset=utf-8" }));
      return;
    case "xlsx":
      downloadBlob(filename, await toXlsx(rows));
      return;
    case "pdf":
      downloadBlob(filename, await toPdf(rows, pdfTitle ?? "Messages"));
      return;
    case "msgpack":
    case "cbor":
    case "protobuf": {
      const native = await nativeBinaryForRows(rows, format);
      if (!native) return;
      const name = filename.endsWith(`.${native.ext}`)
        ? filename
        : `${filename.replace(/\.[^.]+$/, "")}.${native.ext}`;
      downloadBlob(name, new Blob([Uint8Array.from(native.bytes)], { type: native.mimeType }));
      return;
    }
    default:
      return;
  }
}

export const MESSAGE_DOWNLOAD_FORMATS: Array<{
  format: MessageDownloadFormat;
  ext: string;
  labelKey: string;
}> = [
  { format: "json", ext: "json", labelKey: "streams.downloadJson" },
  { format: "csv", ext: "csv", labelKey: "streams.downloadCsv" },
  { format: "xlsx", ext: "xlsx", labelKey: "streams.downloadExcel" },
  { format: "pdf", ext: "pdf", labelKey: "streams.downloadPdf" },
  { format: "txt", ext: "txt", labelKey: "streams.downloadText" },
  { format: "msgpack", ext: "msgpack", labelKey: "streams.downloadMsgpack" },
  { format: "cbor", ext: "cbor", labelKey: "streams.downloadCbor" },
  { format: "protobuf", ext: "pb", labelKey: "streams.downloadProtobuf" },
];
