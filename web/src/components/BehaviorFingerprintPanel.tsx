import { useTranslation } from "react-i18next";
import type { BehaviorFingerprintReport } from "../lib/api";

export type BehaviorFingerprintPanelProps = {
  data?: BehaviorFingerprintReport | null;
  loading?: boolean;
  error?: string | null;
  durable?: string;
};

function formatMsgPerMin(n: number): string {
  if (!Number.isFinite(n) || n < 0) return "—";
  if (n >= 10) return Math.round(n).toLocaleString("en-US");
  return n.toFixed(1).replace(/\.0$/, "");
}

function formatProcessingMs(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return "—";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  const sec = ms / 1000;
  if (sec < 10) return `${sec.toFixed(1).replace(/\.0$/, "")}s`;
  return `${Math.round(sec)}s`;
}

function snapshotLine(
  snap: { msgPerMin: number; processingMs: number } | undefined,
  t: (key: string, opts?: Record<string, string>) => string,
): string {
  if (!snap) return t("consumers.fingerprintUnavailable");
  return t("consumers.fingerprintSnapshot", {
    rate: formatMsgPerMin(snap.msgPerMin),
    processing: formatProcessingMs(snap.processingMs),
  });
}

export default function BehaviorFingerprintPanel({
  data,
  loading,
  error,
  durable,
}: BehaviorFingerprintPanelProps) {
  const { t } = useTranslation();
  const title = durable || data?.durable || t("consumers.fingerprintTitle");
  const available = Boolean(data?.available && data.normal && data.current);

  return (
    <div className="nc-blast-radius nc-behavior-fingerprint">
      <h3 className="nc-blast-radius__title">{title}</h3>
      <p className="nc-blast-radius__subtitle">{t("consumers.fingerprintSubtitle")}</p>

      {loading && !available ? (
        <p className="nc-blast-radius__status">{t("consumers.fingerprintLoading")}</p>
      ) : null}

      {error && !available ? (
        <p className="nc-blast-radius__status nc-blast-radius__status--error" role="status">
          {t("consumers.fingerprintFailed")}
          {typeof error === "string" && error !== "error" ? (
            <>
              {" "}
              <span className="nc-blast-radius__error-detail">({error})</span>
            </>
          ) : null}
        </p>
      ) : null}

      {!loading && !error && data && !available ? (
        <p className="nc-blast-radius__status">{t("consumers.fingerprintIdle")}</p>
      ) : null}

      {available && data ? (
        <>
          <div className="nc-blast-radius__section">
            <div className="nc-blast-radius__section-label">{t("consumers.fingerprintCompare")}</div>
            <dl className="nc-replay-dry-run__stats">
              <div className="nc-replay-dry-run__row">
                <dt>{t("consumers.fingerprintNormal")}</dt>
                <dd>{snapshotLine(data.normal, t)}</dd>
              </div>
              <div className="nc-replay-dry-run__row">
                <dt>{t("consumers.fingerprintCurrent")}</dt>
                <dd>{snapshotLine(data.current, t)}</dd>
              </div>
            </dl>
          </div>

          {data.anomaly ? (
            <div className="nc-blast-radius__section">
              <div className="nc-blast-radius__section-label nc-blast-radius__section-label--critical">
                {t("consumers.fingerprintAnomaly")}
              </div>
            </div>
          ) : null}
        </>
      ) : null}
    </div>
  );
}

export { formatMsgPerMin, formatProcessingMs };
