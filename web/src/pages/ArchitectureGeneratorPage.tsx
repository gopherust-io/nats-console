import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import PageHeader from "../components/ui/PageHeader";
import {
  ARCHITECTURE_EXPORT_FORMATS,
  downloadArchitectureExport,
} from "../lib/architectureExport";
import { fetchAssistantConfig } from "../lib/assistant";
import { useCluster } from "../lib/cluster";

export default function ArchitectureGeneratorPage() {
  const { t } = useTranslation();
  const { clusterId } = useCluster();
  const [includeAI, setIncludeAI] = useState(true);
  const [busy, setBusy] = useState<"live" | "demo" | null>(null);
  const [error, setError] = useState<string | null>(null);

  const assistantConfigQuery = useQuery({
    queryKey: ["assistant-config"],
    queryFn: fetchAssistantConfig,
    staleTime: 60_000,
  });
  const aiEnabled = Boolean(assistantConfigQuery.data?.aiEnabled);

  const runExport = useCallback(
    async (demo: boolean) => {
      setError(null);
      setBusy(demo ? "demo" : "live");
      try {
        await downloadArchitectureExport(demo ? null : clusterId, {
          demo,
          fresh: true,
          ai: !demo && includeAI && aiEnabled,
        });
      } catch (err) {
        setError(err instanceof Error ? err.message : t("archExport.failed"));
      } finally {
        setBusy(null);
      }
    },
    [aiEnabled, clusterId, includeAI, t],
  );

  return (
    <>
      <PageHeader title={t("archExport.title")} subtitle={t("archExport.subtitle")} />

      <section className="arch-export" aria-label={t("archExport.title")}>
        <p className="arch-export__lead">{t("archExport.lead")}</p>
        <ul className="arch-export__formats">
          {ARCHITECTURE_EXPORT_FORMATS.map((label) => (
            <li key={label}>{label}</li>
          ))}
        </ul>

        {aiEnabled && (
          <label className="arch-export__ai">
            <input
              type="checkbox"
              checked={includeAI}
              onChange={(e) => setIncludeAI(e.target.checked)}
            />
            {t("archExport.includeAi")}
          </label>
        )}

        <div className="live-arch-page__toolbar" role="group" aria-label={t("archExport.actionsAria")}>
          <button
            type="button"
            className="btn btn--primary"
            disabled={!clusterId || busy !== null}
            onClick={() => void runExport(false)}
          >
            {busy === "live" ? t("archExport.generating") : t("archExport.generate")}
          </button>
          <button
            type="button"
            className="btn btn--secondary"
            disabled={busy !== null}
            onClick={() => void runExport(true)}
          >
            {busy === "demo" ? t("archExport.generating") : t("archExport.downloadSample")}
          </button>
        </div>

        {!clusterId && <p className="text-muted">{t("archExport.needSystem")}</p>}
        {error && <p className="arch-export__error">{error}</p>}
      </section>
    </>
  );
}
