import { FormEvent, useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { StreamConfig, StreamSource } from "../lib/api";
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

export type StreamConfigPayload = StreamConfig;

type Props = {
  mode: "create" | "edit";
  open: boolean;
  /** "mirror" creates/edits a mirrored stream (subjects hidden; mirror source required). */
  variant?: "stream" | "mirror";
  initial?: Partial<StreamConfig> | null;
  busy?: boolean;
  error?: string;
  onClose: () => void;
  onSubmit: (body: StreamConfigPayload) => Promise<void> | void;
};

type MetaRow = { key: string; value: string };

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
  const d = ns;
  const hour = 3_600_000_000_000;
  const minute = 60_000_000_000;
  const second = 1_000_000_000;
  if (d % hour === 0) return `${d / hour}h`;
  if (d % minute === 0) return `${d / minute}m`;
  if (d % second === 0) return `${d / second}s`;
  return `${d}ns`;
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

export default function CreateStreamPanel({
  mode,
  open,
  variant = "stream",
  initial,
  busy,
  error,
  onClose,
  onSubmit,
}: Props) {
  const { t } = useTranslation();
  const titleId = useId();
  const isMirror = variant === "mirror" || Boolean(initial?.mirror);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [replicas, setReplicas] = useState(1);
  const [storage, setStorage] = useState("file");
  const [subjects, setSubjects] = useState<string[]>([]);
  const [subjectDraft, setSubjectDraft] = useState("");
  const [mirrorSourceName, setMirrorSourceName] = useState("");
  const [mirrorFilter, setMirrorFilter] = useState("");
  const [mirrorStartSeqOn, setMirrorStartSeqOn] = useState(false);
  const [mirrorStartSeq, setMirrorStartSeq] = useState("1");
  const [mirrorStartTimeOn, setMirrorStartTimeOn] = useState(false);
  const [mirrorStartTime, setMirrorStartTime] = useState("");
  const [mirrorExternalOn, setMirrorExternalOn] = useState(false);
  const [mirrorApiPrefix, setMirrorApiPrefix] = useState("");
  const [mirrorDeliverPrefix, setMirrorDeliverPrefix] = useState("");
  const [retention, setRetention] = useState("limits");
  const [discard, setDiscard] = useState("old");
  const [allowRollup, setAllowRollup] = useState(true);
  const [allowDeletion, setAllowDeletion] = useState(true);
  const [allowPurging, setAllowPurging] = useState(true);
  const [maxAgeOn, setMaxAgeOn] = useState(false);
  const [maxAge, setMaxAge] = useState("1h");
  const [maxBytesOn, setMaxBytesOn] = useState(true);
  const [maxBytesValue, setMaxBytesValue] = useState("1");
  const [maxBytesUnit, setMaxBytesUnit] = useState<ByteUnit>("GiB");
  const [maxConsumersOn, setMaxConsumersOn] = useState(false);
  const [maxConsumers, setMaxConsumers] = useState("10");
  const [maxMsgSizeOn, setMaxMsgSizeOn] = useState(false);
  const [maxMsgSize, setMaxMsgSize] = useState("1");
  const [maxMsgSizeUnit, setMaxMsgSizeUnit] = useState<ByteUnit>("MiB");
  const [maxMsgsOn, setMaxMsgsOn] = useState(false);
  const [maxMsgs, setMaxMsgs] = useState("1000");
  const [maxMsgsPerSubjectOn, setMaxMsgsPerSubjectOn] = useState(false);
  const [maxMsgsPerSubject, setMaxMsgsPerSubject] = useState("100");
  const [clusterOn, setClusterOn] = useState(false);
  const [cluster, setCluster] = useState("");
  const [tagsOn, setTagsOn] = useState(false);
  const [tags, setTags] = useState<string[]>([]);
  const [tagDraft, setTagDraft] = useState("");

  const [compression, setCompression] = useState("none");
  const [duplicatesOn, setDuplicatesOn] = useState(false);
  const [duplicates, setDuplicates] = useState("2m");
  const [discardNewPerSubject, setDiscardNewPerSubject] = useState(false);
  const [noAck, setNoAck] = useState(false);
  const [allowDirect, setAllowDirect] = useState(false);
  const [mirrorDirect, setMirrorDirect] = useState(false);
  const [allowMsgTTL, setAllowMsgTTL] = useState(false);
  const [sealed, setSealed] = useState(false);
  const [firstSeqOn, setFirstSeqOn] = useState(false);
  const [firstSeq, setFirstSeq] = useState("1");
  const [markerTTLOn, setMarkerTTLOn] = useState(false);
  const [markerTTL, setMarkerTTL] = useState("1h");
  const [transformOn, setTransformOn] = useState(false);
  const [transformSrc, setTransformSrc] = useState("");
  const [transformDest, setTransformDest] = useState("");
  const [republishOn, setRepublishOn] = useState(false);
  const [republishSrc, setRepublishSrc] = useState("");
  const [republishDest, setRepublishDest] = useState("");
  const [republishHeadersOnly, setRepublishHeadersOnly] = useState(false);
  const [consumerLimitsOn, setConsumerLimitsOn] = useState(false);
  const [inactiveThreshold, setInactiveThreshold] = useState("1m");
  const [maxAckPending, setMaxAckPending] = useState("");
  const [sources, setSources] = useState<StreamSource[]>([]);
  const [sourceName, setSourceName] = useState("");
  const [sourceFilter, setSourceFilter] = useState("");
  const [metadata, setMetadata] = useState<MetaRow[]>([]);
  const [metaKey, setMetaKey] = useState("");
  const [metaValue, setMetaValue] = useState("");
  const [localError, setLocalError] = useState("");

  useEffect(() => {
    if (!open) return;
    const cfg = initial ?? {};
    setName(cfg.name ?? "");
    setDescription(cfg.description ?? "");
    setReplicas(
      cfg.replicas === 5 || cfg.replicas === 3 || cfg.replicas === 1 ? cfg.replicas : 1,
    );
    setStorage(cfg.storage || "file");
    setSubjects([...(cfg.subjects ?? [])]);
    setSubjectDraft("");
    const mirror = cfg.mirror;
    setMirrorSourceName(mirror?.name ?? "");
    setMirrorFilter(mirror?.filterSubject ?? "");
    setMirrorStartSeqOn(Boolean(mirror?.optStartSeq && mirror.optStartSeq > 0));
    setMirrorStartSeq(String(mirror?.optStartSeq && mirror.optStartSeq > 0 ? mirror.optStartSeq : 1));
    setMirrorStartTimeOn(Boolean(mirror?.optStartTime));
    setMirrorStartTime(mirror?.optStartTime ?? "");
    setMirrorExternalOn(Boolean(mirror?.external?.api || mirror?.external?.deliver));
    setMirrorApiPrefix(mirror?.external?.api ?? "");
    setMirrorDeliverPrefix(mirror?.external?.deliver ?? "");
    setRetention(cfg.retention || "limits");
    setDiscard(cfg.discard || "old");
    setAllowRollup(cfg.allowRollup ?? true);
    setAllowDeletion(!(cfg.denyDelete ?? false));
    setAllowPurging(!(cfg.denyPurge ?? false));

    setMaxAgeOn(isLimited(cfg.maxAge));
    setMaxAge(formatDuration(cfg.maxAge));

    const bytesOn = isLimited(cfg.maxBytes);
    setMaxBytesOn(mode === "create" ? (cfg.maxBytes === undefined ? true : bytesOn) : bytesOn);
    const picked = pickByteUnit(cfg.maxBytes && cfg.maxBytes > 0 ? cfg.maxBytes : 1 << 30);
    setMaxBytesValue(picked.value);
    setMaxBytesUnit(picked.unit);

    setMaxConsumersOn(isLimited(cfg.maxConsumers));
    setMaxConsumers(String(cfg.maxConsumers && cfg.maxConsumers > 0 ? cfg.maxConsumers : 10));

    setMaxMsgSizeOn(isLimited(cfg.maxMsgSize));
    const msgSize = pickByteUnit(cfg.maxMsgSize && cfg.maxMsgSize > 0 ? cfg.maxMsgSize : 1 << 20);
    setMaxMsgSize(msgSize.value);
    setMaxMsgSizeUnit(msgSize.unit);

    setMaxMsgsOn(isLimited(cfg.maxMsgs));
    setMaxMsgs(String(cfg.maxMsgs && cfg.maxMsgs > 0 ? cfg.maxMsgs : 1000));

    setMaxMsgsPerSubjectOn(isLimited(cfg.maxMsgsPerSubject));
    setMaxMsgsPerSubject(
      String(cfg.maxMsgsPerSubject && cfg.maxMsgsPerSubject > 0 ? cfg.maxMsgsPerSubject : 100),
    );

    const hasCluster = Boolean(cfg.placement?.cluster);
    const hasTags = (cfg.placement?.tags?.length ?? 0) > 0;
    setClusterOn(hasCluster);
    setCluster(cfg.placement?.cluster ?? "");
    setTagsOn(hasTags);
    setTags([...(cfg.placement?.tags ?? [])]);
    setTagDraft("");

    setCompression(cfg.compression || "none");
    setDuplicatesOn(isLimited(cfg.duplicates));
    setDuplicates(formatDuration(cfg.duplicates, "2m"));
    setDiscardNewPerSubject(cfg.discardNewPerSubject ?? false);
    setNoAck(cfg.noAck ?? false);
    setAllowDirect(cfg.allowDirect ?? false);
    setMirrorDirect(cfg.mirrorDirect ?? (Boolean(cfg.mirror) || variant === "mirror"));
    setAllowMsgTTL(cfg.allowMsgTTL ?? false);
    setSealed(cfg.sealed ?? false);
    setFirstSeqOn(isLimited(cfg.firstSeq));
    setFirstSeq(String(cfg.firstSeq && cfg.firstSeq > 0 ? cfg.firstSeq : 1));
    setMarkerTTLOn(isLimited(cfg.subjectDeleteMarkerTTL));
    setMarkerTTL(formatDuration(cfg.subjectDeleteMarkerTTL, "1h"));
    setTransformOn(Boolean(cfg.subjectTransform?.dest));
    setTransformSrc(cfg.subjectTransform?.src ?? "");
    setTransformDest(cfg.subjectTransform?.dest ?? "");
    setRepublishOn(Boolean(cfg.republish?.dest));
    setRepublishSrc(cfg.republish?.src ?? "");
    setRepublishDest(cfg.republish?.dest ?? "");
    setRepublishHeadersOnly(cfg.republish?.headersOnly ?? false);
    const cl = cfg.consumerLimits;
    setConsumerLimitsOn(Boolean(cl && (isLimited(cl.inactiveThreshold) || (cl.maxAckPending ?? 0) > 0)));
    setInactiveThreshold(formatDuration(cl?.inactiveThreshold, "1m"));
    setMaxAckPending(cl?.maxAckPending && cl.maxAckPending > 0 ? String(cl.maxAckPending) : "");
    setSources([...(cfg.sources ?? [])]);
    setSourceName("");
    setSourceFilter("");
    setMetadata(
      Object.entries(cfg.metadata ?? {}).map(([key, value]) => ({ key, value })),
    );
    setMetaKey("");
    setMetaValue("");
    setLocalError("");
  }, [open, initial, mode, variant]);

  if (!open) return null;

  function addSubject() {
    const next = subjectDraft.trim();
    if (!next || subjects.includes(next)) return;
    setSubjects((prev) => [...prev, next]);
    setSubjectDraft("");
  }

  function addTag() {
    const next = tagDraft.trim();
    if (!next || tags.includes(next)) return;
    setTags((prev) => [...prev, next]);
    setTagDraft("");
  }

  function addSource() {
    const next = sourceName.trim();
    if (!next) return;
    setSources((prev) => [
      ...prev,
      { name: next, filterSubject: sourceFilter.trim() || undefined },
    ]);
    setSourceName("");
    setSourceFilter("");
  }

  function addMeta() {
    const key = metaKey.trim();
    if (!key) return;
    setMetadata((prev) => {
      const without = prev.filter((row) => row.key !== key);
      return [...without, { key, value: metaValue }];
    });
    setMetaKey("");
    setMetaValue("");
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setLocalError("");
    if (!name.trim()) {
      setLocalError(t("streamConfig.nameRequired"));
      return;
    }
    if (isMirror) {
      if (!mirrorSourceName.trim()) {
        setLocalError(t("streamConfig.mirrorSourceRequired"));
        return;
      }
    } else if (subjects.length === 0 && sources.length === 0 && !initial?.mirror) {
      setLocalError(t("streamConfig.subjectsRequired"));
      return;
    }

    const body: StreamConfigPayload = {
      name: name.trim(),
      description: description.trim() || undefined,
      subjects: isMirror ? undefined : subjects.length ? subjects : undefined,
      retention,
      storage,
      discard,
      compression: compression === "none" ? "none" : compression,
      replicas: replicas === 5 || replicas === 3 ? replicas : 1,
      allowRollup,
      denyDelete: !allowDeletion,
      denyPurge: !allowPurging,
      discardNewPerSubject,
      noAck,
      allowDirect,
      mirrorDirect,
      allowMsgTTL,
    };

    if (mode === "edit") {
      body.sealed = sealed;
    }
    if (isMirror) {
      const mirror: StreamSource = {
        name: mirrorSourceName.trim(),
        filterSubject: mirrorFilter.trim() || undefined,
      };
      if (mirrorStartSeqOn) {
        const n = Number(mirrorStartSeq);
        if (!Number.isFinite(n) || n <= 0) {
          setLocalError(t("streamConfig.invalidMirrorStartSeq"));
          return;
        }
        mirror.optStartSeq = Math.floor(n);
      }
      if (mirrorStartTimeOn) {
        const ts = mirrorStartTime.trim();
        if (!ts) {
          setLocalError(t("streamConfig.invalidMirrorStartTime"));
          return;
        }
        mirror.optStartTime = ts;
      }
      if (mirrorExternalOn && (mirrorApiPrefix.trim() || mirrorDeliverPrefix.trim())) {
        mirror.external = {
          api: mirrorApiPrefix.trim() || undefined,
          deliver: mirrorDeliverPrefix.trim() || undefined,
        };
      }
      body.mirror = mirror;
    } else if (initial?.mirror) {
      body.mirror = initial.mirror;
    }
    if (!isMirror && sources.length > 0) {
      body.sources = sources;
    }

    if (maxAgeOn) {
      const ns = parseDurationNs(maxAge);
      if (ns == null) {
        setLocalError(t("streamConfig.invalidMaxAge"));
        return;
      }
      body.maxAge = ns;
    }
    if (maxBytesOn) {
      const n = Number(maxBytesValue);
      if (!Number.isFinite(n) || n <= 0) {
        setLocalError(t("streamConfig.invalidMaxBytes"));
        return;
      }
      body.maxBytes = Math.round(n * BYTE_UNIT_MULT[maxBytesUnit]);
    }
    if (maxConsumersOn) {
      const n = Number(maxConsumers);
      if (!Number.isFinite(n) || n <= 0) {
        setLocalError(t("streamConfig.invalidMaxConsumers"));
        return;
      }
      body.maxConsumers = Math.floor(n);
    }
    if (maxMsgSizeOn) {
      const n = Number(maxMsgSize);
      if (!Number.isFinite(n) || n <= 0) {
        setLocalError(t("streamConfig.invalidMaxMsgSize"));
        return;
      }
      body.maxMsgSize = Math.round(n * BYTE_UNIT_MULT[maxMsgSizeUnit]);
    }
    if (maxMsgsOn) {
      const n = Number(maxMsgs);
      if (!Number.isFinite(n) || n <= 0) {
        setLocalError(t("streamConfig.invalidMaxMsgs"));
        return;
      }
      body.maxMsgs = Math.floor(n);
    }
    if (maxMsgsPerSubjectOn) {
      const n = Number(maxMsgsPerSubject);
      if (!Number.isFinite(n) || n <= 0) {
        setLocalError(t("streamConfig.invalidMaxMsgsPerSubject"));
        return;
      }
      body.maxMsgsPerSubject = Math.floor(n);
    }
    if (duplicatesOn) {
      const ns = parseDurationNs(duplicates);
      if (ns == null) {
        setLocalError(t("streamConfig.invalidDuplicates"));
        return;
      }
      body.duplicates = ns;
    }
    if (firstSeqOn) {
      const n = Number(firstSeq);
      if (!Number.isFinite(n) || n < 1) {
        setLocalError(t("streamConfig.invalidFirstSeq"));
        return;
      }
      body.firstSeq = Math.floor(n);
    }
    if (markerTTLOn) {
      const ns = parseDurationNs(markerTTL);
      if (ns == null) {
        setLocalError(t("streamConfig.invalidMarkerTTL"));
        return;
      }
      body.subjectDeleteMarkerTTL = ns;
    }
    if (transformOn) {
      if (!transformDest.trim()) {
        setLocalError(t("streamConfig.transformDestRequired"));
        return;
      }
      body.subjectTransform = {
        src: transformSrc.trim() || undefined,
        dest: transformDest.trim(),
      };
    }
    if (republishOn) {
      if (!republishDest.trim()) {
        setLocalError(t("streamConfig.republishDestRequired"));
        return;
      }
      body.republish = {
        src: republishSrc.trim() || undefined,
        dest: republishDest.trim(),
        headersOnly: republishHeadersOnly,
      };
    }
    if (consumerLimitsOn) {
      const limits: { inactiveThreshold?: number; maxAckPending?: number } = {};
      if (inactiveThreshold.trim()) {
        const ns = parseDurationNs(inactiveThreshold);
        if (ns == null) {
          setLocalError(t("streamConfig.invalidInactiveThreshold"));
          return;
        }
        limits.inactiveThreshold = ns;
      }
      if (maxAckPending.trim()) {
        const n = Number(maxAckPending);
        if (!Number.isFinite(n) || n <= 0) {
          setLocalError(t("streamConfig.invalidMaxAckPending"));
          return;
        }
        limits.maxAckPending = Math.floor(n);
      }
      body.consumerLimits = limits;
    }
    if (clusterOn || tagsOn) {
      body.placement = {
        cluster: clusterOn ? cluster.trim() || undefined : undefined,
        tags: tagsOn ? tags : undefined,
      };
    }
    if (metadata.length > 0) {
      body.metadata = Object.fromEntries(metadata.map((row) => [row.key, row.value]));
    }

    try {
      await onSubmit(body);
    } catch {
      // onSubmit surfaces errors to the panel
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
            {isMirror
              ? mode === "edit"
                ? t("streamConfig.editMirrorTitle")
                : t("streamConfig.createMirrorTitle")
              : mode === "edit"
                ? t("streamConfig.editTitle")
                : t("streamConfig.createTitle")}
          </h2>
          <button type="button" className="btn secondary" onClick={onClose} aria-label={t("common.close")}>
            {t("common.close")}
          </button>
        </header>

        <form className="stream-config-panel__body" onSubmit={handleSubmit}>
          <section className="stream-config-section">
            <h3>{t("streamConfig.basicInfo")}</h3>
            <p>{t("streamConfig.basicInfoLead")}</p>
            <div className="nc-form-row">
              <label htmlFor="stream-cfg-name">{t("common.name")}</label>
              <input
                id="stream-cfg-name"
                required
                disabled={mode === "edit"}
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t("streamConfig.namePlaceholder")}
                aria-describedby="stream-cfg-name-hint"
              />
              <FieldHint id="stream-cfg-name-hint">{t("streamConfig.nameHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="stream-cfg-desc">{t("common.description")}</label>
              <input
                id="stream-cfg-desc"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder={t("streamConfig.descriptionPlaceholder")}
                aria-describedby="stream-cfg-desc-hint"
              />
              <FieldHint id="stream-cfg-desc-hint">{t("streamConfig.descriptionHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="stream-cfg-replicas">{t("streamConfig.replicas")}</label>
              <select
                id="stream-cfg-replicas"
                value={replicas === 2 || replicas === 4 ? 3 : replicas}
                onChange={(e) => setReplicas(Number(e.target.value) || 1)}
                aria-describedby="stream-cfg-replicas-hint"
              >
                <option value={1}>1</option>
                <option value={3}>3</option>
                <option value={5}>5</option>
              </select>
              <FieldHint id="stream-cfg-replicas-hint">{t("streamConfig.replicasHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="stream-cfg-storage">{t("jetstream.storage")}</label>
              <select
                id="stream-cfg-storage"
                value={storage}
                onChange={(e) => setStorage(e.target.value)}
                aria-describedby="stream-cfg-storage-hint"
              >
                <option value="file">File</option>
                <option value="memory">Memory</option>
              </select>
              <FieldHint id="stream-cfg-storage-hint">{t("streamConfig.storageHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="stream-cfg-compression">{t("streamConfig.compression")}</label>
              <select
                id="stream-cfg-compression"
                value={compression}
                onChange={(e) => setCompression(e.target.value)}
                aria-describedby="stream-cfg-compression-hint"
              >
                <option value="none">None</option>
                <option value="s2">S2</option>
              </select>
              <FieldHint id="stream-cfg-compression-hint">{t("streamConfig.compressionHint")}</FieldHint>
            </div>
          </section>

          {isMirror && (
            <section className="stream-config-section">
              <h3>{t("streamConfig.mirrorSource")}</h3>
              <p>{t("streamConfig.mirrorSourceLead")}</p>
              <div className="nc-form-row">
                <label htmlFor="stream-cfg-mirror-name">{t("streamConfig.mirrorSourceName")}</label>
                <input
                  id="stream-cfg-mirror-name"
                  required
                  value={mirrorSourceName}
                  onChange={(e) => setMirrorSourceName(e.target.value)}
                  placeholder={t("streamConfig.mirrorSourceNamePlaceholder")}
                  aria-describedby="stream-cfg-mirror-name-hint"
                />
                <FieldHint id="stream-cfg-mirror-name-hint">{t("streamConfig.mirrorSourceNameHint")}</FieldHint>
              </div>
              <div className="nc-form-row">
                <label htmlFor="stream-cfg-mirror-filter">{t("streamConfig.mirrorFilter")}</label>
                <input
                  id="stream-cfg-mirror-filter"
                  value={mirrorFilter}
                  onChange={(e) => setMirrorFilter(e.target.value)}
                  placeholder={t("streamConfig.sourceFilter")}
                  aria-describedby="stream-cfg-mirror-filter-hint"
                />
                <FieldHint id="stream-cfg-mirror-filter-hint">{t("streamConfig.mirrorFilterHint")}</FieldHint>
              </div>
              <div className="stream-config-limit">
                <div className="stream-config-toggle-row">
                  <span>{t("streamConfig.mirrorStartSeq")}</span>
                  <Switch
                    on={mirrorStartSeqOn}
                    label={t("streamConfig.mirrorStartSeq")}
                    onToggle={() => setMirrorStartSeqOn((v) => !v)}
                  />
                </div>
                {mirrorStartSeqOn && (
                  <input
                    type="number"
                    min={1}
                    value={mirrorStartSeq}
                    onChange={(e) => setMirrorStartSeq(e.target.value)}
                  />
                )}
                <FieldHint>{t("streamConfig.mirrorStartSeqHint")}</FieldHint>
              </div>
              <div className="stream-config-limit">
                <div className="stream-config-toggle-row">
                  <span>{t("streamConfig.mirrorStartTime")}</span>
                  <Switch
                    on={mirrorStartTimeOn}
                    label={t("streamConfig.mirrorStartTime")}
                    onToggle={() => setMirrorStartTimeOn((v) => !v)}
                  />
                </div>
                {mirrorStartTimeOn && (
                  <input
                    value={mirrorStartTime}
                    onChange={(e) => setMirrorStartTime(e.target.value)}
                    placeholder="2026-01-01T00:00:00Z"
                  />
                )}
                <FieldHint>{t("streamConfig.mirrorStartTimeHint")}</FieldHint>
              </div>
              <div className="stream-config-limit">
                <div className="stream-config-toggle-row">
                  <span>{t("streamConfig.mirrorExternal")}</span>
                  <Switch
                    on={mirrorExternalOn}
                    label={t("streamConfig.mirrorExternal")}
                    onToggle={() => setMirrorExternalOn((v) => !v)}
                  />
                </div>
                {mirrorExternalOn && (
                  <>
                    <input
                      value={mirrorApiPrefix}
                      onChange={(e) => setMirrorApiPrefix(e.target.value)}
                      placeholder={t("streamConfig.mirrorApiPrefix")}
                    />
                    <input
                      className="mt-8"
                      value={mirrorDeliverPrefix}
                      onChange={(e) => setMirrorDeliverPrefix(e.target.value)}
                      placeholder={t("streamConfig.mirrorDeliverPrefix")}
                    />
                  </>
                )}
                <FieldHint>{t("streamConfig.mirrorExternalHint")}</FieldHint>
              </div>
            </section>
          )}

          {!isMirror && (
          <section className="stream-config-section">
            <h3>{t("common.subjects")}</h3>
            <p>{t("streamConfig.subjectsLead")}</p>
            <div className="stream-config-limit">
              <div className="stream-config-add-row">
                <input
                  value={subjectDraft}
                  onChange={(e) => setSubjectDraft(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      addSubject();
                    }
                  }}
                  placeholder="orders.*"
                  aria-describedby="stream-cfg-subjects-hint"
                />
                <button type="button" className="btn secondary" onClick={addSubject}>
                  {t("streamConfig.add")}
                </button>
              </div>
              <FieldHint id="stream-cfg-subjects-hint">{t("streamConfig.subjectsHint")}</FieldHint>
              {subjects.length === 0 ? (
                <p className="text-muted">{t("streamConfig.noSubjects")}</p>
              ) : (
                <ul className="stream-config-chips">
                  {subjects.map((s) => (
                    <li key={s}>
                      <span className="mono">{s}</span>
                      <button
                        type="button"
                        className="stream-config-chip-remove"
                        aria-label={t("common.delete")}
                        onClick={() => setSubjects((prev) => prev.filter((x) => x !== s))}
                      >
                        ×
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </section>
          )}

          <section className="stream-config-section">
            <h3>{t("streamConfig.retention")}</h3>
            <p>{t("streamConfig.retentionLead")}</p>
            <div className="nc-form-row">
              <label htmlFor="stream-cfg-policy">{t("streamConfig.policy")}</label>
              <select
                id="stream-cfg-policy"
                value={retention}
                onChange={(e) => setRetention(e.target.value)}
                aria-describedby="stream-cfg-policy-hint"
              >
                <option value="limits">Limits</option>
                <option value="interest">Interest</option>
                <option value="workqueue">Workqueue</option>
              </select>
              <FieldHint id="stream-cfg-policy-hint">{t("streamConfig.policyHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="stream-cfg-discard">{t("streamConfig.discardPolicy")}</label>
              <select
                id="stream-cfg-discard"
                value={discard}
                onChange={(e) => setDiscard(e.target.value)}
                aria-describedby="stream-cfg-discard-hint"
              >
                <option value="old">Old</option>
                <option value="new">New</option>
              </select>
              <FieldHint id="stream-cfg-discard-hint">{t("streamConfig.discardPolicyHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.allowRollups")}</span>
                <Switch on={allowRollup} label={t("streamConfig.allowRollups")} onToggle={() => setAllowRollup((v) => !v)} />
              </div>
              <FieldHint>{t("streamConfig.allowRollupsHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.allowDeletion")}</span>
                <Switch
                  on={allowDeletion}
                  label={t("streamConfig.allowDeletion")}
                  onToggle={() => setAllowDeletion((v) => !v)}
                />
              </div>
              <FieldHint>{t("streamConfig.allowDeletionHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.allowPurging")}</span>
                <Switch
                  on={allowPurging}
                  label={t("streamConfig.allowPurging")}
                  onToggle={() => setAllowPurging((v) => !v)}
                />
              </div>
              <FieldHint>{t("streamConfig.allowPurgingHint")}</FieldHint>
            </div>
            {discard === "new" && (
              <div className="stream-config-limit">
                <div className="stream-config-toggle-row">
                  <span>{t("streamConfig.discardNewPerSubject")}</span>
                  <Switch
                    on={discardNewPerSubject}
                    label={t("streamConfig.discardNewPerSubject")}
                    onToggle={() => setDiscardNewPerSubject((v) => !v)}
                  />
                </div>
                <FieldHint>{t("streamConfig.discardNewPerSubjectHint")}</FieldHint>
              </div>
            )}
          </section>

          <section className="stream-config-section">
            <h3>{t("streamConfig.limits")}</h3>
            <p>{t("streamConfig.limitsLead")}</p>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.maxAge")}</span>
                <Switch on={maxAgeOn} label={t("streamConfig.maxAge")} onToggle={() => setMaxAgeOn((v) => !v)} />
              </div>
              {maxAgeOn && <input value={maxAge} onChange={(e) => setMaxAge(e.target.value)} placeholder="1h" />}
              <FieldHint>{t("streamConfig.maxAgeHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.maxBytes")}</span>
                <Switch on={maxBytesOn} label={t("streamConfig.maxBytes")} onToggle={() => setMaxBytesOn((v) => !v)} />
              </div>
              {maxBytesOn && (
                <div className="stream-config-unit-row">
                  <input type="number" min={1} value={maxBytesValue} onChange={(e) => setMaxBytesValue(e.target.value)} />
                  <select value={maxBytesUnit} onChange={(e) => setMaxBytesUnit(e.target.value as ByteUnit)}>
                    {(["B", "KiB", "MiB", "GiB", "TiB"] as ByteUnit[]).map((u) => (
                      <option key={u} value={u}>
                        {u}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              <FieldHint>{t("streamConfig.maxBytesHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.maxConsumers")}</span>
                <Switch
                  on={maxConsumersOn}
                  label={t("streamConfig.maxConsumers")}
                  onToggle={() => setMaxConsumersOn((v) => !v)}
                />
              </div>
              {maxConsumersOn && (
                <input type="number" min={1} value={maxConsumers} onChange={(e) => setMaxConsumers(e.target.value)} />
              )}
              <FieldHint>{t("streamConfig.maxConsumersHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.maxMsgSize")}</span>
                <Switch
                  on={maxMsgSizeOn}
                  label={t("streamConfig.maxMsgSize")}
                  onToggle={() => setMaxMsgSizeOn((v) => !v)}
                />
              </div>
              {maxMsgSizeOn && (
                <div className="stream-config-unit-row">
                  <input type="number" min={1} value={maxMsgSize} onChange={(e) => setMaxMsgSize(e.target.value)} />
                  <select value={maxMsgSizeUnit} onChange={(e) => setMaxMsgSizeUnit(e.target.value as ByteUnit)}>
                    {(["B", "KiB", "MiB", "GiB", "TiB"] as ByteUnit[]).map((u) => (
                      <option key={u} value={u}>
                        {u}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              <FieldHint>{t("streamConfig.maxMsgSizeHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.maxMsgs")}</span>
                <Switch on={maxMsgsOn} label={t("streamConfig.maxMsgs")} onToggle={() => setMaxMsgsOn((v) => !v)} />
              </div>
              {maxMsgsOn && (
                <input type="number" min={1} value={maxMsgs} onChange={(e) => setMaxMsgs(e.target.value)} />
              )}
              <FieldHint>{t("streamConfig.maxMsgsHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.maxMsgsPerSubject")}</span>
                <Switch
                  on={maxMsgsPerSubjectOn}
                  label={t("streamConfig.maxMsgsPerSubject")}
                  onToggle={() => setMaxMsgsPerSubjectOn((v) => !v)}
                />
              </div>
              {maxMsgsPerSubjectOn && (
                <input
                  type="number"
                  min={1}
                  value={maxMsgsPerSubject}
                  onChange={(e) => setMaxMsgsPerSubject(e.target.value)}
                />
              )}
              <FieldHint>{t("streamConfig.maxMsgsPerSubjectHint")}</FieldHint>
            </div>
          </section>

          <section className="stream-config-section">
            <h3>{t("streamConfig.placement")}</h3>
            <p>{t("streamConfig.placementLead")}</p>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.cluster")}</span>
                <Switch on={clusterOn} label={t("streamConfig.cluster")} onToggle={() => setClusterOn((v) => !v)} />
              </div>
              {clusterOn && (
                <input value={cluster} onChange={(e) => setCluster(e.target.value)} placeholder="east" />
              )}
              <FieldHint>{t("streamConfig.clusterHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.tags")}</span>
                <Switch on={tagsOn} label={t("streamConfig.tags")} onToggle={() => setTagsOn((v) => !v)} />
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
              <FieldHint>{t("streamConfig.tagsHint")}</FieldHint>
            </div>
          </section>

          <section className="stream-config-section">
            <h3>{t("streamConfig.advanced")}</h3>
            <p>{t("streamConfig.advancedLead")}</p>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.duplicates")}</span>
                <Switch
                  on={duplicatesOn}
                  label={t("streamConfig.duplicates")}
                  onToggle={() => setDuplicatesOn((v) => !v)}
                />
              </div>
              {duplicatesOn && (
                <input value={duplicates} onChange={(e) => setDuplicates(e.target.value)} placeholder="2m" />
              )}
              <FieldHint>{t("streamConfig.duplicatesHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.allowDirect")}</span>
                <Switch on={allowDirect} label={t("streamConfig.allowDirect")} onToggle={() => setAllowDirect((v) => !v)} />
              </div>
              <FieldHint>{t("streamConfig.allowDirectHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.mirrorDirect")}</span>
                <Switch
                  on={mirrorDirect}
                  label={t("streamConfig.mirrorDirect")}
                  onToggle={() => setMirrorDirect((v) => !v)}
                />
              </div>
              <FieldHint>{t("streamConfig.mirrorDirectHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.allowMsgTTL")}</span>
                <Switch on={allowMsgTTL} label={t("streamConfig.allowMsgTTL")} onToggle={() => setAllowMsgTTL((v) => !v)} />
              </div>
              <FieldHint>{t("streamConfig.allowMsgTTLHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.noAck")}</span>
                <Switch on={noAck} label={t("streamConfig.noAck")} onToggle={() => setNoAck((v) => !v)} />
              </div>
              <FieldHint>{t("streamConfig.noAckHint")}</FieldHint>
            </div>

            {mode === "edit" && (
              <div className="stream-config-limit">
                <div className="stream-config-toggle-row">
                  <span>{t("streamConfig.sealed")}</span>
                  <Switch on={sealed} label={t("streamConfig.sealed")} onToggle={() => setSealed((v) => !v)} />
                </div>
                <FieldHint>{t("streamConfig.sealedHint")}</FieldHint>
                {sealed && <p className="text-muted">{t("streamConfig.sealedWarning")}</p>}
              </div>
            )}

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.firstSeq")}</span>
                <Switch on={firstSeqOn} label={t("streamConfig.firstSeq")} onToggle={() => setFirstSeqOn((v) => !v)} />
              </div>
              {firstSeqOn && (
                <input type="number" min={1} value={firstSeq} onChange={(e) => setFirstSeq(e.target.value)} />
              )}
              <FieldHint>{t("streamConfig.firstSeqHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.markerTTL")}</span>
                <Switch on={markerTTLOn} label={t("streamConfig.markerTTL")} onToggle={() => setMarkerTTLOn((v) => !v)} />
              </div>
              {markerTTLOn && (
                <input value={markerTTL} onChange={(e) => setMarkerTTL(e.target.value)} placeholder="1h" />
              )}
              <FieldHint>{t("streamConfig.markerTTLHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.subjectTransform")}</span>
                <Switch
                  on={transformOn}
                  label={t("streamConfig.subjectTransform")}
                  onToggle={() => setTransformOn((v) => !v)}
                />
              </div>
              {transformOn && (
                <>
                  <input
                    value={transformSrc}
                    onChange={(e) => setTransformSrc(e.target.value)}
                    placeholder={t("streamConfig.transformSrc")}
                  />
                  <input
                    className="mt-8"
                    value={transformDest}
                    onChange={(e) => setTransformDest(e.target.value)}
                    placeholder={t("streamConfig.transformDest")}
                    required
                  />
                </>
              )}
              <FieldHint>{t("streamConfig.subjectTransformHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.republish")}</span>
                <Switch on={republishOn} label={t("streamConfig.republish")} onToggle={() => setRepublishOn((v) => !v)} />
              </div>
              {republishOn && (
                <>
                  <input
                    value={republishSrc}
                    onChange={(e) => setRepublishSrc(e.target.value)}
                    placeholder={t("streamConfig.republishSrc")}
                  />
                  <input
                    className="mt-8"
                    value={republishDest}
                    onChange={(e) => setRepublishDest(e.target.value)}
                    placeholder={t("streamConfig.republishDest")}
                    required
                  />
                  <div className="stream-config-toggle-row mt-8">
                    <span>{t("streamConfig.headersOnly")}</span>
                    <Switch
                      on={republishHeadersOnly}
                      label={t("streamConfig.headersOnly")}
                      onToggle={() => setRepublishHeadersOnly((v) => !v)}
                    />
                  </div>
                  <FieldHint>{t("streamConfig.headersOnlyHint")}</FieldHint>
                </>
              )}
              <FieldHint>{t("streamConfig.republishHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("streamConfig.consumerLimits")}</span>
                <Switch
                  on={consumerLimitsOn}
                  label={t("streamConfig.consumerLimits")}
                  onToggle={() => setConsumerLimitsOn((v) => !v)}
                />
              </div>
              {consumerLimitsOn && (
                <>
                  <label className="stream-config-mini-label">{t("streamConfig.inactiveThreshold")}</label>
                  <input
                    value={inactiveThreshold}
                    onChange={(e) => setInactiveThreshold(e.target.value)}
                    placeholder="1m"
                    aria-describedby="stream-cfg-inactive-hint"
                  />
                  <FieldHint id="stream-cfg-inactive-hint">{t("streamConfig.inactiveThresholdHint")}</FieldHint>
                  <label className="stream-config-mini-label">{t("streamConfig.maxAckPending")}</label>
                  <input
                    type="number"
                    min={1}
                    value={maxAckPending}
                    onChange={(e) => setMaxAckPending(e.target.value)}
                    placeholder="1000"
                    aria-describedby="stream-cfg-ackpending-hint"
                  />
                  <FieldHint id="stream-cfg-ackpending-hint">{t("streamConfig.maxAckPendingHint")}</FieldHint>
                </>
              )}
              <FieldHint>{t("streamConfig.consumerLimitsHint")}</FieldHint>
            </div>

            {!isMirror && (
            <div className="stream-config-limit">
              <label className="stream-config-mini-label">{t("streamConfig.sources")}</label>
              <p className="text-muted">{t("streamConfig.sourcesLead")}</p>
              <div className="stream-config-add-row">
                <input
                  value={sourceName}
                  onChange={(e) => setSourceName(e.target.value)}
                  placeholder={t("streamConfig.sourceName")}
                />
                <input
                  value={sourceFilter}
                  onChange={(e) => setSourceFilter(e.target.value)}
                  placeholder={t("streamConfig.sourceFilter")}
                />
                <button type="button" className="btn secondary" onClick={addSource}>
                  {t("streamConfig.add")}
                </button>
              </div>
              <FieldHint>{t("streamConfig.sourcesHint")}</FieldHint>
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
            )}

            <div className="stream-config-limit">
              <label className="stream-config-mini-label">{t("streamConfig.metadata")}</label>
              <div className="stream-config-add-row">
                <input value={metaKey} onChange={(e) => setMetaKey(e.target.value)} placeholder="key" />
                <input value={metaValue} onChange={(e) => setMetaValue(e.target.value)} placeholder="value" />
                <button type="button" className="btn secondary" onClick={addMeta}>
                  {t("streamConfig.add")}
                </button>
              </div>
              <FieldHint>{t("streamConfig.metadataHint")}</FieldHint>
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
