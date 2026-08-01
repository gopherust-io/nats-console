import { useTranslation } from "react-i18next";
import type { ArchitectureRefactorGraph, ArchitectureRefactorPlan } from "../lib/architectureRefactor";

type ArchitectureRefactorPanelProps = {
  plan: ArchitectureRefactorPlan;
  reply?: string | null;
  asking?: boolean;
  aiEnabled?: boolean;
  onAsk?: () => void;
  sample?: boolean;
};

export default function ArchitectureRefactorPanel({
  plan,
  reply,
  asking,
  aiEnabled,
  onAsk,
  sample,
}: ArchitectureRefactorPanelProps) {
  const { t } = useTranslation();

  return (
    <section className="arch-refactor" aria-label={t("archRefactor.panelAria")}>
      <div className="arch-review__header">
        <div>
          <p className="arch-review__question">{plan.question || t("archRefactor.question")}</p>
          <div className="arch-review__verdict-row">
            <span className="badge">{plan.verdict}</span>
            {(sample || plan.demo) && <span className="badge">{t("archRefactor.sampleBadge")}</span>}
            {plan.eventSubject && (
              <span className="badge">
                {t("archRefactor.eventLabel")}: {plan.eventSubject}
              </span>
            )}
          </div>
          {plan.rationale && <p className="arch-refactor__rationale">{plan.rationale}</p>}
        </div>
        {onAsk && (
          <button
            type="button"
            className="btn btn--primary"
            onClick={onAsk}
            disabled={asking || !aiEnabled}
            title={!aiEnabled ? t("archRefactor.aiDisabledHint") : undefined}
          >
            {asking ? t("archRefactor.asking") : t("archRefactor.askAi")}
          </button>
        )}
      </div>

      {reply && (
        <div className="arch-review__reply">
          <h3 className="arch-review__section-title">{t("archRefactor.aiReply")}</h3>
          <pre className="arch-review__reply-body">{reply}</pre>
        </div>
      )}

      <div className="arch-refactor__graphs">
        <GraphCard title={t("archRefactor.before")} graph={plan.before} />
        <GraphCard title={t("archRefactor.after")} graph={plan.after} />
      </div>

      <div>
        <h3 className="arch-review__section-title">{t("archRefactor.steps")}</h3>
        {plan.steps.length === 0 ? (
          <p className="arch-review__empty">{t("archRefactor.noSteps")}</p>
        ) : (
          <ol className="arch-refactor__steps">
            {[...plan.steps]
              .sort((a, b) => a.order - b.order)
              .map((s) => (
                <li key={`${s.order}-${s.title}`}>
                  <strong>{s.title}</strong>
                  <p>{s.detail}</p>
                </li>
              ))}
          </ol>
        )}
      </div>
    </section>
  );
}

function GraphCard({ title, graph }: { title: string; graph: ArchitectureRefactorGraph }) {
  const chain = graphChainLabel(graph);
  return (
    <div className="arch-refactor__graph-card">
      <h3 className="arch-review__section-title">{title}</h3>
      {graph.label && <p className="arch-refactor__graph-label">{graph.label}</p>}
      <div className="arch-refactor__chain" aria-label={chain}>
        {graph.nodes.map((n, i) => (
          <span key={n.id} className="arch-refactor__chain-item">
            {i > 0 && <span className="arch-refactor__arrow">→</span>}
            <span className={`arch-refactor__node arch-refactor__node--${n.kind}`}>{n.label}</span>
          </span>
        ))}
      </div>
      {graph.edges.length > 0 && (
        <ul className="arch-refactor__edges">
          {graph.edges.map((e) => (
            <li key={`${e.from}-${e.to}-${e.label}`}>
              {nodeLabel(graph, e.from)} → {nodeLabel(graph, e.to)}
              {e.label ? ` (${e.label})` : ""}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function nodeLabel(graph: ArchitectureRefactorGraph, id: string): string {
  return graph.nodes.find((n) => n.id === id)?.label ?? id;
}

function graphChainLabel(graph: ArchitectureRefactorGraph): string {
  return graph.nodes.map((n) => n.label).join(" → ");
}
