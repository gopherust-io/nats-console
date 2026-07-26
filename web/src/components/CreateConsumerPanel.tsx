import { FormEvent, useEffect, useId, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { StreamConfig } from "../lib/api";
import {
  recommendFiltersLabel,
  recommendedConsumerFromStream,
} from "../lib/consumerRecommend";
import FieldHint from "./ui/FieldHint";
import PanelError from "./ui/PanelError";

export type ConsumerConfigPayload = {
  durableName?: string;
  name?: string;
  description?: string;
  deliverPolicy?: string;
  ackPolicy?: string;
  replayPolicy?: string;
  filterSubject?: string;
  filterSubjects?: string[];
  optStartSeq?: number;
  optStartTime?: string;
  ackWaitNs?: number;
  maxDeliver?: number;
  backoffNs?: number[];
  maxAckPending?: number;
  rateLimitBps?: number;
  sampleFreq?: string;
  maxWaiting?: number;
  inactiveThresholdNs?: number;
  maxRequestBatch?: number;
  maxRequestExpiresNs?: number;
  maxRequestMaxBytes?: number;
  deliverSubject?: string;
  deliverGroup?: string;
  flowControl?: boolean;
  heartbeatNs?: number;
  headersOnly?: boolean;
  replicas?: number;
  memoryStorage?: boolean;
  metadata?: Record<string, string>;
};

type Props = {
  mode: "create" | "edit";
  open: boolean;
  initial?: Partial<ConsumerConfigPayload> | null;
  /** When creating from a stream detail page, used for concrete recommendations. */
  stream?: StreamConfig | null;
  busy?: boolean;
  error?: string;
  onClose: () => void;
  onSubmit: (body: ConsumerConfigPayload) => Promise<void> | void;
};

function isLimited(n: number | undefined | null): boolean {
  return typeof n === "number" && n > 0;
}

function formatDuration(ns: number | undefined, fallback = "30s"): string {
  if (!ns || ns <= 0) return fallback;
  const hour = 3_600_000_000_000;
  const minute = 60_000_000_000;
  const second = 1_000_000_000;
  if (ns % hour === 0) return `${ns / hour}h`;
  if (ns % minute === 0) return `${ns / minute}m`;
  if (ns % second === 0) return `${ns / second}s`;
  return `${ns}ns`;
}

function parseDurationNs(input: string): number | null {
  const s = input.trim();
  if (!s) return null;
  if (/^\d+$/.test(s)) {
    const n = Number(s);
    return Number.isFinite(n) && n > 0 ? n : null;
  }
  const m = /^(\d+(?:\.\d+)?)\s*(ns|us|µs|ms|s|m|h)$/i.exec(s);
  if (!m) return null;
  const n = Number(m[1]);
  if (!Number.isFinite(n) || n <= 0) return null;
  const unit = m[2].toLowerCase();
  const mult =
    unit === "ns"
      ? 1
      : unit === "us" || unit === "µs"
        ? 1_000
        : unit === "ms"
          ? 1_000_000
          : unit === "s"
            ? 1_000_000_000
            : unit === "m"
              ? 60_000_000_000
              : 3_600_000_000_000;
  return Math.round(n * mult);
}

function Switch({
  on,
  label,
  onToggle,
}: {
  on: boolean;
  label: string;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      className={`pref-switch${on ? " pref-switch--on" : ""}`}
      role="switch"
      aria-checked={on}
      aria-label={label}
      onClick={onToggle}
    >
      <span className="pref-switch__track" aria-hidden>
        <span className="pref-switch__thumb" />
      </span>
    </button>
  );
}

export default function CreateConsumerPanel({
  mode,
  open,
  initial,
  stream,
  busy,
  error,
  onClose,
  onSubmit,
}: Props) {
  const { t } = useTranslation();
  const titleId = useId();
  const [durableName, setDurableName] = useState("");
  const [description, setDescription] = useState("");
  const [replicas, setReplicas] = useState(0);
  const [memoryStorage, setMemoryStorage] = useState(false);
  const [deliverPolicy, setDeliverPolicy] = useState("all");
  const [ackPolicy, setAckPolicy] = useState("explicit");
  const [replayPolicy, setReplayPolicy] = useState("instant");
  const [optStartSeq, setOptStartSeq] = useState("");
  const [optStartTime, setOptStartTime] = useState("");
  const [filterSubjects, setFilterSubjects] = useState<string[]>([]);
  const [filterDraft, setFilterDraft] = useState("");
  const [ackWaitOn, setAckWaitOn] = useState(false);
  const [ackWait, setAckWait] = useState("30s");
  const [maxDeliverOn, setMaxDeliverOn] = useState(false);
  const [maxDeliver, setMaxDeliver] = useState("5");
  const [backoff, setBackoff] = useState<string[]>([]);
  const [backoffDraft, setBackoffDraft] = useState("");
  const [maxAckPendingOn, setMaxAckPendingOn] = useState(false);
  const [maxAckPending, setMaxAckPending] = useState("1000");
  const [maxWaitingOn, setMaxWaitingOn] = useState(false);
  const [maxWaiting, setMaxWaiting] = useState("512");
  const [maxBatchOn, setMaxBatchOn] = useState(false);
  const [maxBatch, setMaxBatch] = useState("100");
  const [maxExpiresOn, setMaxExpiresOn] = useState(false);
  const [maxExpires, setMaxExpires] = useState("30s");
  const [maxBytesOn, setMaxBytesOn] = useState(false);
  const [maxBytes, setMaxBytes] = useState("");
  const [deliverSubject, setDeliverSubject] = useState("");
  const [deliverGroup, setDeliverGroup] = useState("");
  const [flowControl, setFlowControl] = useState(false);
  const [heartbeatOn, setHeartbeatOn] = useState(false);
  const [heartbeat, setHeartbeat] = useState("5s");
  const [headersOnly, setHeadersOnly] = useState(false);
  const [rateLimitOn, setRateLimitOn] = useState(false);
  const [rateLimit, setRateLimit] = useState("");
  const [sampleFreq, setSampleFreq] = useState("");
  const [inactiveOn, setInactiveOn] = useState(false);
  const [inactiveThreshold, setInactiveThreshold] = useState("1h");
  const [metadata, setMetadata] = useState<{ key: string; value: string }[]>([]);
  const [metaKey, setMetaKey] = useState("");
  const [metaValue, setMetaValue] = useState("");
  const [localError, setLocalError] = useState("");

  const recommended = useMemo(
    () => (stream ? recommendedConsumerFromStream(stream) : null),
    [stream],
  );

  function applyRecommendedSetup() {
    if (!recommended) return;
    setLocalError("");
    setDurableName(recommended.durableName);
    setDeliverSubject("");
    setDeliverGroup("");
    setFlowControl(false);
    setHeartbeatOn(false);
    setHeadersOnly(false);
    setFilterSubjects(recommended.filterSubjects);
    setFilterDraft("");
    setDeliverPolicy(recommended.deliverPolicy);
    setAckPolicy(recommended.ackPolicy);
    setReplayPolicy(recommended.replayPolicy);
    setOptStartSeq("");
    setOptStartTime("");
    setAckWaitOn(true);
    setAckWait(formatDuration(recommended.ackWaitNs, "30s"));
    setMaxDeliverOn(true);
    setMaxDeliver(String(recommended.maxDeliver));
    if (recommended.maxAckPending && recommended.maxAckPending > 0) {
      setMaxAckPendingOn(true);
      setMaxAckPending(String(recommended.maxAckPending));
    }
    if (recommended.inactiveThresholdNs && recommended.inactiveThresholdNs > 0) {
      setInactiveOn(true);
      setInactiveThreshold(formatDuration(recommended.inactiveThresholdNs, "1h"));
    }
  }

  useEffect(() => {
    if (!open) return;
    const cfg = initial ?? {};
    setDurableName(cfg.durableName || cfg.name || "");
    setDescription(cfg.description ?? "");
    setReplicas(cfg.replicas && cfg.replicas > 0 ? cfg.replicas : 0);
    setMemoryStorage(Boolean(cfg.memoryStorage));
    setDeliverPolicy(cfg.deliverPolicy || "all");
    setAckPolicy(cfg.ackPolicy || "explicit");
    setReplayPolicy(cfg.replayPolicy || "instant");
    setOptStartSeq(cfg.optStartSeq && cfg.optStartSeq > 0 ? String(cfg.optStartSeq) : "");
    setOptStartTime(cfg.optStartTime ?? "");
    const filters = [
      ...(cfg.filterSubjects ?? []),
      ...(cfg.filterSubject && !(cfg.filterSubjects ?? []).includes(cfg.filterSubject)
        ? [cfg.filterSubject]
        : []),
    ];
    setFilterSubjects(filters);
    setFilterDraft("");
    setAckWaitOn(isLimited(cfg.ackWaitNs));
    setAckWait(formatDuration(cfg.ackWaitNs, "30s"));
    setMaxDeliverOn(isLimited(cfg.maxDeliver));
    setMaxDeliver(String(cfg.maxDeliver && cfg.maxDeliver > 0 ? cfg.maxDeliver : 5));
    setBackoff((cfg.backoffNs ?? []).map((ns) => formatDuration(ns, "1s")));
    setBackoffDraft("");
    setMaxAckPendingOn(isLimited(cfg.maxAckPending));
    setMaxAckPending(String(cfg.maxAckPending && cfg.maxAckPending > 0 ? cfg.maxAckPending : 1000));
    setMaxWaitingOn(isLimited(cfg.maxWaiting));
    setMaxWaiting(String(cfg.maxWaiting && cfg.maxWaiting > 0 ? cfg.maxWaiting : 512));
    setMaxBatchOn(isLimited(cfg.maxRequestBatch));
    setMaxBatch(String(cfg.maxRequestBatch && cfg.maxRequestBatch > 0 ? cfg.maxRequestBatch : 100));
    setMaxExpiresOn(isLimited(cfg.maxRequestExpiresNs));
    setMaxExpires(formatDuration(cfg.maxRequestExpiresNs, "30s"));
    setMaxBytesOn(isLimited(cfg.maxRequestMaxBytes));
    setMaxBytes(cfg.maxRequestMaxBytes && cfg.maxRequestMaxBytes > 0 ? String(cfg.maxRequestMaxBytes) : "");
    setDeliverSubject(cfg.deliverSubject ?? "");
    setDeliverGroup(cfg.deliverGroup ?? "");
    setFlowControl(Boolean(cfg.flowControl));
    setHeartbeatOn(isLimited(cfg.heartbeatNs));
    setHeartbeat(formatDuration(cfg.heartbeatNs, "5s"));
    setHeadersOnly(Boolean(cfg.headersOnly));
    setRateLimitOn(isLimited(cfg.rateLimitBps));
    setRateLimit(cfg.rateLimitBps && cfg.rateLimitBps > 0 ? String(cfg.rateLimitBps) : "");
    setSampleFreq(cfg.sampleFreq ?? "");
    setInactiveOn(isLimited(cfg.inactiveThresholdNs));
    setInactiveThreshold(formatDuration(cfg.inactiveThresholdNs, "1h"));
    setMetadata(Object.entries(cfg.metadata ?? {}).map(([key, value]) => ({ key, value: String(value) })));
    setMetaKey("");
    setMetaValue("");
    setLocalError("");
  }, [open, initial]);

  if (!open) return null;

  function addFilter() {
    const next = filterDraft.trim();
    if (!next || filterSubjects.includes(next)) return;
    setFilterSubjects((prev) => [...prev, next]);
    setFilterDraft("");
  }

  function addBackoff() {
    const next = backoffDraft.trim();
    if (!next || parseDurationNs(next) == null) return;
    setBackoff((prev) => [...prev, next]);
    setBackoffDraft("");
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setLocalError("");
    if (!durableName.trim()) {
      setLocalError(t("consumerConfig.nameRequired"));
      return;
    }

    const body: ConsumerConfigPayload = {
      durableName: durableName.trim(),
      description: description.trim() || undefined,
      deliverPolicy,
      ackPolicy,
      replayPolicy,
      memoryStorage: memoryStorage || undefined,
      flowControl: flowControl || undefined,
      headersOnly: headersOnly || undefined,
    };
    if (replicas > 0) body.replicas = replicas;

    if (deliverPolicy === "by_start_sequence") {
      const n = Number(optStartSeq);
      if (!Number.isFinite(n) || n < 1) {
        setLocalError(t("consumerConfig.optStartSeqRequired"));
        return;
      }
      body.optStartSeq = Math.floor(n);
    }
    if (deliverPolicy === "by_start_time") {
      if (!optStartTime.trim()) {
        setLocalError(t("consumerConfig.optStartTimeRequired"));
        return;
      }
      const iso = new Date(optStartTime).toISOString();
      if (Number.isNaN(Date.parse(iso))) {
        setLocalError(t("consumerConfig.invalidOptStartTime"));
        return;
      }
      body.optStartTime = iso;
    }
    if (filterSubjects.length === 1) {
      body.filterSubject = filterSubjects[0];
    } else if (filterSubjects.length > 1) {
      body.filterSubjects = filterSubjects;
    }

    if (ackWaitOn) {
      const ns = parseDurationNs(ackWait);
      if (ns == null) {
        setLocalError(t("consumerConfig.invalidAckWait"));
        return;
      }
      body.ackWaitNs = ns;
    }
    if (maxDeliverOn) {
      const n = Number(maxDeliver);
      if (!Number.isFinite(n) || n < 1) {
        setLocalError(t("consumerConfig.invalidMaxDeliver"));
        return;
      }
      body.maxDeliver = Math.floor(n);
    }
    if (backoff.length > 0) {
      const parsed: number[] = [];
      for (const item of backoff) {
        const ns = parseDurationNs(item);
        if (ns == null) {
          setLocalError(t("consumerConfig.invalidBackoff"));
          return;
        }
        parsed.push(ns);
      }
      body.backoffNs = parsed;
    }
    if (maxAckPendingOn) {
      const n = Number(maxAckPending);
      if (!Number.isFinite(n) || n < 1) {
        setLocalError(t("consumerConfig.invalidMaxAckPending"));
        return;
      }
      body.maxAckPending = Math.floor(n);
    }
    if (maxWaitingOn) {
      const n = Number(maxWaiting);
      if (!Number.isFinite(n) || n < 1) {
        setLocalError(t("consumerConfig.invalidMaxWaiting"));
        return;
      }
      body.maxWaiting = Math.floor(n);
    }
    if (maxBatchOn) {
      const n = Number(maxBatch);
      if (!Number.isFinite(n) || n < 1) {
        setLocalError(t("consumerConfig.invalidMaxBatch"));
        return;
      }
      body.maxRequestBatch = Math.floor(n);
    }
    if (maxExpiresOn) {
      const ns = parseDurationNs(maxExpires);
      if (ns == null) {
        setLocalError(t("consumerConfig.invalidMaxExpires"));
        return;
      }
      body.maxRequestExpiresNs = ns;
    }
    if (maxBytesOn) {
      const n = Number(maxBytes);
      if (!Number.isFinite(n) || n < 1) {
        setLocalError(t("consumerConfig.invalidMaxBytes"));
        return;
      }
      body.maxRequestMaxBytes = Math.floor(n);
    }
    if (deliverSubject.trim()) body.deliverSubject = deliverSubject.trim();
    if (deliverGroup.trim()) body.deliverGroup = deliverGroup.trim();
    if (heartbeatOn) {
      const ns = parseDurationNs(heartbeat);
      if (ns == null) {
        setLocalError(t("consumerConfig.invalidHeartbeat"));
        return;
      }
      body.heartbeatNs = ns;
    }
    if (rateLimitOn) {
      const n = Number(rateLimit);
      if (!Number.isFinite(n) || n < 1) {
        setLocalError(t("consumerConfig.invalidRateLimit"));
        return;
      }
      body.rateLimitBps = Math.floor(n);
    }
    if (sampleFreq.trim()) body.sampleFreq = sampleFreq.trim();
    if (inactiveOn) {
      const ns = parseDurationNs(inactiveThreshold);
      if (ns == null) {
        setLocalError(t("consumerConfig.invalidInactiveThreshold"));
        return;
      }
      body.inactiveThresholdNs = ns;
    }
    if (metadata.length > 0) {
      body.metadata = Object.fromEntries(metadata.map((row) => [row.key, row.value]));
    }

    try {
      await onSubmit(body);
    } catch {
      // Parent surfaces API error.
    }
  }

  const displayError = localError || error;

  return (
    <div className="stream-config-overlay" role="presentation" onClick={onClose}>
      <aside
        className="stream-config-panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        onClick={(e) => e.stopPropagation()}
      >
        <header className="stream-config-panel__header">
          <h2 id={titleId}>
            {mode === "edit" ? t("consumerConfig.editTitle") : t("consumerConfig.createTitle")}
          </h2>
          <button type="button" className="btn secondary" onClick={onClose} aria-label={t("common.close")}>
            {t("common.close")}
          </button>
        </header>

        <form className="stream-config-panel__body" onSubmit={handleSubmit}>
          <aside
            className="stream-config-recommend"
            aria-label={
              recommended
                ? t("consumerConfig.recommendTitleStream", { stream: stream?.name ?? "" })
                : t("consumerConfig.recommendTitle")
            }
          >
            {recommended ? (
              <>
                <strong>
                  {t("consumerConfig.recommendTitleStream", { stream: stream?.name ?? recommended.durableName })}
                </strong>
                <p>
                  {t("consumerConfig.recommendBodyStream", {
                    durable: recommended.durableName,
                    ack: recommended.ackPolicy,
                    filters:
                      recommended.filterSubjects.length > 0
                        ? t("consumerConfig.recommendFiltersClause", {
                            filters: recommendFiltersLabel(recommended.filterSubjects),
                          })
                        : t("consumerConfig.recommendNoFiltersClause"),
                  })}
                </p>
                {mode === "create" && (
                  <button
                    type="button"
                    className="btn secondary"
                    style={{ marginTop: 10 }}
                    onClick={applyRecommendedSetup}
                    disabled={busy}
                  >
                    {t("consumerConfig.applyRecommended")}
                  </button>
                )}
              </>
            ) : (
              <>
                <strong>{t("consumerConfig.recommendTitle")}</strong>
                <p>{t("consumerConfig.recommendBody")}</p>
              </>
            )}
          </aside>

          <section className="stream-config-section">
            <h3>{t("consumerConfig.basicInfo")}</h3>
            <p>{t("consumerConfig.basicInfoLead")}</p>
            <div className="nc-form-row">
              <label htmlFor="cons-cfg-name">{t("consumerConfig.durableName")}</label>
              <input
                id="cons-cfg-name"
                required
                disabled={mode === "edit"}
                value={durableName}
                onChange={(e) => setDurableName(e.target.value)}
                placeholder={t("consumerConfig.durableNamePlaceholder")}
                aria-describedby="cons-cfg-name-hint"
              />
              <FieldHint id="cons-cfg-name-hint">{t("consumerConfig.durableNameHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="cons-cfg-desc">{t("common.description")}</label>
              <input
                id="cons-cfg-desc"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder={t("consumerConfig.descriptionPlaceholder")}
              />
              <FieldHint>{t("consumerConfig.descriptionHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="cons-cfg-replicas">{t("consumerConfig.replicas")}</label>
              <select
                id="cons-cfg-replicas"
                value={replicas}
                onChange={(e) => setReplicas(Number(e.target.value) || 0)}
              >
                <option value={0}>{t("consumerConfig.replicasInherit")}</option>
                <option value={1}>1</option>
                <option value={3}>3</option>
                <option value={5}>5</option>
              </select>
              <FieldHint>{t("consumerConfig.replicasHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("consumerConfig.memoryStorage")}</span>
                <Switch
                  on={memoryStorage}
                  label={t("consumerConfig.memoryStorage")}
                  onToggle={() => setMemoryStorage((v) => !v)}
                />
              </div>
              <FieldHint>{t("consumerConfig.memoryStorageHint")}</FieldHint>
            </div>
          </section>

          <section className="stream-config-section">
            <h3>{t("consumerConfig.delivery")}</h3>
            <p>{t("consumerConfig.deliveryLead")}</p>
            <div className="nc-form-row">
              <label htmlFor="cons-cfg-deliver">{t("consumerConfig.deliverPolicy")}</label>
              <select
                id="cons-cfg-deliver"
                value={deliverPolicy}
                onChange={(e) => setDeliverPolicy(e.target.value)}
              >
                <option value="all">all</option>
                <option value="last">last</option>
                <option value="new">new</option>
                <option value="by_start_sequence">by_start_sequence</option>
                <option value="by_start_time">by_start_time</option>
                <option value="last_per_subject">last_per_subject</option>
              </select>
              <FieldHint>{t("consumerConfig.deliverPolicyHint")}</FieldHint>
            </div>
            {deliverPolicy === "by_start_sequence" && (
              <div className="nc-form-row">
                <label htmlFor="cons-cfg-start-seq">{t("consumerConfig.optStartSeq")}</label>
                <input
                  id="cons-cfg-start-seq"
                  type="number"
                  min={1}
                  required
                  value={optStartSeq}
                  onChange={(e) => setOptStartSeq(e.target.value)}
                />
              </div>
            )}
            {deliverPolicy === "by_start_time" && (
              <div className="nc-form-row">
                <label htmlFor="cons-cfg-start-time">{t("consumerConfig.optStartTime")}</label>
                <input
                  id="cons-cfg-start-time"
                  type="datetime-local"
                  required
                  value={optStartTime.includes("T") && !optStartTime.endsWith("Z") ? optStartTime.slice(0, 16) : optStartTime}
                  onChange={(e) => setOptStartTime(e.target.value)}
                />
              </div>
            )}
            <div className="nc-form-row">
              <label htmlFor="cons-cfg-ack">{t("consumerConfig.ackPolicy")}</label>
              <select id="cons-cfg-ack" value={ackPolicy} onChange={(e) => setAckPolicy(e.target.value)}>
                <option value="explicit">explicit</option>
                <option value="none">none</option>
                <option value="all">all</option>
              </select>
              <FieldHint>{t("consumerConfig.ackPolicyHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="cons-cfg-replay">{t("consumerConfig.replayPolicy")}</label>
              <select
                id="cons-cfg-replay"
                value={replayPolicy}
                onChange={(e) => setReplayPolicy(e.target.value)}
              >
                <option value="instant">instant</option>
                <option value="original">original</option>
              </select>
              <FieldHint>{t("consumerConfig.replayPolicyHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <label className="stream-config-mini-label">{t("consumerConfig.filterSubjects")}</label>
              <div className="stream-config-add-row">
                <input
                  value={filterDraft}
                  onChange={(e) => setFilterDraft(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      addFilter();
                    }
                  }}
                  placeholder="orders.*"
                />
                <button type="button" className="btn secondary" onClick={addFilter}>
                  {t("streamConfig.add")}
                </button>
              </div>
              <FieldHint>{t("consumerConfig.filterSubjectsHint")}</FieldHint>
              {filterSubjects.length > 0 && (
                <ul className="stream-config-chips">
                  {filterSubjects.map((s) => (
                    <li key={s}>
                      <span className="mono">{s}</span>
                      <button
                        type="button"
                        className="stream-config-chip-remove"
                        aria-label={t("common.delete")}
                        onClick={() => setFilterSubjects((prev) => prev.filter((x) => x !== s))}
                      >
                        ×
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </section>

          <section className="stream-config-section">
            <h3>{t("consumerConfig.ackRedelivery")}</h3>
            <p>{t("consumerConfig.ackRedeliveryLead")}</p>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("consumerConfig.ackWait")}</span>
                <Switch on={ackWaitOn} label={t("consumerConfig.ackWait")} onToggle={() => setAckWaitOn((v) => !v)} />
              </div>
              {ackWaitOn && <input value={ackWait} onChange={(e) => setAckWait(e.target.value)} placeholder="30s" />}
              <FieldHint>{t("consumerConfig.ackWaitHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("consumerConfig.maxDeliver")}</span>
                <Switch
                  on={maxDeliverOn}
                  label={t("consumerConfig.maxDeliver")}
                  onToggle={() => setMaxDeliverOn((v) => !v)}
                />
              </div>
              {maxDeliverOn && (
                <input type="number" min={1} value={maxDeliver} onChange={(e) => setMaxDeliver(e.target.value)} />
              )}
              <FieldHint>{t("consumerConfig.maxDeliverHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <label className="stream-config-mini-label">{t("consumerConfig.backoff")}</label>
              <div className="stream-config-add-row">
                <input
                  value={backoffDraft}
                  onChange={(e) => setBackoffDraft(e.target.value)}
                  placeholder="1s"
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      addBackoff();
                    }
                  }}
                />
                <button type="button" className="btn secondary" onClick={addBackoff}>
                  {t("streamConfig.add")}
                </button>
              </div>
              <FieldHint>{t("consumerConfig.backoffHint")}</FieldHint>
              {backoff.length > 0 && (
                <ul className="stream-config-chips">
                  {backoff.map((item, idx) => (
                    <li key={`${item}-${idx}`}>
                      <span className="mono">{item}</span>
                      <button
                        type="button"
                        className="stream-config-chip-remove"
                        aria-label={t("common.delete")}
                        onClick={() => setBackoff((prev) => prev.filter((_, i) => i !== idx))}
                      >
                        ×
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("consumerConfig.maxAckPending")}</span>
                <Switch
                  on={maxAckPendingOn}
                  label={t("consumerConfig.maxAckPending")}
                  onToggle={() => setMaxAckPendingOn((v) => !v)}
                />
              </div>
              {maxAckPendingOn && (
                <input
                  type="number"
                  min={1}
                  value={maxAckPending}
                  onChange={(e) => setMaxAckPending(e.target.value)}
                />
              )}
              <FieldHint>{t("consumerConfig.maxAckPendingHint")}</FieldHint>
            </div>
          </section>

          <section className="stream-config-section">
            <h3>{t("consumerConfig.pull")}</h3>
            <p>{t("consumerConfig.pullLead")}</p>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("consumerConfig.maxWaiting")}</span>
                <Switch
                  on={maxWaitingOn}
                  label={t("consumerConfig.maxWaiting")}
                  onToggle={() => setMaxWaitingOn((v) => !v)}
                />
              </div>
              {maxWaitingOn && (
                <input type="number" min={1} value={maxWaiting} onChange={(e) => setMaxWaiting(e.target.value)} />
              )}
              <FieldHint>{t("consumerConfig.maxWaitingHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("consumerConfig.maxRequestBatch")}</span>
                <Switch on={maxBatchOn} label={t("consumerConfig.maxRequestBatch")} onToggle={() => setMaxBatchOn((v) => !v)} />
              </div>
              {maxBatchOn && (
                <input type="number" min={1} value={maxBatch} onChange={(e) => setMaxBatch(e.target.value)} />
              )}
              <FieldHint>{t("consumerConfig.maxRequestBatchHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("consumerConfig.maxRequestExpires")}</span>
                <Switch
                  on={maxExpiresOn}
                  label={t("consumerConfig.maxRequestExpires")}
                  onToggle={() => setMaxExpiresOn((v) => !v)}
                />
              </div>
              {maxExpiresOn && (
                <input value={maxExpires} onChange={(e) => setMaxExpires(e.target.value)} placeholder="30s" />
              )}
              <FieldHint>{t("consumerConfig.maxRequestExpiresHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("consumerConfig.maxRequestMaxBytes")}</span>
                <Switch
                  on={maxBytesOn}
                  label={t("consumerConfig.maxRequestMaxBytes")}
                  onToggle={() => setMaxBytesOn((v) => !v)}
                />
              </div>
              {maxBytesOn && (
                <input type="number" min={1} value={maxBytes} onChange={(e) => setMaxBytes(e.target.value)} />
              )}
              <FieldHint>{t("consumerConfig.maxRequestMaxBytesHint")}</FieldHint>
            </div>
          </section>

          <section className="stream-config-section">
            <h3>{t("consumerConfig.push")}</h3>
            <p>{t("consumerConfig.pushLead")}</p>
            <div className="nc-form-row">
              <label htmlFor="cons-cfg-deliver-subj">{t("consumerConfig.deliverSubject")}</label>
              <input
                id="cons-cfg-deliver-subj"
                value={deliverSubject}
                onChange={(e) => setDeliverSubject(e.target.value)}
                placeholder="_INBOX.orders"
              />
              <FieldHint>{t("consumerConfig.deliverSubjectHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="cons-cfg-deliver-group">{t("consumerConfig.deliverGroup")}</label>
              <input
                id="cons-cfg-deliver-group"
                value={deliverGroup}
                onChange={(e) => setDeliverGroup(e.target.value)}
                placeholder="workers"
              />
              <FieldHint>{t("consumerConfig.deliverGroupHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("consumerConfig.flowControl")}</span>
                <Switch
                  on={flowControl}
                  label={t("consumerConfig.flowControl")}
                  onToggle={() => setFlowControl((v) => !v)}
                />
              </div>
              <FieldHint>{t("consumerConfig.flowControlHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("consumerConfig.heartbeat")}</span>
                <Switch
                  on={heartbeatOn}
                  label={t("consumerConfig.heartbeat")}
                  onToggle={() => setHeartbeatOn((v) => !v)}
                />
              </div>
              {heartbeatOn && (
                <input value={heartbeat} onChange={(e) => setHeartbeat(e.target.value)} placeholder="5s" />
              )}
              <FieldHint>{t("consumerConfig.heartbeatHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("consumerConfig.headersOnly")}</span>
                <Switch
                  on={headersOnly}
                  label={t("consumerConfig.headersOnly")}
                  onToggle={() => setHeadersOnly((v) => !v)}
                />
              </div>
              <FieldHint>{t("consumerConfig.headersOnlyHint")}</FieldHint>
            </div>
          </section>

          <section className="stream-config-section">
            <h3>{t("consumerConfig.advanced")}</h3>
            <p>{t("consumerConfig.advancedLead")}</p>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("consumerConfig.rateLimit")}</span>
                <Switch
                  on={rateLimitOn}
                  label={t("consumerConfig.rateLimit")}
                  onToggle={() => setRateLimitOn((v) => !v)}
                />
              </div>
              {rateLimitOn && (
                <input type="number" min={1} value={rateLimit} onChange={(e) => setRateLimit(e.target.value)} />
              )}
              <FieldHint>{t("consumerConfig.rateLimitHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="cons-cfg-sample">{t("consumerConfig.sampleFreq")}</label>
              <input
                id="cons-cfg-sample"
                value={sampleFreq}
                onChange={(e) => setSampleFreq(e.target.value)}
                placeholder="100"
              />
              <FieldHint>{t("consumerConfig.sampleFreqHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("consumerConfig.inactiveThreshold")}</span>
                <Switch
                  on={inactiveOn}
                  label={t("consumerConfig.inactiveThreshold")}
                  onToggle={() => setInactiveOn((v) => !v)}
                />
              </div>
              {inactiveOn && (
                <input
                  value={inactiveThreshold}
                  onChange={(e) => setInactiveThreshold(e.target.value)}
                  placeholder="1h"
                />
              )}
              <FieldHint>{t("consumerConfig.inactiveThresholdHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <label className="stream-config-mini-label">{t("consumerConfig.metadata")}</label>
              <div className="stream-config-add-row">
                <input value={metaKey} onChange={(e) => setMetaKey(e.target.value)} placeholder="key" />
                <input value={metaValue} onChange={(e) => setMetaValue(e.target.value)} placeholder="value" />
                <button
                  type="button"
                  className="btn secondary"
                  onClick={() => {
                    const key = metaKey.trim();
                    if (!key) return;
                    setMetadata((prev) => {
                      const next = prev.filter((row) => row.key !== key);
                      next.push({ key, value: metaValue });
                      return next;
                    });
                    setMetaKey("");
                    setMetaValue("");
                  }}
                >
                  {t("streamConfig.add")}
                </button>
              </div>
              <FieldHint>{t("consumerConfig.metadataHint")}</FieldHint>
              {metadata.length > 0 && (
                <ul className="stream-config-chips">
                  {metadata.map((row) => (
                    <li key={row.key}>
                      <span className="mono">
                        {row.key}={row.value}
                      </span>
                      <button
                        type="button"
                        className="stream-config-chip-remove"
                        aria-label={t("common.delete")}
                        onClick={() => setMetadata((prev) => prev.filter((r) => r.key !== row.key))}
                      >
                        ×
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </section>

          <footer className="stream-config-panel__footer">
            <PanelError message={displayError} />
            <div className="stream-config-panel__footer-actions">
              <button type="button" className="btn secondary" onClick={onClose} disabled={busy}>
                {t("common.cancel")}
              </button>
              <button type="submit" className="btn" disabled={busy}>
                {t("common.save")}
              </button>
            </div>
          </footer>
        </form>
      </aside>
    </div>
  );
}
