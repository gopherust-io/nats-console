import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import ArchitectureScorePanel from "../components/ArchitectureScorePanel";
import PageHeader from "../components/ui/PageHeader";
import QueryErrorState from "../components/ui/QueryErrorState";
import {
  askArchitectureScore,
  demoArchitectureScore,
  fetchArchitectureScore,
  type ArchitectureScoreSnapshot,
} from "../lib/architectureScore";
import { fetchAssistantConfig } from "../lib/assistant";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

export default function ArchitectureScorePage() {
  const { t } = useTranslation();
  const { clusterId } = useCluster();
  const [forceSample, setForceSample] = useState(false);
  const [aiReply, setAiReply] = useState<string | null>(null);
  const [asking, setAsking] = useState(false);
  const askGenRef = useRef(0);

  const scoreQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "architecture-score"),
    queryFn: () => fetchArchitectureScore(clusterId!),
    enabled: Boolean(clusterId) && !forceSample,
    staleTime: 5 * 60_000,
    refetchInterval: false,
  });

  const assistantConfigQuery = useQuery({
    queryKey: ["assistant-config"],
    queryFn: fetchAssistantConfig,
    staleTime: 60_000,
  });

  const demo = useMemo(() => demoArchitectureScore(), []);
  const useDemo = forceSample || !clusterId;
  const snapshot: ArchitectureScoreSnapshot = useDemo ? demo : (scoreQuery.data ?? demo);
  const sample = useDemo;
  const aiEnabled = Boolean(assistantConfigQuery.data?.aiEnabled) && Boolean(clusterId) && !sample;

  useEffect(() => {
    askGenRef.current += 1;
    setAiReply(null);
    setAsking(false);
  }, [clusterId, forceSample]);

  const handleAsk = useCallback(async () => {
    if (!clusterId || asking || sample) return;
    const gen = ++askGenRef.current;
    setAsking(true);
    try {
      const result = await askArchitectureScore(clusterId, undefined, { fresh: true });
      if (gen !== askGenRef.current) return;
      setAiReply(result.reply);
    } catch {
      if (gen !== askGenRef.current) return;
      setAiReply(null);
    } finally {
      if (gen === askGenRef.current) setAsking(false);
    }
  }, [asking, clusterId, sample]);

  return (
    <>
      <PageHeader title={t("archScore.title")} subtitle={t("archScore.subtitle")} />
      <div className="live-arch-page__toolbar" role="toolbar" aria-label={t("archScore.title")}>
        {!clusterId && <span className="text-muted">{t("archScore.needSystem")}</span>}
        {clusterId && (
          <button
            type="button"
            aria-pressed={forceSample}
            onClick={() => {
              setForceSample((v) => !v);
              setAiReply(null);
            }}
          >
            {forceSample ? t("archScore.useLive") : t("archScore.useSample")}
          </button>
        )}
      </div>
      {clusterId && scoreQuery.isError && !forceSample && (
        <QueryErrorState
          error={scoreQuery.error}
          onRetry={() => void scoreQuery.refetch()}
          title={t("archScore.loadFailed")}
        />
      )}
      {clusterId && scoreQuery.isLoading && !forceSample && !scoreQuery.data && (
        <div className="skeleton skeleton--panel" />
      )}
      {(forceSample || (!scoreQuery.isError && (!scoreQuery.isLoading || scoreQuery.data)) || !clusterId) && (
        <ArchitectureScorePanel
          snapshot={snapshot}
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
