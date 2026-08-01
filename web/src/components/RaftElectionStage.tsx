import { useTranslation } from "react-i18next";
import { motion, useReducedMotion } from "motion/react";
import ClockNumber from "./ui/ClockNumber";
import type { ElectionPhase, RaftVisualRole } from "../lib/raftElection";
import type { ReplicaPeer } from "../lib/replicas";

export type RaftElectionNode = {
  name: string;
  online: boolean;
};

type Props = {
  nodes: RaftElectionNode[];
  peers: ReplicaPeer[];
  visualRoles: Record<string, RaftVisualRole>;
  phase: ElectionPhase;
  caption: string;
  selectedName: string | null;
  onSelect: (name: string | null) => void;
  formatMem: (bytes?: number) => string;
};

const ROLE_CLASS: Record<RaftVisualRole, string> = {
  leader: "raft-node--leader",
  candidate: "raft-node--candidate",
  hotStandby: "raft-node--hot-standby",
  follower: "raft-node--follower",
  offline: "raft-node--offline",
};

function dash(value: string | number | undefined | null, empty: string): string {
  if (value == null || value === "") return empty;
  return String(value);
}

export default function RaftElectionStage({
  nodes,
  peers,
  visualRoles,
  phase,
  caption,
  selectedName,
  onSelect,
  formatMem,
}: Props) {
  const { t } = useTranslation();
  const reduceMotion = Boolean(useReducedMotion());
  const selected = selectedName ? peers.find((p) => p.name === selectedName) : undefined;
  // Offline is Status only — RAFT column stays empty when the peer is down.
  // Prefer stage visualRoles so candidate/overlay matches the table.
  const selectedVisual = selectedName ? visualRoles[selectedName] : undefined;
  const selectedRaftRole: RaftVisualRole | undefined =
    selected && selected.online && selectedVisual && selectedVisual !== "offline"
      ? selectedVisual
      : undefined;

  return (
    <section className="raft-election" aria-label={t("replicas.election.title")}>
      <div className="raft-election__toolbar">
        <h2 className="raft-election__title">{t("replicas.election.title")}</h2>
      </div>
      <p className="raft-election__caption" aria-live="polite">
        {caption}
      </p>
      <div className="raft-election__stage" data-phase={phase}>
        {nodes.map((node) => {
          const role = visualRoles[node.name] ?? (node.online ? "hotStandby" : "offline");
          const isSelected = selectedName === node.name;
          const animate = reduceMotion
            ? { scale: 1 }
            : role === "candidate"
              ? { scale: [1, 1.03, 1] }
              : role === "leader"
                ? { scale: 1.02 }
                : { scale: 1 };
          const transition = reduceMotion
            ? { duration: 0 }
            : role === "candidate"
              ? { duration: 0.65, repeat: Infinity, ease: "easeInOut" as const }
              : { type: "spring" as const, stiffness: 380, damping: 28 };
          return (
            <motion.button
              key={node.name}
              type="button"
              className={`raft-node ${ROLE_CLASS[role]}${isSelected ? " raft-node--selected" : ""}`}
              layout={!reduceMotion}
              animate={animate}
              transition={transition}
              aria-pressed={isSelected}
              onClick={() => onSelect(isSelected ? null : node.name)}
            >
              <span className="raft-node__shell">
                <span className={`raft-node__badge raft-node__badge--${role}`}>
                  <span className={`raft-node__dot raft-node__dot--${role}`} aria-hidden="true" />
                  {t(`replicas.election.role.${role}`)}
                </span>
                <span className="raft-node__name mono">{node.name}</span>
              </span>
            </motion.button>
          );
        })}
      </div>

      {selected ? (
        <div className="raft-peer-detail" aria-live="polite">
          <div className="raft-peer-detail__header">
            <h3 className="raft-peer-detail__title">
              {t("replicas.detailTitle", { name: selected.name })}
            </h3>
            <button
              type="button"
              className="btn btn--ghost btn--small"
              onClick={() => onSelect(null)}
            >
              {t("replicas.detailClose")}
            </button>
          </div>
          <dl className="raft-peer-detail__grid">
            <div>
              <dt>{t("replicas.colName")}</dt>
              <dd className="mono">{selected.name}</dd>
            </div>
            <div>
              <dt>{t("replicas.colRaft")}</dt>
              <dd>
                {selectedRaftRole
                  ? t(`replicas.election.role.${selectedRaftRole}`)
                  : t("common.emDash")}
              </dd>
            </div>
            <div>
              <dt>{t("replicas.colRole")}</dt>
              <dd>{t(`replicas.role.${selected.role}`, { defaultValue: selected.role })}</dd>
            </div>
            <div>
              <dt>{t("replicas.colStatus")}</dt>
              <dd>
                {selected.online ? t("replicas.onlineLabel") : t("replicas.offlineLabel")}
                {selected.current === false && selected.online
                  ? ` · ${t("replicas.notCurrent")}`
                  : ""}
              </dd>
            </div>
            <div>
              <dt>{t("replicas.colUptime")}</dt>
              <dd className="mono">{dash(selected.uptime, t("common.emDash"))}</dd>
            </div>
            <div>
              <dt>{t("replicas.colRtt")}</dt>
              <dd className="mono">{dash(selected.rtt, t("common.emDash"))}</dd>
            </div>
            <div>
              <dt>{t("replicas.colIdle")}</dt>
              <dd className="mono">{dash(selected.idle, t("common.emDash"))}</dd>
            </div>
            <div>
              <dt>{t("replicas.colVersion")}</dt>
              <dd className="mono">{dash(selected.version, t("common.emDash"))}</dd>
            </div>
            <div>
              <dt>{t("replicas.colIp")}</dt>
              <dd className="mono">{dash(selected.ip, t("common.emDash"))}</dd>
            </div>
            <div>
              <dt>{t("replicas.colConnections")}</dt>
              <dd className="mono">
                {selected.connections != null ? (
                  <ClockNumber value={selected.connections} />
                ) : (
                  t("common.emDash")
                )}
              </dd>
            </div>
            <div>
              <dt>{t("replicas.colCpu")}</dt>
              <dd className="mono">
                {selected.cpu != null ? (
                  <>
                    <ClockNumber
                      value={Math.round(selected.cpu * 10)}
                      format={(n) => (n / 10).toFixed(1)}
                    />
                    %
                  </>
                ) : (
                  t("common.emDash")
                )}
              </dd>
            </div>
            <div>
              <dt>{t("replicas.colMem")}</dt>
              <dd className="mono">{formatMem(selected.mem)}</dd>
            </div>
            <div>
              <dt>{t("replicas.colMsgs")}</dt>
              <dd className="mono">
                {selected.inMsgs != null || selected.outMsgs != null ? (
                  <>
                    <ClockNumber value={selected.inMsgs ?? 0} />
                    {" / "}
                    <ClockNumber value={selected.outMsgs ?? 0} />
                  </>
                ) : (
                  t("common.emDash")
                )}
              </dd>
            </div>
            <div>
              <dt>{t("replicas.colPending")}</dt>
              <dd className="mono">
                {selected.pending != null ? (
                  <ClockNumber value={selected.pending} />
                ) : (
                  t("common.emDash")
                )}
              </dd>
            </div>
          </dl>
        </div>
      ) : null}
    </section>
  );
}
