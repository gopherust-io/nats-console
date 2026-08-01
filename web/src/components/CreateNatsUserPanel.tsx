import { FormEvent, useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import FieldHint from "./ui/FieldHint";
import PanelError from "./ui/PanelError";

type ByteUnit = "B" | "KiB" | "MiB" | "GiB" | "TiB";
type LifetimeUnit = "Seconds" | "Minutes" | "Hours" | "Days";

const BYTE_UNIT_MULT: Record<ByteUnit, number> = {
  B: 1,
  KiB: 1024,
  MiB: 1024 ** 2,
  GiB: 1024 ** 3,
  TiB: 1024 ** 4,
};

const LIFETIME_NS: Record<LifetimeUnit, number> = {
  Seconds: 1_000_000_000,
  Minutes: 60_000_000_000,
  Hours: 3_600_000_000_000,
  Days: 86_400_000_000_000,
};

const CONNECTION_TYPES = [
  "STANDARD",
  "WEBSOCKET",
  "LEAFNODE",
  "LEAFNODE_WS",
  "MQTT",
  "MQTT_WS",
  "IN_PROCESS",
] as const;

export type NatsUserTimeRange = { start: string; end: string };

export type NatsUserConfigPayload = {
  name: string;
  signingGroup: string;
  tags: string[];
  pubAllow: string[];
  pubDeny: string[];
  subAllow: string[];
  subDeny: string[];
  allowedConnectionTypes: string[];
  srcCidrs: string[];
  timesLocale: string;
  timeRanges: NatsUserTimeRange[];
  maxSubs: number;
  maxPayload: number;
  maxData: number;
  jwtLifetimeNs: number;
  respMaxMsgs: number;
  respTTLNs: number;
  bearerToken: boolean;
  proxyRequired: boolean;
};

export type NatsUserConfigInitial = Partial<NatsUserConfigPayload> & {
  id?: string;
};

type SigningGroupOption = { id: string; name: string };

type Props = {
  mode: "create" | "edit";
  open: boolean;
  groups: SigningGroupOption[];
  initial?: NatsUserConfigInitial | null;
  busy?: boolean;
  error?: string;
  onClose: () => void;
  onSubmit: (body: NatsUserConfigPayload) => Promise<void> | void;
};

type PermKind = "allow" | "deny";
type PermSide = "pub" | "sub";

function isLimited(n: number | undefined | null): boolean {
  return typeof n === "number" && n >= 0;
}

function pickByteUnit(bytes: number): { value: string; unit: ByteUnit } {
  if (!bytes || bytes <= 0) return { value: "1", unit: "MiB" };
  const units: ByteUnit[] = ["TiB", "GiB", "MiB", "KiB", "B"];
  for (const unit of units) {
    const mult = BYTE_UNIT_MULT[unit];
    if (bytes % mult === 0 && bytes / mult >= 1) {
      return { value: String(bytes / mult), unit };
    }
  }
  return { value: String(bytes), unit: "B" };
}

function pickLifetime(ns: number): { value: string; unit: LifetimeUnit } {
  if (!ns || ns <= 0) return { value: "1", unit: "Hours" };
  const units: LifetimeUnit[] = ["Days", "Hours", "Minutes", "Seconds"];
  for (const unit of units) {
    const mult = LIFETIME_NS[unit];
    if (ns % mult === 0 && ns / mult >= 1) {
      return { value: String(ns / mult), unit };
    }
  }
  return { value: String(Math.max(1, Math.round(ns / LIFETIME_NS.Seconds))), unit: "Seconds" };
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

export default function CreateNatsUserPanel({
  mode,
  open,
  groups,
  initial,
  busy,
  error,
  onClose,
  onSubmit,
}: Props) {
  const { t } = useTranslation();
  const titleId = useId();
  const [name, setName] = useState("");
  const [signingGroup, setSigningGroup] = useState("Default");
  const [tagsOn, setTagsOn] = useState(false);
  const [tags, setTags] = useState<string[]>([]);
  const [tagDraft, setTagDraft] = useState("");
  const [pubAllow, setPubAllow] = useState<string[]>([]);
  const [pubDeny, setPubDeny] = useState<string[]>([]);
  const [subAllow, setSubAllow] = useState<string[]>([]);
  const [subDeny, setSubDeny] = useState<string[]>([]);
  const [permSide, setPermSide] = useState<PermSide | null>(null);
  const [permKind, setPermKind] = useState<PermKind>("allow");
  const [permDraft, setPermDraft] = useState("");
  const [maxSubsOn, setMaxSubsOn] = useState(false);
  const [maxSubs, setMaxSubs] = useState("100");
  const [maxPayloadOn, setMaxPayloadOn] = useState(false);
  const [maxPayloadValue, setMaxPayloadValue] = useState("1");
  const [maxPayloadUnit, setMaxPayloadUnit] = useState<ByteUnit>("MiB");
  const [maxDataOn, setMaxDataOn] = useState(false);
  const [maxDataValue, setMaxDataValue] = useState("1");
  const [maxDataUnit, setMaxDataUnit] = useState<ByteUnit>("GiB");
  const [jwtOn, setJwtOn] = useState(false);
  const [jwtValue, setJwtValue] = useState("1");
  const [jwtUnit, setJwtUnit] = useState<LifetimeUnit>("Hours");
  const [bearerToken, setBearerToken] = useState(false);
  const [proxyRequired, setProxyRequired] = useState(false);
  const [connTypesOn, setConnTypesOn] = useState(false);
  const [connTypes, setConnTypes] = useState<string[]>([]);
  const [srcCidrsOn, setSrcCidrsOn] = useState(false);
  const [srcCidrs, setSrcCidrs] = useState<string[]>([]);
  const [cidrDraft, setCidrDraft] = useState("");
  const [timesOn, setTimesOn] = useState(false);
  const [timesLocale, setTimesLocale] = useState("");
  const [timeRanges, setTimeRanges] = useState<NatsUserTimeRange[]>([]);
  const [respOn, setRespOn] = useState(false);
  const [respMaxMsgs, setRespMaxMsgs] = useState("1");
  const [respTTLValue, setRespTTLValue] = useState("5");
  const [respTTLUnit, setRespTTLUnit] = useState<LifetimeUnit>("Seconds");
  const [localError, setLocalError] = useState("");

  useEffect(() => {
    if (!open) return;
    const cfg = initial ?? {};
    setName(cfg.name ?? "");
    setSigningGroup(cfg.signingGroup || groups[0]?.name || "Default");
    setTagsOn((cfg.tags?.length ?? 0) > 0);
    setTags([...(cfg.tags ?? [])]);
    setTagDraft("");
    setPubAllow([...(cfg.pubAllow ?? [])]);
    setPubDeny([...(cfg.pubDeny ?? [])]);
    setSubAllow([...(cfg.subAllow ?? [])]);
    setSubDeny([...(cfg.subDeny ?? [])]);
    setPermSide(null);
    setPermDraft("");
    setMaxSubsOn(isLimited(cfg.maxSubs));
    setMaxSubs(String(cfg.maxSubs && cfg.maxSubs >= 0 ? cfg.maxSubs : 100));
    setMaxPayloadOn(isLimited(cfg.maxPayload));
    const payload = pickByteUnit(cfg.maxPayload && cfg.maxPayload >= 0 ? cfg.maxPayload : 1 << 20);
    setMaxPayloadValue(payload.value);
    setMaxPayloadUnit(payload.unit);
    setMaxDataOn(isLimited(cfg.maxData));
    const data = pickByteUnit(cfg.maxData && cfg.maxData >= 0 ? cfg.maxData : 1 << 30);
    setMaxDataValue(data.value);
    setMaxDataUnit(data.unit);
    setJwtOn(isLimited(cfg.jwtLifetimeNs) && (cfg.jwtLifetimeNs ?? 0) > 0);
    const life = pickLifetime(cfg.jwtLifetimeNs && cfg.jwtLifetimeNs > 0 ? cfg.jwtLifetimeNs : LIFETIME_NS.Hours);
    setJwtValue(life.value);
    setJwtUnit(life.unit);
    setBearerToken(Boolean(cfg.bearerToken));
    setProxyRequired(Boolean(cfg.proxyRequired));
    setConnTypesOn((cfg.allowedConnectionTypes?.length ?? 0) > 0);
    setConnTypes([...(cfg.allowedConnectionTypes ?? [])]);
    setSrcCidrsOn((cfg.srcCidrs?.length ?? 0) > 0);
    setSrcCidrs([...(cfg.srcCidrs ?? [])]);
    setCidrDraft("");
    const ranges = [...(cfg.timeRanges ?? [])];
    setTimesOn(Boolean(cfg.timesLocale) || ranges.length > 0);
    setTimesLocale(cfg.timesLocale ?? "");
    setTimeRanges(ranges);
    const hasResp = (cfg.respMaxMsgs ?? 0) > 0 || (cfg.respTTLNs ?? 0) > 0;
    setRespOn(hasResp);
    setRespMaxMsgs(String(cfg.respMaxMsgs && cfg.respMaxMsgs > 0 ? cfg.respMaxMsgs : 1));
    const ttl = pickLifetime(cfg.respTTLNs && cfg.respTTLNs > 0 ? cfg.respTTLNs : LIFETIME_NS.Seconds * 5);
    setRespTTLValue(ttl.value);
    setRespTTLUnit(ttl.unit);
    setLocalError("");
  }, [open, initial, groups]);

  if (!open) return null;

  function addTag() {
    const next = tagDraft.trim();
    if (!next || tags.includes(next)) return;
    setTags((prev) => [...prev, next]);
    setTagDraft("");
  }

  function addCidr() {
    const next = cidrDraft.trim();
    if (!next || srcCidrs.includes(next)) return;
    setSrcCidrs((prev) => [...prev, next]);
    setCidrDraft("");
  }

  function addPermission() {
    const subject = permDraft.trim();
    if (!subject || !permSide) return;
    const setter =
      permSide === "pub"
        ? permKind === "allow"
          ? setPubAllow
          : setPubDeny
        : permKind === "allow"
          ? setSubAllow
          : setSubDeny;
    setter((prev) => (prev.includes(subject) ? prev : [...prev, subject]));
    setPermDraft("");
    setPermSide(null);
  }

  function removePerm(side: PermSide, kind: PermKind, subject: string) {
    const setter =
      side === "pub"
        ? kind === "allow"
          ? setPubAllow
          : setPubDeny
        : kind === "allow"
          ? setSubAllow
          : setSubDeny;
    setter((prev) => prev.filter((s) => s !== subject));
  }

  function toggleConnType(ct: string) {
    setConnTypes((prev) => (prev.includes(ct) ? prev.filter((x) => x !== ct) : [...prev, ct]));
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setLocalError("");
    if (!name.trim()) {
      setLocalError(t("natsUserConfig.nameRequired"));
      return;
    }

    let maxSubsVal = -1;
    if (maxSubsOn) {
      const n = Number(maxSubs);
      if (!Number.isFinite(n) || n < 0) {
        setLocalError(t("natsUserConfig.invalidMaxSubs"));
        return;
      }
      maxSubsVal = Math.floor(n);
    }

    let maxPayloadVal = -1;
    if (maxPayloadOn) {
      const n = Number(maxPayloadValue);
      if (!Number.isFinite(n) || n <= 0) {
        setLocalError(t("natsUserConfig.invalidMaxPayload"));
        return;
      }
      maxPayloadVal = Math.round(n * BYTE_UNIT_MULT[maxPayloadUnit]);
    }

    let maxDataVal = -1;
    if (maxDataOn) {
      const n = Number(maxDataValue);
      if (!Number.isFinite(n) || n <= 0) {
        setLocalError(t("natsUserConfig.invalidMaxData"));
        return;
      }
      maxDataVal = Math.round(n * BYTE_UNIT_MULT[maxDataUnit]);
    }

    let jwtNs = 0;
    if (jwtOn) {
      const n = Number(jwtValue);
      if (!Number.isFinite(n) || n <= 0) {
        setLocalError(t("natsUserConfig.invalidJwtLifetime"));
        return;
      }
      jwtNs = Math.round(n * LIFETIME_NS[jwtUnit]);
    }

    let respMax = 0;
    let respTTL = 0;
    if (respOn) {
      const msgs = Number(respMaxMsgs);
      const ttl = Number(respTTLValue);
      if (!Number.isFinite(msgs) || msgs < 0) {
        setLocalError(t("natsUserConfig.invalidRespMax"));
        return;
      }
      if (!Number.isFinite(ttl) || ttl <= 0) {
        setLocalError(t("natsUserConfig.invalidRespTTL"));
        return;
      }
      respMax = Math.floor(msgs);
      respTTL = Math.round(ttl * LIFETIME_NS[respTTLUnit]);
    }

    const body: NatsUserConfigPayload = {
      name: name.trim(),
      signingGroup: signingGroup || "Default",
      tags: tagsOn ? tags : [],
      pubAllow,
      pubDeny,
      subAllow,
      subDeny,
      allowedConnectionTypes: connTypesOn ? connTypes : [],
      srcCidrs: srcCidrsOn ? srcCidrs : [],
      timesLocale: timesOn ? timesLocale.trim() : "",
      timeRanges: timesOn ? timeRanges.filter((tr) => tr.start || tr.end) : [],
      maxSubs: maxSubsVal,
      maxPayload: maxPayloadVal,
      maxData: maxDataVal,
      jwtLifetimeNs: jwtNs,
      respMaxMsgs: respMax,
      respTTLNs: respTTL,
      bearerToken,
      proxyRequired,
    };

    try {
      await onSubmit(body);
    } catch {
      // onSubmit surfaces errors to the panel
    }
  }

  const displayError = localError || error;

  function renderPermList(side: PermSide, kind: PermKind, items: string[]) {
    if (items.length === 0) return null;
    return (
      <ul className="stream-config-chips">
        {items.map((subject) => (
          <li key={`${side}-${kind}-${subject}`}>
            <span className="mono">
              {kind === "deny" ? "deny: " : "allow: "}
              {subject}
            </span>
            <button
              type="button"
              className="stream-config-chip-remove"
              aria-label={t("common.delete")}
              onClick={() => removePerm(side, kind, subject)}
            >
              ×
            </button>
          </li>
        ))}
      </ul>
    );
  }

  function renderPermSection(side: PermSide, title: string, unrestricted: string) {
    const allow = side === "pub" ? pubAllow : subAllow;
    const deny = side === "pub" ? pubDeny : subDeny;
    const empty = allow.length === 0 && deny.length === 0;
    const hintKey = side === "pub" ? "natsUserConfig.publishHint" : "natsUserConfig.subscribeHint";
    return (
      <div className="stream-config-limit">
        <div className="stream-config-toggle-row">
          <strong>{title}</strong>
          <div className="actions">
            <button
              type="button"
              className="btn secondary"
              onClick={() => {
                setPermSide(side);
                setPermKind("allow");
                setPermDraft("");
              }}
            >
              {t("natsUserConfig.addAllow")}
            </button>
            <button
              type="button"
              className="btn secondary"
              onClick={() => {
                setPermSide(side);
                setPermKind("deny");
                setPermDraft("");
              }}
            >
              {t("natsUserConfig.addDeny")}
            </button>
          </div>
        </div>
        <FieldHint>{t(hintKey)}</FieldHint>
        {empty ? <p className="text-muted">{unrestricted}</p> : null}
        {renderPermList(side, "allow", allow)}
        {renderPermList(side, "deny", deny)}
        {permSide === side && (
          <div className="stream-config-add-row mt-8">
            <input
              value={permDraft}
              onChange={(e) => setPermDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  addPermission();
                }
              }}
              placeholder="orders.* or telemetry.>"
              autoFocus
            />
            <button type="button" className="btn secondary" onClick={addPermission}>
              {t("streamConfig.add")}
            </button>
            <button type="button" className="btn secondary" onClick={() => setPermSide(null)}>
              {t("common.cancel")}
            </button>
          </div>
        )}
      </div>
    );
  }

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
            {mode === "edit" ? t("natsUserConfig.editTitle") : t("natsUserConfig.createTitle")}
          </h2>
          <button type="button" className="btn secondary" onClick={onClose} aria-label={t("common.close")}>
            {t("common.close")}
          </button>
        </header>

        <form className="stream-config-panel__body" onSubmit={handleSubmit}>
          <section className="stream-config-section">
            <h3>{t("natsUserConfig.nameSection")}</h3>
            <div className="nc-form-row">
              <label htmlFor="nats-user-name">{t("common.name")}</label>
              <input
                id="nats-user-name"
                required
                disabled={mode === "edit"}
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t("natsUserConfig.namePlaceholder")}
                aria-describedby="nats-user-name-hint"
              />
              <FieldHint id="nats-user-name-hint">{t("natsUserConfig.nameHint")}</FieldHint>
            </div>
            <div className="nc-form-row">
              <label htmlFor="nats-user-group">{t("natsUsers.signingGroup")}</label>
              <select
                id="nats-user-group"
                value={signingGroup}
                onChange={(e) => setSigningGroup(e.target.value)}
                aria-describedby="nats-user-group-hint"
              >
                {(groups.length ? groups : [{ id: "Default", name: "Default" }]).map((g) => (
                  <option key={g.id} value={g.name}>
                    {g.name}
                  </option>
                ))}
              </select>
              <FieldHint id="nats-user-group-hint">{t("natsUserConfig.signingGroupHint")}</FieldHint>
            </div>
            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("natsUserConfig.tags")}</span>
                <Switch on={tagsOn} label={t("natsUserConfig.tags")} onToggle={() => setTagsOn((v) => !v)} />
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
                      placeholder="team-a"
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
              <FieldHint>{t("natsUserConfig.tagsHint")}</FieldHint>
            </div>
          </section>

          <section className="stream-config-section">
            <h3>{t("natsUserConfig.permissions")}</h3>
            <p>{t("natsUserConfig.permissionsLead")}</p>
            {renderPermSection("pub", t("natsUserConfig.publish"), t("natsUserConfig.unrestrictedPublish"))}
            {renderPermSection("sub", t("natsUserConfig.subscribe"), t("natsUserConfig.unrestrictedSubscribe"))}
          </section>

          <section className="stream-config-section">
            <h3>{t("natsUserConfig.limits")}</h3>
            <p>{t("natsUserConfig.limitsLead")}</p>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("natsUserConfig.maxSubs")}</span>
                <Switch on={maxSubsOn} label={t("natsUserConfig.maxSubs")} onToggle={() => setMaxSubsOn((v) => !v)} />
              </div>
              {maxSubsOn && (
                <input type="number" min={0} value={maxSubs} onChange={(e) => setMaxSubs(e.target.value)} />
              )}
              <FieldHint>{t("natsUserConfig.maxSubsHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("natsUserConfig.maxPayload")}</span>
                <Switch
                  on={maxPayloadOn}
                  label={t("natsUserConfig.maxPayload")}
                  onToggle={() => setMaxPayloadOn((v) => !v)}
                />
              </div>
              {maxPayloadOn && (
                <div className="stream-config-unit-row">
                  <input
                    type="number"
                    min={1}
                    value={maxPayloadValue}
                    onChange={(e) => setMaxPayloadValue(e.target.value)}
                  />
                  <select value={maxPayloadUnit} onChange={(e) => setMaxPayloadUnit(e.target.value as ByteUnit)}>
                    {(["B", "KiB", "MiB", "GiB", "TiB"] as ByteUnit[]).map((u) => (
                      <option key={u} value={u}>
                        {u}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              <FieldHint>{t("natsUserConfig.maxPayloadHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("natsUserConfig.maxData")}</span>
                <Switch on={maxDataOn} label={t("natsUserConfig.maxData")} onToggle={() => setMaxDataOn((v) => !v)} />
              </div>
              {maxDataOn && (
                <div className="stream-config-unit-row">
                  <input
                    type="number"
                    min={1}
                    value={maxDataValue}
                    onChange={(e) => setMaxDataValue(e.target.value)}
                  />
                  <select value={maxDataUnit} onChange={(e) => setMaxDataUnit(e.target.value as ByteUnit)}>
                    {(["B", "KiB", "MiB", "GiB", "TiB"] as ByteUnit[]).map((u) => (
                      <option key={u} value={u}>
                        {u}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              <FieldHint>{t("natsUserConfig.maxDataHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("natsUserConfig.jwtLifetime")}</span>
                <Switch on={jwtOn} label={t("natsUserConfig.jwtLifetime")} onToggle={() => setJwtOn((v) => !v)} />
              </div>
              {jwtOn && (
                <div className="stream-config-unit-row">
                  <input type="number" min={1} value={jwtValue} onChange={(e) => setJwtValue(e.target.value)} />
                  <select value={jwtUnit} onChange={(e) => setJwtUnit(e.target.value as LifetimeUnit)}>
                    {(["Seconds", "Minutes", "Hours", "Days"] as LifetimeUnit[]).map((u) => (
                      <option key={u} value={u}>
                        {u}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              <FieldHint>{t("natsUserConfig.jwtLifetimeHint")}</FieldHint>
            </div>
          </section>

          <section className="stream-config-section">
            <h3>{t("natsUserConfig.advanced")}</h3>
            <p>{t("natsUserConfig.advancedLead")}</p>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("natsUserConfig.bearerToken")}</span>
                <Switch
                  on={bearerToken}
                  label={t("natsUserConfig.bearerToken")}
                  onToggle={() => setBearerToken((v) => !v)}
                />
              </div>
              <FieldHint>{t("natsUserConfig.bearerTokenHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("natsUserConfig.proxyRequired")}</span>
                <Switch
                  on={proxyRequired}
                  label={t("natsUserConfig.proxyRequired")}
                  onToggle={() => setProxyRequired((v) => !v)}
                />
              </div>
              <FieldHint>{t("natsUserConfig.proxyRequiredHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("natsUserConfig.connectionTypes")}</span>
                <Switch
                  on={connTypesOn}
                  label={t("natsUserConfig.connectionTypes")}
                  onToggle={() => setConnTypesOn((v) => !v)}
                />
              </div>
              {connTypesOn && (
                <div className="actions" style={{ flexWrap: "wrap", gap: "0.5rem" }}>
                  {CONNECTION_TYPES.map((ct) => (
                    <label key={ct} className="role-chip">
                      <input
                        type="checkbox"
                        checked={connTypes.includes(ct)}
                        onChange={() => toggleConnType(ct)}
                      />
                      {ct}
                    </label>
                  ))}
                </div>
              )}
              <FieldHint>{t("natsUserConfig.connectionTypesHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("natsUserConfig.srcCidrs")}</span>
                <Switch
                  on={srcCidrsOn}
                  label={t("natsUserConfig.srcCidrs")}
                  onToggle={() => setSrcCidrsOn((v) => !v)}
                />
              </div>
              {srcCidrsOn && (
                <>
                  <div className="stream-config-add-row">
                    <input
                      value={cidrDraft}
                      onChange={(e) => setCidrDraft(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          e.preventDefault();
                          addCidr();
                        }
                      }}
                      placeholder="10.0.0.0/8"
                    />
                    <button type="button" className="btn secondary" onClick={addCidr}>
                      {t("streamConfig.add")}
                    </button>
                  </div>
                  {srcCidrs.length > 0 && (
                    <ul className="stream-config-chips">
                      {srcCidrs.map((cidr) => (
                        <li key={cidr}>
                          <span className="mono">{cidr}</span>
                          <button
                            type="button"
                            className="stream-config-chip-remove"
                            aria-label={t("common.delete")}
                            onClick={() => setSrcCidrs((prev) => prev.filter((x) => x !== cidr))}
                          >
                            ×
                          </button>
                        </li>
                      ))}
                    </ul>
                  )}
                </>
              )}
              <FieldHint>{t("natsUserConfig.srcCidrsHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("natsUserConfig.timeRestrictions")}</span>
                <Switch on={timesOn} label={t("natsUserConfig.timeRestrictions")} onToggle={() => setTimesOn((v) => !v)} />
              </div>
              {timesOn && (
                <>
                  <div className="nc-form-row">
                    <label htmlFor="nats-user-locale">{t("natsUserConfig.timesLocale")}</label>
                    <input
                      id="nats-user-locale"
                      value={timesLocale}
                      onChange={(e) => setTimesLocale(e.target.value)}
                      placeholder="America/New_York"
                    />
                  </div>
                  {timeRanges.map((tr, idx) => (
                    <div key={idx} className="stream-config-unit-row">
                      <input
                        type="text"
                        value={tr.start}
                        onChange={(e) =>
                          setTimeRanges((prev) =>
                            prev.map((row, i) => (i === idx ? { ...row, start: e.target.value } : row)),
                          )
                        }
                        placeholder="09:00:00"
                      />
                      <input
                        type="text"
                        value={tr.end}
                        onChange={(e) =>
                          setTimeRanges((prev) =>
                            prev.map((row, i) => (i === idx ? { ...row, end: e.target.value } : row)),
                          )
                        }
                        placeholder="17:00:00"
                      />
                      <button
                        type="button"
                        className="btn secondary"
                        onClick={() => setTimeRanges((prev) => prev.filter((_, i) => i !== idx))}
                      >
                        ×
                      </button>
                    </div>
                  ))}
                  <button
                    type="button"
                    className="btn secondary"
                    onClick={() => setTimeRanges((prev) => [...prev, { start: "09:00:00", end: "17:00:00" }])}
                  >
                    {t("natsUserConfig.addTimeRange")}
                  </button>
                </>
              )}
              <FieldHint>{t("natsUserConfig.timeRestrictionsHint")}</FieldHint>
            </div>

            <div className="stream-config-limit">
              <div className="stream-config-toggle-row">
                <span>{t("natsUserConfig.responsePerms")}</span>
                <Switch on={respOn} label={t("natsUserConfig.responsePerms")} onToggle={() => setRespOn((v) => !v)} />
              </div>
              {respOn && (
                <>
                  <div className="nc-form-row">
                    <label>{t("natsUserConfig.respMaxMsgs")}</label>
                    <input
                      type="number"
                      min={0}
                      value={respMaxMsgs}
                      onChange={(e) => setRespMaxMsgs(e.target.value)}
                    />
                  </div>
                  <div className="stream-config-unit-row">
                    <input
                      type="number"
                      min={1}
                      value={respTTLValue}
                      onChange={(e) => setRespTTLValue(e.target.value)}
                    />
                    <select value={respTTLUnit} onChange={(e) => setRespTTLUnit(e.target.value as LifetimeUnit)}>
                      {(["Seconds", "Minutes", "Hours", "Days"] as LifetimeUnit[]).map((u) => (
                        <option key={u} value={u}>
                          {u}
                        </option>
                      ))}
                    </select>
                  </div>
                </>
              )}
              <FieldHint>{t("natsUserConfig.responsePermsHint")}</FieldHint>
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
