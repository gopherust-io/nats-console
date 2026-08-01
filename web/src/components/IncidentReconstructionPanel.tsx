import { useTranslation } from "react-i18next";
import type { IncidentReconstruction, IncidentTimelineEvent } from "../lib/api";

export type IncidentReconstructionPanelProps = {
  data?: IncidentReconstruction | null;
  loading?: boolean;
  error?: string | null;
};

function formatEventTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function eventKey(ev: IncidentTimelineEvent, index: number): string {
  return `${ev.at}:${ev.category}:${ev.label}:${index}`;
}

export default function IncidentReconstructionPanel({
  data,
  loading,
  error,
}: IncidentReconstructionPanelProps) {
  const { t } = useTranslation();
  const events = data?.events ?? [];
  const hasEvents = events.length > 0;

  return (
    <div className="nc-blast-radius nc-incident-reconstruction">
      <h3 className="nc-blast-radius__title">{t("audit.incidentTitle")}</h3>
      <p className="nc-blast-radius__subtitle">{t("audit.incidentSubtitle")}</p>

      {loading && !data ? (
        <p className="nc-blast-radius__status">{t("audit.incidentLoading")}</p>
      ) : null}

      {error && !data ? (
        <p className="nc-blast-radius__status nc-blast-radius__status--error" role="status">
          {t("audit.incidentFailed")}
          {typeof error === "string" && error !== "error" ? (
            <>
              {" "}
              <span className="nc-blast-radius__error-detail">({error})</span>
            </>
          ) : null}
        </p>
      ) : null}

      {!loading && !error && data && !hasEvents ? (
        <p className="nc-blast-radius__status">{t("audit.incidentEmpty")}</p>
      ) : null}

      {hasEvents ? (
        <div className="nc-blast-radius__section">
          <div className="nc-blast-radius__section-label">{t("audit.incidentTimeline")}</div>
          <ol className="nc-incident-reconstruction__list" aria-label={t("audit.incidentTimeline")}>
            {events.map((ev, index) => (
              <li key={eventKey(ev, index)} className="nc-incident-reconstruction__item">
                <time className="nc-incident-reconstruction__time" dateTime={ev.at}>
                  {formatEventTime(ev.at)}
                </time>
                <div className="nc-incident-reconstruction__body">
                  <span className="nc-incident-reconstruction__label">{ev.label}</span>
                  {ev.evidence ? (
                    <span className="nc-incident-reconstruction__evidence">{ev.evidence}</span>
                  ) : null}
                </div>
              </li>
            ))}
          </ol>
          {data?.usedAuditFallback ? (
            <p className="nc-incident-reconstruction__note">{t("audit.incidentAuditNote")}</p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

