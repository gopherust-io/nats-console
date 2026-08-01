import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  compactPayloadPreview,
  decodeMessagePayload,
  payloadFormatLabel,
  type DecodedMessagePayload,
} from "../lib/messagePayloadDecode";

const PREVIEW_LIMIT = 8192;

export type MessagePayloadViewerProps = {
  /** Base64-encoded payload (preferred when coming from the API). */
  data?: string;
  /** Already-decoded payload text; used when `data` is omitted (no format sniffing). */
  payload?: string;
  headers?: Record<string, string>;
  /** Compact layout for live-tail rows (smaller controls, no header list by default). */
  compact?: boolean;
  showHeaders?: boolean;
  /** Optional host object to cache a full decode (e.g. live message row). */
  cacheHost?: object;
};

export default function MessagePayloadViewer({
  data,
  payload: payloadProp,
  headers,
  compact = false,
  showHeaders = true,
  cacheHost,
}: MessagePayloadViewerProps) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(!compact);
  const [showFull, setShowFull] = useState(false);
  const [decoded, setDecoded] = useState<DecodedMessagePayload | null>(null);
  const [decoding, setDecoding] = useState(false);

  useEffect(() => {
    if (payloadProp !== undefined || data === undefined || !expanded) {
      return;
    }
    let cancelled = false;
    setDecoding(true);
    void decodeMessagePayload(data, headers, cacheHost).then((result) => {
      if (!cancelled) {
        setDecoded(result);
        setDecoding(false);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [expanded, data, headers, payloadProp, cacheHost]);

  const preview = payloadProp ?? (data !== undefined ? compactPayloadPreview(data) : "");
  const formatted = payloadProp !== undefined ? payloadProp : (decoded?.text ?? preview);
  const truncated = expanded && !showFull && formatted.length > PREVIEW_LIMIT;
  const display = truncated ? `${formatted.slice(0, PREVIEW_LIMIT)}\n…` : formatted;
  const formatBadge =
    payloadProp === undefined && decoded ? payloadFormatLabel(decoded.format) : null;

  const headerEntries = headers ? Object.entries(headers) : [];

  return (
    <div className={`message-payload${compact ? " message-payload--compact" : ""}`}>
      {!compact && (
        <div className="message-payload__toolbar">
          <span className="message-payload__title">
            {t("streams.payload")}
            {formatBadge && (
              <span className="message-payload__format" title={t("streams.detectedFormat")}>
                {formatBadge}
              </span>
            )}
          </span>
          <div className="message-payload__actions">
            {truncated && (
              <button type="button" className="btn btn--secondary btn--small" onClick={() => setShowFull(true)}>
                {t("streams.showFullPayload")}
              </button>
            )}
          </div>
        </div>
      )}

      {compact && (
        <div className="message-payload__toolbar">
          {formatBadge && (
            <span className="message-payload__format" title={t("streams.detectedFormat")}>
              {formatBadge}
            </span>
          )}
          {!expanded ? (
            <button
              type="button"
              className="btn btn--secondary btn--small"
              onClick={() => setExpanded(true)}
            >
              {t("streams.showFullPayload")}
            </button>
          ) : truncated ? (
            <button type="button" className="btn btn--secondary btn--small" onClick={() => setShowFull(true)}>
              {t("streams.showFullPayload")}
            </button>
          ) : null}
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

      <pre className="mono message-payload__body">
        {decoding && expanded && !decoded ? "…" : display}
      </pre>
    </div>
  );
}
