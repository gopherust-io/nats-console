import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import type { HiddenBottleneckSnapshot } from "../lib/hiddenBottlenecks";
import { HIDDEN_BOTTLENECKS_HREF } from "../lib/hiddenBottlenecks";
import { useCluster } from "../lib/cluster";

type HiddenBottlenecksPanelProps = {
  snapshot: HiddenBottleneckSnapshot;
  reply?: string | null;
  asking?: boolean;
  aiEnabled?: boolean;
  onAsk?: () => void;
  sample?: boolean;
  filterConsumer?: string;
};

export default function HiddenBottlenecksPanel({
  snapshot,
  reply,
  asking,
  aiEnabled,
  onAsk,
  sample,
  filterConsumer,
}: HiddenBottlenecksPanelProps) {
  const { t } = useTranslation();
  const { clusterId } = useCluster();
  const verdictKey =
    snapshot.verdict === "at_risk"
      ? "hiddenBottlenecks.verdictAtRisk"
      : snapshot.verdict === "needs_attention"
        ? "hiddenBottlenecks.verdictNeedsAttention"
        : "hiddenBottlenecks.verdictHealthy";

  const findings = filterConsumer
    ? snapshot.findings.filter((f) => f.consumer === filterConsumer)
    : snapshot.findings;

  return (
    <section className="arch-review" aria-label={t("hiddenBottlenecks.panelAria")}>
      <div className="arch-review__header">
        <div>
          <p className="arch-review__question">{t("hiddenBottlenecks.question")}</p>
          <div className="arch-review__verdict-row">
            <span className={`arch-review__verdict arch-review__verdict--${snapshot.verdict}`}>
              {t(verdictKey)}
            </span>
            {(sample || snapshot.demo) && (
              <span className="badge">{t("hiddenBottlenecks.sampleBadge")}</span>
            )}
            <span className="badge">
              {t("hiddenBottlenecks.findingCount", { count: findings.length })}
            </span>
          </div>
        </div>
        {onAsk && (
          <button
            type="button"
            className="btn btn--primary"
            onClick={onAsk}
            disabled={asking || !aiEnabled}
            title={!aiEnabled ? t("hiddenBottlenecks.aiDisabledHint") : undefined}
          >
            {asking ? t("hiddenBottlenecks.asking") : t("hiddenBottlenecks.askAi")}
          </button>
        )}
      </div>

      {reply && (
        <div className="arch-review__reply">
          <h3 className="arch-review__section-title">{t("hiddenBottlenecks.aiReply")}</h3>
          <pre className="arch-review__reply-body">{reply}</pre>
        </div>
      )}

      <div className="arch-review__columns">
        <div>
          <h3 className="arch-review__section-title">{t("hiddenBottlenecks.findings")}</h3>
          {findings.length === 0 ? (
            <p className="arch-review__empty">{t("hiddenBottlenecks.noFindings")}</p>
          ) : (
            <ul className="arch-review__list">
              {findings.map((f, i) => (
                <li
                  key={`${f.kind}-${f.title}-${i}`}
                  className={`arch-review__item arch-review__item--${f.severity}`}
                >
                  <div className="arch-review__item-top">
                    <span className="arch-review__kind">{kindLabel(t, f.kind)}</span>
                    <span className="arch-review__severity">{f.severity}</span>
                  </div>
                  <div className="arch-review__title">{f.title}</div>
                  {f.schedule && (
                    <div className="arch-review__evidence">
                      <span className="arch-review__chip">{f.schedule}</span>
                      {f.stream && clusterId ? (
                        <Link
                          className="arch-review__chip"
                          to={`/systems/${clusterId}/accounts/Default/jetstream/streams/${encodeURIComponent(f.stream)}`}
                        >
                          {f.stream}
                        </Link>
                      ) : null}
                      {f.consumer && f.stream && clusterId ? (
                        <Link
                          className="arch-review__chip"
                          to={`/systems/${clusterId}/accounts/Default/jetstream/streams/${encodeURIComponent(f.stream)}/consumers/${encodeURIComponent(f.consumer)}`}
                        >
                          {f.consumer}
                        </Link>
                      ) : null}
                    </div>
                  )}
                  {f.evidence?.length > 0 && (
                    <div className="arch-review__evidence">
                      {f.evidence.slice(0, 6).map((e) => (
                        <span key={e} className="arch-review__chip">
                          {e}
                        </span>
                      ))}
                    </div>
                  )}
                  {f.suggestion && <p className="arch-review__suggestion">{f.suggestion}</p>}
                </li>
              ))}
            </ul>
          )}
        </div>
        <div>
          <h3 className="arch-review__section-title">{t("hiddenBottlenecks.suggestions")}</h3>
          {snapshot.suggestions.length === 0 ? (
            <p className="arch-review__empty">{t("hiddenBottlenecks.noSuggestions")}</p>
          ) : (
            <ul className="arch-review__suggestions">
              {snapshot.suggestions.map((s) => (
                <li key={s}>{s}</li>
              ))}
            </ul>
          )}
          <p className="arch-review__empty" style={{ marginTop: "1rem" }}>
            <Link to={HIDDEN_BOTTLENECKS_HREF}>{t("hiddenBottlenecks.openDocs")}</Link>
          </p>
        </div>
      </div>
    </section>
  );
}

function kindLabel(t: (key: string) => string, kind: string): string {
  switch (kind) {
    case "correlated_payload_lag":
      return t("hiddenBottlenecks.kindCorrelated");
    case "schedule_lag_spike":
      return t("hiddenBottlenecks.kindLagSpike");
    case "schedule_payload_growth":
      return t("hiddenBottlenecks.kindPayload");
    case "schedule_processing_slow":
      return t("hiddenBottlenecks.kindProcessing");
    default:
      return kind;
  }
}
