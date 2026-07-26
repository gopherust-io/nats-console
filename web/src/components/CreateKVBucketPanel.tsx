import { FormEvent, useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { KVBucketInfo } from "../lib/api";
import FieldHint from "./ui/FieldHint";
import PanelError from "./ui/PanelError";

type ByteUnit = "B" | "KiB" | "MiB" | "GiB" | "TiB";

const BYTE_UNIT_MULT: Record<ByteUnit, number> = {
  B: 1,
  KiB: 1024,
  MiB: 1024 ** 2,
  GiB: 1024 ** 3,
  TiB: 1024 ** 4,
};

export type KVBucketConfigPayload = {
  bucket: string;
  description?: string;
  storage?: string;
  history?: number;
  ttlNs?: number;
  limitMarkerTTLNs?: number;
  maxValueSize?: number;
  maxBytes?: number;
  replicas?: number;
  compression?: boolean;
  placement?: { cluster?: string; tags?: string[] };
  republish?: { src?: string; dest: string; headersOnly?: boolean };
  mirror?: { name: string; filterSubject?: string };
  sources?: { name: string; filterSubject?: string }[];
  metadata?: Record<string, string>;
};

type Props = {
  mode: "create" | "edit";
  open: boolean;
  initial?: Partial<KVBucketInfo> | null;
  busy?: boolean;
  error?: string;
  onClose: () => void;
  onSubmit: (body: KVBucketConfigPayload) => Promise<void> | void;
};

function isLimited(n: number | undefined | null): boolean {
  return typeof n === "number" && n > 0;
}

function pickByteUnit(bytes: number): { value: string; unit: ByteUnit } {
  if (!bytes || bytes <= 0) return { value: "1", unit: "GiB" };
  const units: ByteUnit[] = ["TiB", "GiB", "MiB", "KiB", "B"];
  for (const unit of units) {
    const mult = BYTE_UNIT_MULT[unit];
    if (bytes % mult === 0 && bytes / mult >= 1) {
      return { value: String(bytes / mult), unit };
    }
  }
  return { value: String(bytes), unit: "B" };
}

function formatDuration(ns: number | undefined, fallback = "1h"): string {
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

export default function CreateKVBucketPanel({
  mode,
  open,
  initial,
  busy,
  error,
  onClose,
  onSubmit,
}: Props) {
  const { t } = useTranslation();
  const titleId = useId();
  const [bucket, setBucket] = useState("");
  const [description, setDescription] = useState("");
  const [replicas, setReplicas] = useState(1);
  const [storage, setStorage] = useState("file");
  const [history, setHistory] = useState("1");
  const [ttlOn, setTtlOn] = useState(false);
  const [ttl, setTtl] = useState("1h");
  const [maxValueOn, setMaxValueOn] = useState(false);
  const [maxValueValue, setMaxValueValue] = useState("1");
  const [maxValueUnit, setMaxValueUnit] = useState<ByteUnit>("MiB");
  const [maxBytesOn, setMaxBytesOn] = useState(false);
  const [maxBytesValue, setMaxBytesValue] = useState("1");
  const [maxBytesUnit, setMaxBytesUnit] = useState<ByteUnit>("GiB");
  const [clusterOn, setClusterOn] = useState(false);
  const [cluster, setCluster] = useState("");
  const [tagsOn, setTagsOn] = useState(false);
  const [tags, setTags] = useState<string[]>([]);
  const [tagDraft, setTagDraft] = useState("");
  const [compression, setCompression] = useState(false);
  const [limitMarkerOn, setLimitMarkerOn] = useState(false);
  const [limitMarkerTTL, setLimitMarkerTTL] = useState("1s");
  const [republishOn, setRepublishOn] = useState(false);
  const [republishSrc, setRepublishSrc] = useState("");
  const [republishDest, setRepublishDest] = useState("");
  const [republishHeadersOnly, setRepublishHeadersOnly] = useState(false);
  const [mirrorOn, setMirrorOn] = useState(false);
  const [mirrorName, setMirrorName] = useState("");
  const [mirrorFilter, setMirrorFilter] = useState("");
  const [sources, setSources] = useState<{ name: string; filterSubject?: string }[]>([]);
  const [sourceName, setSourceName] = useState("");
  const [sourceFilter, setSourceFilter] = useState("");
  const [metadata, setMetadata] = useState<{ key: string; value: string }[]>([]);
  const [metaKey, setMetaKey] = useState("");
  const [metaValue, setMetaValue] = useState("");
  const [localError, setLocalError] = useState("");

  useEffect(() => {
    if (!open) return;
    const cfg = initial ?? {};
    setBucket(cfg.bucket ?? "");
    setDescription(cfg.description ?? "");
    const r = cfg.replicas && cfg.replicas > 0 ? cfg.replicas : 1;
    setReplicas(r === 2 || r === 4 ? 3 : r === 5 ? 5 : r >= 3 ? 3 : 1);
    setStorage(cfg.storage || "file");
    setHistory(String(cfg.history && cfg.history > 0 ? cfg.history : 1));
    setTtlOn(isLimited(cfg.ttlNs));
    setTtl(formatDuration(cfg.ttlNs, "1h"));
    setMaxValueOn(isLimited(cfg.maxValueSize));
    const mv = pickByteUnit(cfg.maxValueSize && cfg.maxValueSize > 0 ? cfg.maxValueSize : 1 << 20);
    setMaxValueValue(mv.value);
    setMaxValueUnit(mv.unit);
    setMaxBytesOn(isLimited(cfg.maxBytes));
    const mb = pickByteUnit(cfg.maxBytes && cfg.maxBytes > 0 ? cfg.maxBytes : 1 << 30);
    setMaxBytesValue(mb.value);
    setMaxBytesUnit(mb.unit);
    setClusterOn(Boolean(cfg.placement?.cluster));
    setCluster(cfg.placement?.cluster ?? "");
    setTagsOn((cfg.placement?.tags?.length ?? 0) > 0);
    setTags([...(cfg.placement?.tags ?? [])]);
    setTagDraft("");
    setCompression(Boolean(cfg.compressed));
    setLimitMarkerOn(isLimited(cfg.limitMarkerTTLNs));
    setLimitMarkerTTL(formatDuration(cfg.limitMarkerTTLNs, "1s"));
    setRepublishOn(Boolean(cfg.republish?.dest));
    setRepublishSrc(cfg.republish?.src ?? "");
    setRepublishDest(cfg.republish?.dest ?? "");
    setRepublishHeadersOnly(Boolean(cfg.republish?.headersOnly));
    setMirrorOn(Boolean(cfg.mirror?.name));
    setMirrorName(cfg.mirror?.name ?? "");
    setMirrorFilter(cfg.mirror?.filterSubject ?? "");
    setSources([...(cfg.sources ?? [])]);
    setSourceName("");
    setSourceFilter("");
    setMetadata(
      Object.entries(cfg.metadata ?? {}).map(([key, value]) => ({ key, value: String(value) })),
    );
    setMetaKey("");
    setMetaValue("");
    setLocalError("");
  }, [open, initial]);

  if (!open) return null;

  function addTag() {
    const next = tagDraft.trim();
    if (!next || tags.includes(next)) return;
    setTags((prev) => [...prev, next]);
    setTagDraft("");
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setLocalError("");
    if (!bucket.trim()) {
      setLocalError(t("kvConfig.nameRequired"));
      return;
    }
    const hist = Number(history);
    if (!Number.isFinite(hist) || hist < 1 || hist > 64) {
      setLocalError(t("kvConfig.invalidHistory"));
      return;
    }

    let ttlNs = 0;
    if (ttlOn) {
      const parsed = parseDurationNs(ttl);
      if (parsed == null) {
        setLocalError(t("kvConfig.invalidTTL"));
        return;
      }
      ttlNs = parsed;
    }

    let maxValueSize = 0;
    if (maxValueOn) {
      const n = Number(maxValueValue);
      if (!Number.isFinite(n) || n <= 0) {
        setLocalError(t("kvConfig.invalidMaxValueSize"));
        return;
      }
      maxValueSize = Math.round(n * BYTE_UNIT_MULT[maxValueUnit]);
    }

    let maxBytes = 0;
    if (maxBytesOn) {
      const n = Number(maxBytesValue);
      if (!Number.isFinite(n) || n <= 0) {
        setLocalError(t("kvConfig.invalidMaxBytes"));
        return;
      }
      maxBytes = Math.round(n * BYTE_UNIT_MULT[maxBytesUnit]);
    }

    let limitMarkerTTLNs = 0;
    if (limitMarkerOn) {
      const parsed = parseDurationNs(limitMarkerTTL);
      if (parsed == null) {
        setLocalError(t("kvConfig.invalidLimitMarkerTTL"));
        return;
      }
      limitMarkerTTLNs = parsed;
    }

    if (republishOn && !republishDest.trim()) {
      setLocalError(t("kvConfig.republishDestRequired"));
      return;
    }
    if (mirrorOn && !mirrorName.trim()) {
      setLocalError(t("kvConfig.mirrorNameRequired"));
      return;
    }

    const body: KVBucketConfigPayload = {
      bucket: bucket.trim(),
      description: description.trim() || undefined,
      storage,
      history: Math.floor(hist),
      ttlNs: ttlNs || undefined,
      limitMarkerTTLNs: limitMarkerTTLNs || undefined,
      maxValueSize: maxValueSize || undefined,
      maxBytes: maxBytes || undefined,
      replicas,
      compression: compression || undefined,
    };
    if (clusterOn || tagsOn) {
      body.placement = {
        cluster: clusterOn ? cluster.trim() : undefined,
        tags: tagsOn ? tags : undefined,
      };
    }
    if (republishOn) {
      body.republish = {
        src: republishSrc.trim() || undefined,
        dest: republishDest.trim(),
        headersOnly: republishHeadersOnly || undefined,
      };
    }
    if (mirrorOn) {
      body.mirror = {
        name: mirrorName.trim(),
        filterSubject: mirrorFilter.trim() || undefined,
      };
    }
    if (sources.length > 0) {
      body.sources = sources;
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
            {mode === "edit" ? t("kvConfig.editTitle") : t("kvConfig.createTitle")}
          </h2>
          <button type="button" className="btn secondary" onClick={onClose} aria-label={t("common.close")}>
            {t("common.close")}
          </button>
        </header>

        <form className="stream-config-panel__body" onSubmit={handleSubmit}>
          <section className="stream-config-section">
            <h3>{t("kvConfig.basicInfo")}</h3>
            <p>{t("kvConfig.basicInfoLead")}</p>
            <div className="nc-form-row">
              <label htmlFor="kv-cfg-name">{t("common.name")}</label>
              <input
                id="kv-cfg-name"
                required
                disabled={mode === "edit"}
                value={bucket}
                onChange={(e) => setBucket(e.target.value)}
                placeholder={t("kvConfig.namePlaceholder")}
                aria-describedby="kv-cfg-name-hint"
              />
              <FieldHint id="kv-cfg-name-hint">{t("kvConfig.nameHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="kv-cfg-desc">{t("common.description")}</label>
              <input
                id="kv-cfg-desc"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder={t("kvConfig.descriptionPlaceholder")}
                aria-describedby="kv-cfg-desc-hint"
              />
              <FieldHint id="kv-cfg-desc-hint">{t("kvConfig.descriptionHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="kv-cfg-replicas">{t("kvConfig.replicas")}</label>
              <select
                id="kv-cfg-replicas"
                value={replicas}
                onChange={(e) => setReplicas(Number(e.target.value) || 1)}
                aria-describedby="kv-cfg-replicas-hint"
              >
                <option value={1}>1</option>
                <option value={3}>3</option>
                <option value={5}>5</option>
              </select>
              <FieldHint id="kv-cfg-replicas-hint">{t("kvConfig.replicasHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="kv-cfg-storage">{t("jetstream.storage")}</label>
              <select
                id="kv-cfg-storage"
                value={storage}
                onChange={(e) => setStorage(e.target.value)}
                aria-describedby="kv-cfg-storage-hint"
              >
                <option value="file">File</option>
                <option value="memory">Memory</option>
              </select>
              <FieldHint id="kv-cfg-storage-hint">{t("kvConfig.storageHint")}</FieldHint>
            </div>
          </section>

          <section className="stream-config-section">
            <h3>{t("kvConfig.storeSettings")}</h3>
            <p>{t("kvConfig.storeSettingsLead")}</p>

            <div className="nc-form-row">
              <label htmlFor="kv-cfg-history">{t("kvConfig.history")}</label>
              <input
                id="kv-cfg-history"
                type="number"
                min={1}
                max={64}
                value={history}
                onChange={(e) => setHistory(e.target.value)}
                aria-describedby="kv-cfg-history-hint"
              />
              <FieldHint id="kv-cfg-history-hint">{t("kvConfig.historyHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("kvConfig.ttl")}</span>
                <Switch on={ttlOn} label={t("kvConfig.ttl")} onToggle={() => setTtlOn((v) => !v)} />
              </div>
              {ttlOn && <input value={ttl} onChange={(e) => setTtl(e.target.value)} placeholder="1h" />}
              <FieldHint>{t("kvConfig.ttlHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("kvConfig.maxValueSize")}</span>
                <Switch
                  on={maxValueOn}
                  label={t("kvConfig.maxValueSize")}
                  onToggle={() => setMaxValueOn((v) => !v)}
                />
              </div>
              {maxValueOn && (
                <div className="stream-config-unit-row">
                  <input
                    type="number"
                    min={1}
                    value={maxValueValue}
                    onChange={(e) => setMaxValueValue(e.target.value)}
                  />
                  <select value={maxValueUnit} onChange={(e) => setMaxValueUnit(e.target.value as ByteUnit)}>
                    {(["B", "KiB", "MiB", "GiB", "TiB"] as ByteUnit[]).map((u) => (
                      <option key={u} value={u}>
                        {u}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              <FieldHint>{t("kvConfig.maxValueSizeHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("kvConfig.maxBytes")}</span>
                <Switch on={maxBytesOn} label={t("kvConfig.maxBytes")} onToggle={() => setMaxBytesOn((v) => !v)} />
              </div>
              {maxBytesOn && (
                <div className="stream-config-unit-row">
                  <input
                    type="number"
                    min={1}
                    value={maxBytesValue}
                    onChange={(e) => setMaxBytesValue(e.target.value)}
                  />
                  <select value={maxBytesUnit} onChange={(e) => setMaxBytesUnit(e.target.value as ByteUnit)}>
                    {(["B", "KiB", "MiB", "GiB", "TiB"] as ByteUnit[]).map((u) => (
                      <option key={u} value={u}>
                        {u}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              <FieldHint>{t("kvConfig.maxBytesHint")}</FieldHint>
            </div>
          </section>

          <section className="stream-config-section">
            <h3>{t("kvConfig.placement")}</h3>
            <p>{t("kvConfig.placementLead")}</p>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("kvConfig.cluster")}</span>
                <Switch on={clusterOn} label={t("kvConfig.cluster")} onToggle={() => setClusterOn((v) => !v)} />
              </div>
              {clusterOn && (
                <input value={cluster} onChange={(e) => setCluster(e.target.value)} placeholder="east" />
              )}
              <FieldHint>{t("kvConfig.clusterHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("kvConfig.tags")}</span>
                <Switch on={tagsOn} label={t("kvConfig.tags")} onToggle={() => setTagsOn((v) => !v)} />
              </div>
              {tagsOn && (
                <>
                  <div className="stream-config-add-row">
                    <input
                      value={tagDraft}
                      onChange={(e) => setTagDraft(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          e.preventDefault();
                          addTag();
                        }
                      }}
                      placeholder="ssd"
                    />
                    <button type="button" className="btn secondary" onClick={addTag}>
                      {t("streamConfig.add")}
                    </button>
                  </div>
                  {tags.length > 0 && (
                    <ul className="stream-config-chips">
                      {tags.map((tag) => (
                        <li key={tag}>
                          <span>{tag}</span>
                          <button
                            type="button"
                            className="stream-config-chip-remove"
                            aria-label={t("common.delete")}
                            onClick={() => setTags((prev) => prev.filter((x) => x !== tag))}
                          >
                            ×
                          </button>
                        </li>
                      ))}
                    </ul>
                  )}
                </>
              )}
              <FieldHint>{t("kvConfig.tagsHint")}</FieldHint>
            </div>
          </section>

          <section className="stream-config-section">
            <h3>{t("kvConfig.advanced")}</h3>
            <p>{t("kvConfig.advancedLead")}</p>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("kvConfig.compression")}</span>
                <Switch
                  on={compression}
                  label={t("kvConfig.compression")}
                  onToggle={() => setCompression((v) => !v)}
                />
              </div>
              <FieldHint>{t("kvConfig.compressionHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("kvConfig.limitMarkerTTL")}</span>
                <Switch
                  on={limitMarkerOn}
                  label={t("kvConfig.limitMarkerTTL")}
                  onToggle={() => setLimitMarkerOn((v) => !v)}
                />
              </div>
              {limitMarkerOn && (
                <input
                  value={limitMarkerTTL}
                  onChange={(e) => setLimitMarkerTTL(e.target.value)}
                  placeholder="1s"
                />
              )}
              <FieldHint>{t("kvConfig.limitMarkerTTLHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("kvConfig.republish")}</span>
                <Switch
                  on={republishOn}
                  label={t("kvConfig.republish")}
                  onToggle={() => setRepublishOn((v) => !v)}
                />
              </div>
              {republishOn && (
                <>
                  <input
                    value={republishSrc}
                    onChange={(e) => setRepublishSrc(e.target.value)}
                    placeholder={t("kvConfig.republishSrc")}
                  />
                  <input
                    className="mt-8"
                    value={republishDest}
                    onChange={(e) => setRepublishDest(e.target.value)}
                    placeholder={t("kvConfig.republishDest")}
                    required
                  />
                  <div className="stream-config-toggle-row mt-8">
                    <span>{t("kvConfig.headersOnly")}</span>
                    <Switch
                      on={republishHeadersOnly}
                      label={t("kvConfig.headersOnly")}
                      onToggle={() => setRepublishHeadersOnly((v) => !v)}
                    />
                  </div>
                </>
              )}
              <FieldHint>{t("kvConfig.republishHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("kvConfig.mirror")}</span>
                <Switch on={mirrorOn} label={t("kvConfig.mirror")} onToggle={() => setMirrorOn((v) => !v)} />
              </div>
              {mirrorOn && (
                <>
                  <input
                    value={mirrorName}
                    onChange={(e) => setMirrorName(e.target.value)}
                    placeholder={t("kvConfig.mirrorName")}
                    required
                  />
                  <input
                    className="mt-8"
                    value={mirrorFilter}
                    onChange={(e) => setMirrorFilter(e.target.value)}
                    placeholder={t("kvConfig.sourceFilter")}
                  />
                </>
              )}
              <FieldHint>{t("kvConfig.mirrorHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <label className="stream-config-mini-label">{t("kvConfig.sources")}</label>
              <div className="stream-config-add-row">
                <input
                  value={sourceName}
                  onChange={(e) => setSourceName(e.target.value)}
                  placeholder={t("kvConfig.sourceName")}
                />
                <input
                  value={sourceFilter}
                  onChange={(e) => setSourceFilter(e.target.value)}
                  placeholder={t("kvConfig.sourceFilter")}
                />
                <button
                  type="button"
                  className="btn secondary"
                  onClick={() => {
                    const name = sourceName.trim();
                    if (!name) return;
                    setSources((prev) => [
                      ...prev,
                      { name, filterSubject: sourceFilter.trim() || undefined },
                    ]);
                    setSourceName("");
                    setSourceFilter("");
                  }}
                >
                  {t("streamConfig.add")}
                </button>
              </div>
              <FieldHint>{t("kvConfig.sourcesHint")}</FieldHint>
              {sources.length > 0 && (
                <ul className="stream-config-chips">
                  {sources.map((src, idx) => (
                    <li key={`${src.name}-${idx}`}>
                      <span className="mono">
                        {src.name}
                        {src.filterSubject ? ` (${src.filterSubject})` : ""}
                      </span>
                      <button
                        type="button"
                        className="stream-config-chip-remove"
                        aria-label={t("common.delete")}
                        onClick={() => setSources((prev) => prev.filter((_, i) => i !== idx))}
                      >
                        ×
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>

            <div className="stream-config-limit">
              <label className="stream-config-mini-label">{t("kvConfig.metadata")}</label>
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
              <FieldHint>{t("kvConfig.metadataHint")}</FieldHint>
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
