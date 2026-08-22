import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router";
import ArchitectureRefactorPanel from "../components/ArchitectureRefactorPanel";
import PageHeader from "../components/ui/PageHeader";
import QueryErrorState from "../components/ui/QueryErrorState";
import {
  askArchitectureRefactor,
  demoArchitectureRefactor,
  fetchArchitectureRefactor,
  type ArchitectureRefactorPlan,
  type ArchitectureRefactorSeed,
} from "../lib/architectureRefactor";
import { fetchAssistantConfig } from "../lib/assistant";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

export default function ArchitectureRefactorPage() {
  const { t } = useTranslation();
  const { clusterId } = useCluster();
  const [searchParams] = useSearchParams();
  const seed: ArchitectureRefactorSeed = useMemo(
    () => ({
      kind: searchParams.get("kind") ?? undefined,
      stream: searchParams.get("stream") ?? undefined,
      subject: searchParams.get("subject") ?? undefined,
    }),
    [searchParams],
  );
  const [forceSample, setForceSample] = useState(false);
  const [aiReply, setAiReply] = useState<string | null>(null);
  const [asking, setAsking] = useState(false);
  const askGenRef = useRef(0);

  const planQuery = useQuery({
    queryKey: [
      ...clusterQueryKey(clusterId, "architecture-refactor"),
      seed.kind ?? "",
      seed.stream ?? "",
      seed.subject ?? "",
    ],
    queryFn: () =>
      fetchArchitectureRefactor(clusterId!, { ...seed }),
    enabled: Boolean(clusterId) && !forceSample,
    staleTime: 5 * 60_000,
    refetchInterval: false,
  });

  const assistantConfigQuery = useQuery({
    queryKey: ["assistant-config"],
    queryFn: fetchAssistantConfig,
    staleTime: 60_000,
  });

  const demo = useMemo(() => demoArchitectureRefactor(), []);
  const useDemo = forceSample || !clusterId;
  const plan: ArchitectureRefactorPlan = useDemo ? demo : (planQuery.data ?? demo);
  const sample = useDemo;
  const aiEnabled = Boolean(assistantConfigQuery.data?.aiEnabled) && Boolean(clusterId) && !sample;

  useEffect(() => {
    askGenRef.current += 1;
    setAiReply(null);
    setAsking(false);
  }, [clusterId, seed.kind, seed.stream, seed.subject, forceSample]);

  const handleAsk = useCallback(async () => {
    if (!clusterId || asking || sample) return;
    const gen = ++askGenRef.current;
    setAsking(true);
    try {
      const result = await askArchitectureRefactor(clusterId, undefined, { fresh: true, ...seed });
      if (gen !== askGenRef.current) return;
      setAiReply(result.reply);
    } catch {
      if (gen !== askGenRef.current) return;
      setAiReply(null);
    } finally {
      if (gen === askGenRef.current) setAsking(false);
    }
  }, [asking, clusterId, sample, seed]);

  return (
    <>
      <PageHeader title={t("archRefactor.title")} subtitle={t("archRefactor.subtitle")} />
      <div className="live-arch-page__toolbar" role="toolbar" aria-label={t("archRefactor.title")}>
        {!clusterId && <span className="text-muted">{t("archRefactor.needSystem")}</span>}
        {clusterId && (
          <button
            type="button"
            aria-pressed={forceSample}
            onClick={() => {
              setForceSample((v) => !v);
              setAiReply(null);
            }}
          >
            {forceSample ? t("archRefactor.useLive") : t("archRefactor.useSample")}
          </button>
        )}
      </div>
      {clusterId && planQuery.isError && !forceSample && (
        <QueryErrorState error={planQuery.error} onRetry={() => void planQuery.refetch()} />
      )}
      {clusterId && planQuery.isLoading && !forceSample && !planQuery.data && (
        <div className="skeleton skeleton--panel" />
      )}
      {(forceSample || (!planQuery.isError && (!planQuery.isLoading || planQuery.data)) || !clusterId) && (
        <ArchitectureRefactorPanel
          plan={plan}
          reply={aiReply}
          asking={asking}
          aiEnabled={aiEnabled}
          onAsk={sample ? undefined : () => void handleAsk()}
          sample={sample}
        />
      )}
    </>
  );
}
