import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import {
  architectureRefactorHref,
  isCouplingFindingKind,
} from "../lib/architectureRefactor";
import { ARCHITECTURE_SCORE_HREF } from "../lib/architectureScore";
import type { ArchitectureReviewSnapshot } from "../lib/architectureReview";

type ArchitectureReviewPanelProps = {
  snapshot: ArchitectureReviewSnapshot;
  reply?: string | null;
  asking?: boolean;
  aiEnabled?: boolean;
  onAsk?: () => void;
  sample?: boolean;
  score?: number | null;
  maxScore?: number;
};

export default function ArchitectureReviewPanel({
  snapshot,
  reply,
  asking,
  aiEnabled,
  onAsk,
  sample,
  score,
  maxScore = 100,
}: ArchitectureReviewPanelProps) {
  const { t } = useTranslation();
  const verdictKey =
    snapshot.verdict === "at_risk"
      ? "archReview.verdictAtRisk"
      : snapshot.verdict === "needs_attention"
        ? "archReview.verdictNeedsAttention"
        : "archReview.verdictHealthy";

  return (
    <section className="arch-review" aria-label={t("archReview.panelAria")}>
      <div className="arch-review__header">
        <div>
          <p className="arch-review__question">{t("archReview.question")}</p>
          <div className="arch-review__verdict-row">
            <span className={`arch-review__verdict arch-review__verdict--${snapshot.verdict}`}>
              {t(verdictKey)}
            </span>
            {(sample || snapshot.demo) && (
              <span className="badge">{t("archReview.sampleBadge")}</span>
            )}
            <span className="badge">
              {t("archReview.problemCount", { count: snapshot.totals.problems })}
            </span>
            {typeof score === "number" && (
              <Link className="arch-review__score-chip" to={ARCHITECTURE_SCORE_HREF}>
                {t("archReview.scoreChip", { score, max: maxScore })}
              </Link>
            )}
          </div>
        </div>
        {onAsk && (
          <button
            type="button"
            className="btn btn--primary"
            onClick={onAsk}
            disabled={asking || !aiEnabled}
            title={!aiEnabled ? t("archReview.aiDisabledHint") : undefined}
          >
            {asking ? t("archReview.asking") : t("archReview.askAi")}
          </button>
        )}
      </div>

      {reply && (
        <div className="arch-review__reply">
          <h3 className="arch-review__section-title">{t("archReview.aiReply")}</h3>
          <pre className="arch-review__reply-body">{reply}</pre>
        </div>
      )}

      <div className="arch-review__columns">
        <div>
          <h3 className="arch-review__section-title">{t("archReview.problems")}</h3>
          {snapshot.problems.length === 0 ? (
            <p className="arch-review__empty">{t("archReview.noProblems")}</p>
          ) : (
            <ul className="arch-review__list">
              {snapshot.problems.map((p, i) => (
                <li key={`${p.kind}-${p.title}-${i}`} className={`arch-review__item arch-review__item--${p.severity}`}>
                  <div className="arch-review__item-top">
                    <span className="arch-review__kind">{kindLabel(t, p.kind)}</span>
                    <span className="arch-review__severity">{p.severity}</span>
                  </div>
                  <div className="arch-review__title">{p.title}</div>
                  {p.evidence?.length > 0 && (
                    <div className="arch-review__evidence">
                      {p.evidence.slice(0, 6).map((e) => (
                        <span key={e} className="arch-review__chip">
                          {e}
                        </span>
                      ))}
                    </div>
                  )}
                  {p.suggestion && <p className="arch-review__suggestion">{p.suggestion}</p>}
                  {isCouplingFindingKind(p.kind) && (
                    <Link
                      className="arch-review__refactor-link"
                      to={architectureRefactorHref({
                        kind: p.kind,
                        stream: p.stream,
                        subject: p.subject,
                      })}
                    >
                      {t("archReview.reduceCoupling")}
                    </Link>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
        <div>
          <h3 className="arch-review__section-title">{t("archReview.suggestions")}</h3>
          {snapshot.suggestions.length === 0 ? (
            <p className="arch-review__empty">{t("archReview.noSuggestions")}</p>
          ) : (
            <ul className="arch-review__suggestions">
              {snapshot.suggestions.map((s) => (
                <li key={s}>{s}</li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </section>
  );
}

function kindLabel(t: (key: string) => string, kind: string): string {
  switch (kind) {
    case "too_many_consumers":
      return t("archReview.kindTooManyConsumers");
    case "circular_dependency":
      return t("archReview.kindCircular");
    case "tight_coupling":
      return t("archReview.kindCoupling");
    case "naming_inconsistent":
      return t("archReview.kindNaming");
    case "payload_too_large":
      return t("archReview.kindPayload");
    default:
      return kind;
  }
}
