import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import ArchitectureReviewPanel from "../components/ArchitectureReviewPanel";
import PageHeader from "../components/ui/PageHeader";
import QueryErrorState from "../components/ui/QueryErrorState";
import {
  askArchitectureReview,
  demoArchitectureReview,
  fetchArchitectureReview,
  type ArchitectureReviewSnapshot,
} from "../lib/architectureReview";
import { demoArchitectureScore, fetchArchitectureScore } from "../lib/architectureScore";
import { fetchAssistantConfig } from "../lib/assistant";
import { useCluster } from "../lib/cluster";
import { MONITORING_POLL_MS } from "../lib/constants";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";

export default function ArchitectureReviewPage() {
  const { t } = useTranslation();
  const { clusterId } = useCluster();
  const [forceSample, setForceSample] = useState(false);
  const [aiReply, setAiReply] = useState<string | null>(null);
  const [asking, setAsking] = useState(false);
  const askGenRef = useRef(0);

  useEffect(() => {
    askGenRef.current += 1;
    setAiReply(null);
    setAsking(false);
  }, [clusterId, forceSample]);

  const reviewQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "architecture-review"),
    queryFn: () => fetchArchitectureReview(clusterId!, { fresh: true }),
    enabled: Boolean(clusterId) && !forceSample,
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS),
  });

  const scoreQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "architecture-score"),
    queryFn: () => fetchArchitectureScore(clusterId!, { fresh: true }),
    enabled: Boolean(clusterId) && !forceSample,
    staleTime: 60_000,
  });

  const assistantConfigQuery = useQuery({
    queryKey: ["assistant-config"],
    queryFn: fetchAssistantConfig,
    staleTime: 60_000,
  });

  const demo = useMemo(() => demoArchitectureReview(), []);
  const demoScore = useMemo(() => demoArchitectureScore(), []);
  const useDemo = forceSample || !clusterId || reviewQuery.isError;
  const snapshot: ArchitectureReviewSnapshot = useDemo
    ? demo
    : (reviewQuery.data ?? demo);
  const sample = useDemo;
  const scoreValue = sample
    ? demoScore.score
    : (scoreQuery.data?.score ?? null);
  const maxScore = sample
    ? demoScore.maxScore
    : (scoreQuery.data?.maxScore ?? 100);
  const aiEnabled = Boolean(assistantConfigQuery.data?.aiEnabled) && Boolean(clusterId) && !sample;

  const handleAsk = useCallback(async () => {
    if (!clusterId || asking || sample) return;
    const gen = ++askGenRef.current;
    setAsking(true);
    try {
      const result = await askArchitectureReview(clusterId, undefined, { fresh: true });
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
    <div className="arch-page">
      <PageHeader title={t("archReview.title")} subtitle={t("archReview.subtitle")} />
      <div className="live-arch-page__toolbar" role="toolbar" aria-label={t("archReview.title")}>
        {!clusterId && <span className="text-muted">{t("archReview.needSystem")}</span>}
        {clusterId && (
          <button
            type="button"
            aria-pressed={forceSample}
            onClick={() => {
              setForceSample((v) => !v);
              setAiReply(null);
            }}
          >
            {forceSample ? t("archReview.useLive") : t("archReview.useSample")}
          </button>
        )}
      </div>
      {clusterId && reviewQuery.isError && !forceSample && (
        <QueryErrorState
          error={reviewQuery.error}
          onRetry={() => void reviewQuery.refetch()}
        />
      )}
      {clusterId && reviewQuery.isLoading && !forceSample && !reviewQuery.data && (
        <div className="skeleton skeleton--panel" />
      )}
      {(forceSample || !reviewQuery.isLoading || reviewQuery.data || !clusterId) && (
        <ArchitectureReviewPanel
          snapshot={snapshot}
          reply={aiReply}
          asking={asking}
          aiEnabled={aiEnabled}
          onAsk={sample ? undefined : () => void handleAsk()}
          sample={sample}
          score={scoreValue}
          maxScore={maxScore}
        />
      )}
    </div>
  );
}
