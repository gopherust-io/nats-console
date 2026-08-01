import { useTranslation } from "react-i18next";
import type { ChaosStory, ChaosStorySeed } from "../lib/chaosStory";

type ChaosStoryPanelProps = {
  story: ChaosStory;
  seed?: ChaosStorySeed;
  sample?: boolean;
  actIndex: number;
  simulating: boolean;
  paused: boolean;
  generating?: boolean;
  aiEnabled?: boolean;
  onGenerate?: () => void;
  onSimulate: () => void;
  onPause: () => void;
  onReset: () => void;
  error?: string | null;
};

function kindLabel(t: (k: string) => string, kind: string): string {
  const map: Record<string, string> = {
    cluster_down: "chaosStory.kindClusterDown",
    quorum_loss: "chaosStory.kindQuorumLoss",
    schema_mismatch: "chaosStory.kindSchemaMismatch",
    consumer_deploy: "chaosStory.kindConsumerDeploy",
    traffic_spike: "chaosStory.kindTrafficSpike",
    partition: "chaosStory.kindPartition",
    recovery: "chaosStory.kindRecovery",
  };
  const key = map[kind];
  return key ? t(key) : kind;
}

export default function ChaosStoryPanel({
  story,
  seed,
  sample,
  actIndex,
  simulating,
  paused,
  generating,
  aiEnabled,
  onGenerate,
  onSimulate,
  onPause,
  onReset,
  error,
}: ChaosStoryPanelProps) {
  const { t } = useTranslation();
  const progress =
    story.acts.length > 0 ? Math.round(((actIndex + 1) / story.acts.length) * 100) : 0;

  return (
    <section className="chaos-story" aria-label={t("chaosStory.panelAria")}>
      <div className="chaos-story__banner" role="status">
        {t("chaosStory.narrativeOnly")}
      </div>

      <div className="chaos-story__header">
        <div>
          <h2 className="chaos-story__title">{story.title}</h2>
          <div className="chaos-story__meta">
            <span className={`chaos-story__severity chaos-story__severity--${story.severity}`}>
              {story.severity}
            </span>
            {(sample || story.demo) && <span className="badge">{t("chaosStory.sampleBadge")}</span>}
            {story.setting && <span className="badge">{story.setting}</span>}
          </div>
          <p className="chaos-story__summary">{story.summary}</p>
        </div>
        <div className="chaos-story__actions">
          {onGenerate && (
            <button
              type="button"
              className="btn btn--ghost"
              onClick={onGenerate}
              disabled={generating || !aiEnabled || simulating}
              title={!aiEnabled ? t("chaosStory.aiDisabledHint") : undefined}
            >
              {generating ? t("chaosStory.generating") : t("chaosStory.generate")}
            </button>
          )}
          {!simulating || paused ? (
            <button type="button" className="btn btn--primary" onClick={onSimulate} disabled={story.acts.length === 0}>
              {paused ? t("chaosStory.resume") : t("chaosStory.simulate")}
            </button>
          ) : (
            <button type="button" className="btn btn--secondary" onClick={onPause}>
              {t("chaosStory.pause")}
            </button>
          )}
          <button type="button" className="btn btn--ghost" onClick={onReset} disabled={!simulating && actIndex === 0}>
            {t("chaosStory.reset")}
          </button>
        </div>
      </div>

      {error && <p className="chaos-story__error">{error}</p>}

      {(simulating || actIndex > 0) && (
        <div className="chaos-story__progress" aria-valuenow={progress} aria-valuemin={0} aria-valuemax={100}>
          <div className="chaos-story__progress-bar" style={{ width: `${progress}%` }} />
          <span className="chaos-story__progress-label">
            {t("chaosStory.actProgress", { current: actIndex + 1, total: story.acts.length })}
          </span>
        </div>
      )}

      <h3 className="chaos-story__section-title">{t("chaosStory.acts")}</h3>
      <ol className="chaos-story__acts">
        {story.acts.map((act, i) => (
          <li
            key={`${act.title}-${i}`}
            className={`chaos-story__act${i === actIndex && simulating ? " is-active" : ""}${i < actIndex ? " is-done" : ""}`}
          >
            <div className="chaos-story__act-top">
              <span className="chaos-story__kind">{kindLabel(t, act.kind)}</span>
              <span className="text-muted">{act.durationSec}s</span>
            </div>
            <div className="chaos-story__act-title">{act.title}</div>
            <p className="chaos-story__act-desc">{act.description}</p>
            {act.targets && act.targets.length > 0 && (
              <div className="chaos-story__chips">
                {act.targets.map((target) => (
                  <span key={target} className="chaos-story__chip">
                    {target}
                  </span>
                ))}
              </div>
            )}
          </li>
        ))}
      </ol>

      <div className="chaos-story__columns">
        <div>
          <h3 className="chaos-story__section-title">{t("chaosStory.blastRadius")}</h3>
          {(story.blastRadius?.length ?? 0) === 0 ? (
            <p className="text-muted">{t("chaosStory.emptyList")}</p>
          ) : (
            <ul>
              {story.blastRadius!.map((b) => (
                <li key={b}>{b}</li>
              ))}
            </ul>
          )}
        </div>
        <div>
          <h3 className="chaos-story__section-title">{t("chaosStory.recoveryHints")}</h3>
          {(story.recoveryHints?.length ?? 0) === 0 ? (
            <p className="text-muted">{t("chaosStory.emptyList")}</p>
          ) : (
            <ul>
              {story.recoveryHints!.map((h) => (
                <li key={h}>{h}</li>
              ))}
            </ul>
          )}
        </div>
      </div>

      {seed && (seed.streams.length > 0 || seed.consumers.length > 0) && (
        <div className="chaos-story__seed">
          <h3 className="chaos-story__section-title">{t("chaosStory.seed")}</h3>
          <p className="text-muted">{t("chaosStory.seedHint")}</p>
          <div className="chaos-story__chips">
            {seed.streams.slice(0, 12).map((s) => (
              <span key={`s-${s}`} className="chaos-story__chip">
                {s}
              </span>
            ))}
            {seed.consumers.slice(0, 12).map((c) => (
              <span key={`c-${c}`} className="chaos-story__chip">
                {c}
              </span>
            ))}
          </div>
        </div>
      )}
    </section>
  );
}
