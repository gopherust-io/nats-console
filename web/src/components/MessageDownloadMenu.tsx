import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  downloadMessages,
  liveBufferFilename,
  MESSAGE_DOWNLOAD_FORMATS,
  type MessageDownloadFormat,
  type MessageExportRow,
  singleMessageFilename,
} from "../lib/messageDownload";
import {
  decodeMessagePayload,
  detectPayloadFormatFromHeaders,
  type PayloadWireFormat,
} from "../lib/messagePayloadDecode";

type MessageDownloadMenuProps = {
  /** Prebuilt rows (e.g. single-message export). Prefer getRows for live buffers. */
  rows?: MessageExportRow[];
  /** Lazily build rows on download so hot paths avoid remapping every flush. */
  getRows?: () => MessageExportRow[];
  disabled?: boolean;
  /** When set, filenames use `{stream}-seq-{seq}.{ext}`; otherwise live buffer naming. */
  stream: string;
  mode: "single" | "live";
  className?: string;
  buttonClassName?: string;
};

export default function MessageDownloadMenu({
  rows,
  getRows,
  disabled = false,
  stream,
  mode,
  className = "nc-dropdown",
  buttonClassName = "btn secondary",
}: MessageDownloadMenuProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [wireFormat, setWireFormat] = useState<PayloadWireFormat | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointer = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  function resolveRows(): MessageExportRow[] {
    if (getRows) return getRows();
    return rows ?? [];
  }

  useEffect(() => {
    if (!open) {
      setWireFormat(null);
      return;
    }
    const exportRows = resolveRows();
    const row = exportRows[0];
    if (!row) {
      setWireFormat(null);
      return;
    }
    const fromHeader = detectPayloadFormatFromHeaders(row.headers);
    if (fromHeader) {
      setWireFormat(fromHeader);
      return;
    }
    let cancelled = false;
    void decodeMessagePayload(row.data, row.headers).then((decoded) => {
      if (!cancelled) setWireFormat(decoded.format);
    });
    return () => {
      cancelled = true;
    };
    // resolveRows depends on open-time rows/getRows; intentional on menu open.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only when menu opens
  }, [open, rows, getRows]);

  function formatEnabled(format: MessageDownloadFormat): boolean {
    if (format !== "msgpack" && format !== "cbor" && format !== "protobuf") return true;
    if (!wireFormat) return false;
    return format === wireFormat;
  }

  async function onDownload(format: MessageDownloadFormat, ext: string) {
    if (busy) return;
    const exportRows = resolveRows();
    if (exportRows.length === 0) return;
    if (!formatEnabled(format)) return;
    setBusy(true);
    try {
      const filename =
        mode === "single"
          ? singleMessageFilename(stream, exportRows[0]?.seq ?? 0, ext)
          : liveBufferFilename(stream, ext);
      await downloadMessages(exportRows, format, filename, `${stream} messages`);
      setOpen(false);
    } finally {
      setBusy(false);
    }
  }

  const isDisabled =
    disabled || busy || (!getRows && (rows?.length ?? 0) === 0);

  return (
    <div className={className} ref={rootRef}>
      <button
        type="button"
        className={buttonClassName}
        aria-expanded={open}
        aria-haspopup="menu"
        disabled={isDisabled}
        onClick={() => setOpen((value) => !value)}
      >
        {busy ? t("streams.downloading") : t("streams.download")}
      </button>
      {open && (
        <div className="nc-dropdown__menu" data-state="open" role="menu">
          {MESSAGE_DOWNLOAD_FORMATS.map((item) => {
            const enabled = formatEnabled(item.format);
            return (
              <button
                key={item.format}
                type="button"
                role="menuitem"
                disabled={busy || !enabled}
                onClick={() => void onDownload(item.format, item.ext)}
              >
                {t(item.labelKey)}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
