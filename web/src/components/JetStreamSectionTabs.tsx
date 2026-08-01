import { Link } from "react-router";
import { useTranslation } from "react-i18next";

export type JetStreamSection = "overview" | "streams" | "consumers" | "messages";

type Props = {
  base: string;
  active: JetStreamSection;
  /** Extra query/hash-free path for section links (defaults to base). */
  state?: unknown;
};

function sectionHref(base: string, section: JetStreamSection): string {
  if (section === "streams") return base;
  return `${base}?tab=${section}`;
}

export default function JetStreamSectionTabs({ base, active, state }: Props) {
  const { t } = useTranslation();
  const tabs: { id: JetStreamSection; label: string }[] = [
    { id: "overview", label: t("streams.tabOverview") },
    { id: "streams", label: t("streams.tabStreams") },
    { id: "consumers", label: t("streams.tabConsumers") },
    { id: "messages", label: t("streams.tabMessages") },
  ];

  return (
    <nav className="nc-tabs stream-tabs" aria-label={t("jetstream.sectionsAria")}>
      {tabs.map(({ id, label }) => (
        <Link
          key={id}
          to={sectionHref(base, id)}
          state={state}
          className={`nc-tab${active === id ? " active" : ""}`}
          aria-current={active === id ? "page" : undefined}
        >
          {label}
        </Link>
      ))}
    </nav>
  );
}

export function parseJetStreamSection(raw: string | null): JetStreamSection {
  if (
    raw === "overview" ||
    raw === "consumers" ||
    raw === "messages" ||
    raw === "streams"
  ) {
    return raw;
  }
  return "streams";
}
