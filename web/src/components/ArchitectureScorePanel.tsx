import { useTranslation } from "react-i18next";
import type {
  ArchitectureScoreFactor,
  ArchitectureScoreSnapshot,
  ArchitectureScoreTrendPoint,
} from "../lib/architectureScore";

type ArchitectureScorePanelProps = {
  snapshot: ArchitectureScoreSnapshot;
  reply?: string | null;
  asking?: boolean;
  aiEnabled?: boolean;
  onAsk?: () => void;
  sample?: boolean;
};

export default function ArchitectureScorePanel({
  snapshot,
  reply,
  asking,
  aiEnabled,
  onAsk,
  sample,
}: ArchitectureScorePanelProps) {
  const { t } = useTranslation();

  return (
    <section className="arch-score" aria-label={t("archScore.panelAria")}>
      <div className="arch-review__header">
        <div>
          <p className="arch-review__question">{t("archScore.question")}</p>
          <div className="arch-score__hero">
            <span className="arch-score__value" aria-label={`${snapshot.score} of ${snapshot.maxScore}`}>
              {snapshot.score}
              <span className="arch-score__max">/{snapshot.maxScore}</span>
            </span>
            <div className="arch-review__verdict-row">
              {(sample || snapshot.demo) && <span className="badge">{t("archScore.sampleBadge")}</span>}
            </div>
          </div>
          {snapshot.verdict && <p className="arch-score__verdict">{snapshot.verdict}</p>}
        </div>
        {onAsk && (
          <button
            type="button"
            className="btn btn--primary"
            onClick={onAsk}
            disabled={asking || !aiEnabled}
            title={!aiEnabled ? t("archScore.aiDisabledHint") : undefined}
          >
            {asking ? t("archScore.asking") : t("archScore.askAi")}
          </button>
        )}
      </div>

      {reply && (
        <div className="arch-review__reply">
          <h3 className="arch-review__section-title">{t("archScore.aiReply")}</h3>
          <pre className="arch-review__reply-body">{reply}</pre>
        </div>
      )}

      <div className="arch-score__columns">
        <div>
          <h3 className="arch-review__section-title">{t("archScore.factors")}</h3>
          {snapshot.factors.length === 0 ? (
            <p className="arch-review__empty">{t("archScore.noFactors")}</p>
          ) : (
            <ul className="arch-score__factors">
              {snapshot.factors.map((f) => (
                <FactorRow key={`${f.id}-${f.label}`} factor={f} />
              ))}
            </ul>
          )}
        </div>
        <div>
          <h3 className="arch-review__section-title">{t("archScore.trend")}</h3>
          {snapshot.trend.length === 0 ? (
            <p className="arch-review__empty">{t("archScore.noTrend")}</p>
          ) : (
            <TrendChart points={snapshot.trend} />
          )}
        </div>
      </div>
    </section>
  );
}

function FactorRow({ factor }: { factor: ArchitectureScoreFactor }) {
  const plus = factor.sign === "plus" || factor.delta > 0;
  const sign = plus ? "+" : "−";
  const delta = Math.abs(factor.delta);
  return (
    <li className={`arch-score__factor arch-score__factor--${plus ? "plus" : "minus"}`}>
      <span className="arch-score__factor-sign">
        {sign} {factor.label}
      </span>
      <span className="arch-score__factor-delta">
        {plus ? "+" : "−"}
        {delta}
      </span>
    </li>
  );
}

function TrendChart({ points }: { points: ArchitectureScoreTrendPoint[] }) {
  const w = 320;
  const h = 120;
  const pad = 12;
  const scores = points.map((p) => p.score);
  const min = Math.min(...scores, 0);
  const max = Math.max(...scores, 100);
  const span = Math.max(max - min, 1);
  const coords = points.map((p, i) => {
    const x = pad + (i / Math.max(points.length - 1, 1)) * (w - pad * 2);
    const y = h - pad - ((p.score - min) / span) * (h - pad * 2);
    return `${x},${y}`;
  });
  const poly = coords.join(" ");

  return (
    <div className="arch-score__trend">
      <svg viewBox={`0 0 ${w} ${h}`} className="arch-score__trend-svg" role="img" aria-label="score trend">
        <polyline fill="none" stroke="currentColor" strokeWidth="2" points={poly} />
        {points.map((p, i) => {
          const [x, y] = coords[i].split(",").map(Number);
          return <circle key={p.period} cx={x} cy={y} r="3" fill="currentColor" />;
        })}
      </svg>
      <ul className="arch-score__trend-legend">
        {points.map((p) => (
          <li key={p.period}>
            <span>{p.period}</span>
            <strong>{p.score}</strong>
          </li>
        ))}
      </ul>
    </div>
  );
}
