import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import ChaosStoryPanel from "../components/ChaosStoryPanel";
import PageHeader from "../components/ui/PageHeader";
import QueryErrorState from "../components/ui/QueryErrorState";
import { fetchAssistantConfig } from "../lib/assistant";
import {
  actDurationMs,
  demoChaosStory,
  demoChaosStorySeed,
  fetchChaosStorySeed,
  generateChaosStory,
  nextChaosActIndex,
  type ChaosStory,
  type ChaosStorySeed,
} from "../lib/chaosStory";
import { useCluster } from "../lib/cluster";
import { MONITORING_POLL_MS } from "../lib/constants";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";

export default function ChaosStoryPage() {
  const { t } = useTranslation();
  const { clusterId } = useCluster();
  const [forceSample, setForceSample] = useState(false);
  const [story, setStory] = useState<ChaosStory>(() => demoChaosStory());
  const [seed, setSeed] = useState<ChaosStorySeed>(() => demoChaosStorySeed());
  const [actIndex, setActIndex] = useState(0);
  const [simulating, setSimulating] = useState(false);
  const [paused, setPaused] = useState(false);
  const [generating, setGenerating] = useState(false);
  const [genError, setGenError] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const seedQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "chaos-story-seed"),
    queryFn: () => fetchChaosStorySeed(clusterId!, { fresh: true }),
    enabled: Boolean(clusterId) && !forceSample,
    refetchInterval: visibilityAwareInterval(MONITORING_POLL_MS * 5),
  });

  const assistantConfigQuery = useQuery({
    queryKey: ["assistant-config"],
    queryFn: fetchAssistantConfig,
    staleTime: 60_000,
  });

  const useDemo = forceSample || !clusterId;
  const sample = useDemo;
  const aiEnabled = Boolean(assistantConfigQuery.data?.aiEnabled) && Boolean(clusterId) && !forceSample;

  useEffect(() => {
    setStory(demoChaosStory());
    setSeed(demoChaosStorySeed());
    setActIndex(0);
    setSimulating(false);
    setPaused(false);
    setGenError(null);
  }, [clusterId]);

  useEffect(() => {
    if (useDemo) {
      setStory(demoChaosStory());
      setSeed(demoChaosStorySeed());
      return;
    }
    if (seedQuery.data) {
      setSeed(seedQuery.data.seed);
      if (seedQuery.data.story) setStory(seedQuery.data.story);
    }
  }, [useDemo, seedQuery.data]);

  const clearTimer = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const resetPlaybook = useCallback(() => {
    clearTimer();
    setActIndex(0);
    setSimulating(false);
    setPaused(false);
  }, [clearTimer]);

  useEffect(() => () => clearTimer(), [clearTimer]);

  useEffect(() => {
    resetPlaybook();
  }, [story.title, story.summary, resetPlaybook]);

  const scheduleNextRef = useRef<(fromIndex: number) => void>(() => {});
  const scheduleNext = useCallback(
    (fromIndex: number) => {
      clearTimer();
      const act = story.acts[fromIndex];
      timerRef.current = setTimeout(() => {
        const { next, done } = nextChaosActIndex(fromIndex, story.acts.length);
        setActIndex(next);
        if (done) {
          setSimulating(false);
          setPaused(false);
          return;
        }
        scheduleNextRef.current(next);
      }, actDurationMs(act));
    },
    [clearTimer, story.acts],
  );
  scheduleNextRef.current = scheduleNext;

  const handleSimulate = useCallback(() => {
    if (story.acts.length === 0) return;
    if (paused) {
      setPaused(false);
      setSimulating(true);
      scheduleNext(actIndex);
      return;
    }
    setActIndex(0);
    setSimulating(true);
    setPaused(false);
    scheduleNext(0);
  }, [actIndex, paused, scheduleNext, story.acts.length]);

  const handlePause = useCallback(() => {
    clearTimer();
    setPaused(true);
  }, [clearTimer]);

  const handleGenerate = useCallback(async () => {
    if (!clusterId || generating || forceSample) return;
    setGenerating(true);
    setGenError(null);
    resetPlaybook();
    try {
      const result = await generateChaosStory(clusterId, undefined, { fresh: true });
      setStory(result.story);
      setSeed(result.seed);
    } catch (e) {
      setGenError(e instanceof Error ? e.message : t("chaosStory.generateFailed"));
    } finally {
      setGenerating(false);
    }
  }, [clusterId, forceSample, generating, resetPlaybook, t]);

  const liveSeed = useMemo(() => (useDemo ? demoChaosStorySeed() : seed), [seed, useDemo]);

  return (
    <>
      <PageHeader title={t("chaosStory.title")} subtitle={t("chaosStory.subtitle")} />
      <div className="live-arch-page__toolbar" role="toolbar" aria-label={t("chaosStory.title")}>
        {!clusterId && <span className="text-muted">{t("chaosStory.needSystem")}</span>}
        {clusterId && (
          <button
            type="button"
            aria-pressed={forceSample}
            onClick={() => {
              setForceSample((v) => !v);
              setGenError(null);
              resetPlaybook();
            }}
          >
            {forceSample ? t("chaosStory.useLive") : t("chaosStory.useSample")}
          </button>
        )}
      </div>
      {clusterId && seedQuery.isError && !forceSample && (
        <QueryErrorState error={seedQuery.error} onRetry={() => void seedQuery.refetch()} />
      )}
      <ChaosStoryPanel
        story={story}
        seed={liveSeed}
        sample={sample}
        actIndex={actIndex}
        simulating={simulating}
        paused={paused}
        generating={generating}
        aiEnabled={aiEnabled}
        onGenerate={forceSample || !clusterId ? undefined : () => void handleGenerate()}
        onSimulate={handleSimulate}
        onPause={handlePause}
        onReset={resetPlaybook}
        error={genError}
      />
    </>
  );
}
