import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { decodeBase64, tryParseJSON } from "../lib/api";

const PREVIEW_LIMIT = 8192;

export type MessagePayloadViewerProps = {
  /** Base64-encoded payload (preferred when coming from the API). */
  data?: string;
  /** Already-decoded payload; used when `data` is omitted. */
  payload?: string;
  headers?: Record<string, string>;
  /** Start in raw mode (default: pretty-print JSON when possible). */
  defaultRaw?: boolean;
  /** Controlled raw mode; when set, internal toggle is hidden unless onRawChange is provided. */
  rawMode?: boolean;
  onRawChange?: (raw: boolean) => void;
  /** Compact layout for live-tail rows (smaller controls, no header list by default). */
  compact?: boolean;
  showHeaders?: boolean;
};

export default function MessagePayloadViewer({
  data,
  payload: payloadProp,
  headers,
  defaultRaw = false,
  rawMode: rawModeProp,
  onRawChange,
  compact = false,
  showHeaders = true,
}: MessagePayloadViewerProps) {
  const { t } = useTranslation();
  const [internalRaw, setInternalRaw] = useState(defaultRaw);
  const [showFull, setShowFull] = useState(false);

  const rawMode = rawModeProp ?? internalRaw;
  const isControlled = rawModeProp !== undefined;

  function setRawMode(next: boolean) {
    if (onRawChange) onRawChange(next);
    else setInternalRaw(next);
  }

  const payload = useMemo(() => {
    if (payloadProp !== undefined) return payloadProp;
    if (data !== undefined) return decodeBase64(data);
    return "";
  }, [data, payloadProp]);

  const parsed = useMemo(() => tryParseJSON(payload), [payload]);
  const truncated = !showFull && payload.length > PREVIEW_LIMIT;
  const displaySource = truncated ? `${payload.slice(0, PREVIEW_LIMIT)}\n…` : payload;
  const display =
    !rawMode && parsed.isJSON && !truncated
      ? JSON.stringify(parsed.parsed, null, 2)
      : displaySource;

  const headerEntries = headers ? Object.entries(headers) : [];
  const showRawToggle = (parsed.isJSON || rawMode) && (!isControlled || Boolean(onRawChange));

  return (
    <div className={`message-payload${compact ? " message-payload--compact" : ""}`}>
      {!compact && (
        <div className="message-payload__toolbar">
          <span className="message-payload__title">{t("streams.payload")}</span>
          <div className="message-payload__actions">
            {showRawToggle && (
              <button
                type="button"
                className="btn btn--secondary btn--small"
                aria-pressed={rawMode}
                onClick={() => setRawMode(!rawMode)}
              >
                {rawMode ? t("streams.showJson") : t("streams.showRaw")}
              </button>
            )}
            {truncated && (
              <button type="button" className="btn btn--secondary btn--small" onClick={() => setShowFull(true)}>
                {t("streams.showFullPayload")}
              </button>
            )}
          </div>
        </div>
      )}

      {compact && truncated && (
        <div className="message-payload__toolbar">
          <button type="button" className="btn btn--secondary btn--small" onClick={() => setShowFull(true)}>
            {t("streams.showFullPayload")}
          </button>
        </div>
      )}

      {showHeaders && headerEntries.length > 0 && (
        <dl className="message-headers">
          {headerEntries.map(([key, value]) => (
            <div key={key} className="message-headers__row">
              <dt className="mono">{key}</dt>
              <dd className="mono">{value}</dd>
            </div>
          ))}
        </dl>
      )}

      <pre className="mono message-payload__body">{display}</pre>
    </div>
  );
}
