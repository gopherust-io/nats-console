import { useEffect, useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { MESSAGE_IMPORT_MAX_BATCH } from "../lib/constants";
import { parseMessageImportFile, type MessageImportItem } from "../lib/messageImport";

type MessageImportButtonProps = {
  disabled?: boolean;
  busy?: boolean;
  onImport: (items: MessageImportItem[]) => Promise<void>;
  className?: string;
};

export default function MessageImportButton({
  disabled = false,
  busy = false,
  onImport,
  className = "btn secondary",
}: MessageImportButtonProps) {
  const { t } = useTranslation();
  const inputId = useId();
  const inputRef = useRef<HTMLInputElement>(null);
  const [error, setError] = useState("");
  const [pending, setPending] = useState<MessageImportItem[] | null>(null);
  const [importing, setImporting] = useState(false);

  useEffect(() => {
    if (!pending) return;
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape" && !importing) setPending(null);
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [pending, importing]);

  async function onFileChange(file: File | null) {
    setError("");
    if (!file) return;
    try {
      const parsed = await parseMessageImportFile(file);
      setPending(parsed.items);
    } catch (err) {
      const code = err instanceof Error ? err.message : "";
      if (code === "invalid-json") setError(t("streams.importInvalidJson"));
      else if (code === "empty") setError(t("streams.importEmpty"));
      else if (code === "too-many") setError(t("streams.importTooMany", { max: MESSAGE_IMPORT_MAX_BATCH }));
      else if (code === "missing-subject" || code === "missing-payload" || code === "invalid-item") {
        setError(t("streams.importInvalidShape"));
      } else if (code === "invalid base64") {
        setError(t("streams.importInvalidPayload"));
      } else {
        setError(t("streams.importFailed"));
      }
    } finally {
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  async function confirmImport() {
    if (!pending || importing) return;
    setImporting(true);
    setError("");
    try {
      await onImport(pending);
      setPending(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("streams.importFailed"));
    } finally {
      setImporting(false);
    }
  }

  return (
    <div className="message-import">
      <input
        ref={inputRef}
        id={inputId}
        type="file"
        accept="application/json,.json"
        hidden
        disabled={disabled || busy || importing}
        onChange={(e) => void onFileChange(e.target.files?.[0] ?? null)}
      />
      <button
        type="button"
        className={className}
        disabled={disabled || busy || importing}
        onClick={() => inputRef.current?.click()}
      >
        {importing ? t("streams.importing") : t("streams.import")}
      </button>
      {error && (
        <p className="field-error" role="alert">
          {error}
        </p>
      )}
      {pending && (
        <div className="message-import__confirm card" role="dialog" aria-labelledby={`${inputId}-confirm`}>
          <p id={`${inputId}-confirm`}>
            {t("streams.importConfirm", { count: pending.length })}
          </p>
          <div className="actions">
            <button type="button" className="btn secondary" disabled={importing} onClick={() => setPending(null)}>
              {t("common.cancel")}
            </button>
            <button type="button" className="btn" disabled={importing} onClick={() => void confirmImport()}>
              {importing ? t("streams.importing") : t("streams.importConfirmAction")}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
