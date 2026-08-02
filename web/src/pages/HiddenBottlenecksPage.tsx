import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router";
import HiddenBottlenecksPanel from "../components/HiddenBottlenecksPanel";
import PageHeader from "../components/ui/PageHeader";
import QueryErrorState from "../components/ui/QueryErrorState";
import {
  askHiddenBottlenecks,
  demoHiddenBottlenecks,
  fetchHiddenBottlenecks,
  type HiddenBottleneckSnapshot,
} from "../lib/hiddenBottlenecks";
import { fetchAssistantConfig } from "../lib/assistant";
import { useCluster } from "../lib/cluster";
import { MONITORING_POLL_MS } from "../lib/constants";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";

export default function HiddenBottlenecksPage() {
  const { t } = useTranslation();
  const { clusterId } = useCluster();
  const [searchParams] = useSearchParams();
  const filterConsumer = searchParams.get("consumer") ?? undefined;
  const [forceSample, setForceSample] = useState(false);
  const [aiReply, setAiReply] = useState<string | null>(null);
  const [asking, setAsking] = useState(false);
  const askGenRef = useRef(0);

  useEffect(() => {
    askGenRef.current += 1;
    setAiReply(null);
    setAsking(false);
  }, [clusterId, forceSample]);

  const bottlenecksQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "hidden-bottlenecks"),
    queryFn: () => fetchHiddenBottlenecks(clusterId!),
    enabled: Boolean(clusterId) && !forceSample,
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS * 5),
  });

  const assistantConfigQuery = useQuery({
    queryKey: ["assistant-config"],
    queryFn: fetchAssistantConfig,
    staleTime: 60_000,
  });

  const demo = useMemo(() => demoHiddenBottlenecks(), []);
  const useDemo = forceSample || !clusterId || bottlenecksQuery.isError;
  const snapshot: HiddenBottleneckSnapshot = useDemo
    ? demo
    : (bottlenecksQuery.data ?? demo);
  const sample = useDemo;
  const aiEnabled = Boolean(assistantConfigQuery.data?.aiEnabled) && Boolean(clusterId) && !sample;

  const handleAsk = useCallback(async () => {
    if (!clusterId || asking || sample) return;
    const gen = ++askGenRef.current;
    setAsking(true);
    try {
      const result = await askHiddenBottlenecks(clusterId);
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
      <PageHeader title={t("hiddenBottlenecks.title")} subtitle={t("hiddenBottlenecks.subtitle")} />
      <div className="live-arch-page__toolbar" role="toolbar" aria-label={t("hiddenBottlenecks.title")}>
        {!clusterId && <span className="text-muted">{t("hiddenBottlenecks.needSystem")}</span>}
        {clusterId && (
          <button
            type="button"
            aria-pressed={forceSample}
            onClick={() => {
              setForceSample((v) => !v);
              setAiReply(null);
            }}
          >
            {forceSample ? t("hiddenBottlenecks.useLive") : t("hiddenBottlenecks.useSample")}
          </button>
        )}
      </div>
      {clusterId && bottlenecksQuery.isError && !forceSample && (
        <QueryErrorState
          error={bottlenecksQuery.error}
          onRetry={() => void bottlenecksQuery.refetch()}
        />
      )}
      <HiddenBottlenecksPanel
        snapshot={snapshot}
        reply={aiReply}
        asking={asking}
        aiEnabled={aiEnabled}
        onAsk={handleAsk}
        sample={sample}
        filterConsumer={filterConsumer ?? undefined}
      />
    </>
  );
}
