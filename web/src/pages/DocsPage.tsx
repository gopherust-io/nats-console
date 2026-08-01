import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import PageHeader from "../components/ui/PageHeader";

type DocsCard = {
  key: string;
  to?: string;
  href?: string;
  titleKey: string;
  descKey: string;
  icon: ReactNode;
};

function DocsIcon({ children }: { children: ReactNode }) {
  return (
    <span className="nc-system-card__icon" aria-hidden="true">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
        {children}
      </svg>
    </span>
  );
}

const ICONS = {
  catalog: (
    <DocsIcon>
      <path d="M4 6h16M4 12h16M4 18h10" strokeLinecap="round" />
    </DocsIcon>
  ),
  wiki: (
    <DocsIcon>
      <path d="M4 19V5a2 2 0 0 1 2-2h9l5 5v11a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2Z" strokeLinejoin="round" />
      <path d="M14 3v5h5" strokeLinejoin="round" />
    </DocsIcon>
  ),
  liveArch: (
    <DocsIcon>
      <circle cx="12" cy="12" r="3" />
      <path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" strokeLinecap="round" />
    </DocsIcon>
  ),
  review: (
    <DocsIcon>
      <path d="M9 11l3 3L22 4" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" strokeLinecap="round" />
    </DocsIcon>
  ),
  refactor: (
    <DocsIcon>
      <path d="M4 7h10M14 7l-3-3M14 7l-3 3M20 17H10M10 17l3-3M10 17l3 3" strokeLinecap="round" strokeLinejoin="round" />
    </DocsIcon>
  ),
  score: (
    <DocsIcon>
      <path d="M12 17a5 5 0 1 0-5-5" strokeLinecap="round" />
      <path d="M12 17V9l4-2" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M4.5 12a7.5 7.5 0 0 1 15 0" strokeLinecap="round" />
    </DocsIcon>
  ),
  bottlenecks: (
    <DocsIcon>
      <path d="M4 12h4l2-6 4 12 2-6h4" strokeLinecap="round" strokeLinejoin="round" />
    </DocsIcon>
  ),
  chaos: (
    <DocsIcon>
      <path d="M13 2 4 14h7l-1 8 9-12h-7l1-8Z" strokeLinejoin="round" />
    </DocsIcon>
  ),
  generator: (
    <DocsIcon>
      <rect x="4" y="4" width="16" height="16" rx="2" />
      <path d="M9 9h6M9 12h6M9 15h3" strokeLinecap="round" />
    </DocsIcon>
  ),
  api: (
    <DocsIcon>
      <path d="M8 8 4 12l4 4M16 8l4 4-4 4M13 5l-2 14" strokeLinecap="round" strokeLinejoin="round" />
    </DocsIcon>
  ),
};

export default function DocsPage() {
  const { t } = useTranslation();

  const cards: DocsCard[] = [
    {
      key: "event-catalog",
      to: "/docs/event-catalog",
      titleKey: "nav.eventCatalog",
      descKey: "docs.eventCatalogDesc",
      icon: ICONS.catalog,
    },
    {
      key: "event-wikipedia",
      to: "/docs/event-wikipedia",
      titleKey: "nav.eventWikipedia",
      descKey: "docs.eventWikipediaDesc",
      icon: ICONS.wiki,
    },
    {
      key: "live-architecture",
      to: "/docs/live-architecture",
      titleKey: "nav.liveArchitecture",
      descKey: "docs.liveArchitectureDesc",
      icon: ICONS.liveArch,
    },
    {
      key: "architecture-review",
      to: "/docs/architecture-review",
      titleKey: "nav.architectureReview",
      descKey: "docs.architectureReviewDesc",
      icon: ICONS.review,
    },
    {
      key: "architecture-refactor",
      to: "/docs/architecture-refactor",
      titleKey: "nav.architectureRefactor",
      descKey: "docs.architectureRefactorDesc",
      icon: ICONS.refactor,
    },
    {
      key: "architecture-score",
      to: "/docs/architecture-score",
      titleKey: "nav.architectureScore",
      descKey: "docs.architectureScoreDesc",
      icon: ICONS.score,
    },
    {
      key: "hidden-bottlenecks",
      to: "/docs/hidden-bottlenecks",
      titleKey: "nav.hiddenBottlenecks",
      descKey: "docs.hiddenBottlenecksDesc",
      icon: ICONS.bottlenecks,
    },
    {
      key: "chaos-story",
      to: "/docs/chaos-story",
      titleKey: "nav.chaosStory",
      descKey: "docs.chaosStoryDesc",
      icon: ICONS.chaos,
    },
    {
      key: "architecture-generator",
      to: "/docs/architecture-generator",
      titleKey: "nav.architectureGenerator",
      descKey: "docs.architectureGeneratorDesc",
      icon: ICONS.generator,
    },
    {
      key: "api-docs",
      href: "/api/openapi.yaml",
      titleKey: "nav.apiDocs",
      descKey: "docs.apiReferenceDesc",
      icon: ICONS.api,
    },
  ];

  return (
    <div className="docs-page">
      <PageHeader title={t("docs.title")} subtitle={t("docs.subtitle")} />
      <div className="nc-card-grid">
        {cards.map((card) => {
          const body = (
            <>
              {card.icon}
              <div className="nc-system-card__body">
                <div>
                  <div className="nc-system-card__name">{t(card.titleKey)}</div>
                  <div className="nc-system-card__meta">
                    <span>{t(card.descKey)}</span>
                  </div>
                </div>
              </div>
            </>
          );
          if (card.href) {
            return (
              <a
                key={card.key}
                className="nc-system-card"
                href={card.href}
                target="_blank"
                rel="noreferrer"
              >
                {body}
              </a>
            );
          }
          return (
            <Link key={card.key} className="nc-system-card" to={card.to!}>
              {body}
            </Link>
          );
        })}
      </div>
    </div>
  );
}
