import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import RaftElectionStage from "../components/RaftElectionStage";
import Alert from "../components/ui/Alert";
import ClockNumber from "../components/ui/ClockNumber";
import PageLoader from "../components/ui/PageLoader";
import QueryErrorState from "../components/ui/QueryErrorState";
import { useRaftElection } from "../hooks/useRaftElection";
import { useReplicasEvents } from "../hooks/useReplicasEvents";
import { api, clusterPath } from "../lib/api";
import { useCluster } from "../lib/cluster";
import {
  isExpectedMetricNa,
  isReplicasSnapshotNewer,
  monitoringRaftRole,
  type ReplicaPeer,
  type ReplicasSnapshot,
} from "../lib/replicas";
import { clusterQueryKey, queryClient, visibilityAwareInterval } from "../lib/query";
import "../styles/replicas.css";

export type { ReplicaPeer, ReplicasSnapshot } from "../lib/replicas";

function formatMem(bytes?: number) {
  if (bytes == null || Number.isNaN(bytes)) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GiB`;
}

function asOfflineSticky(old: ReplicaPeer): ReplicaPeer {
  return {
    name: old.name,
    role: old.role === "monitored" ? "route" : old.role,
    online: false,
    leader: false,
    current: false,
  };
}

/** Keep recently-seen peers as offline when a scrape drops them (routez omits dead nodes). */
function mergeStickyPeers(
  incoming: ReplicaPeer[],
  prev: ReplicaPeer[],
  jetstreamLeader: string | undefined,
  clusterSize: number,
): ReplicaPeer[] {
  const byName = new Map(incoming.map((p) => [p.name, p]));
  if (jetstreamLeader && !byName.has(jetstreamLeader)) {
    byName.set(jetstreamLeader, {
      name: jetstreamLeader,
      role: "meta",
      online: false,
      leader: true,
    });
  }
  // Keep prior peers as offline sticky even when meta clusterSize briefly shrinks.
  const room = Math.max(clusterSize, incoming.length, prev.length) - byName.size;
  if (room > 0) {
    let added = 0;
    for (const old of prev) {
      if (added >= room) break;
      if (byName.has(old.name)) continue;
      byName.set(old.name, asOfflineSticky(old));
      added += 1;
    }
  }
  return [...byName.values()].sort((a, b) => {
    if (a.leader !== b.leader) return a.leader ? -1 : 1;
    if (a.role === "monitored" && b.role !== "monitored") return -1;
    if (b.role === "monitored" && a.role !== "monitored") return 1;
    return a.name.localeCompare(b.name);
  });
}

/** Table status tone: ok = online, warn = lagging, danger = offline. */
function peerStatusTone(peer: ReplicaPeer): "ok" | "warn" | "danger" {
  if (!peer.online) return "danger";
  if (peer.current === false) return "warn";
  return "ok";
}

function peerStatusLabel(
  peer: ReplicaPeer,
  t: (key: string, opts?: Record<string, unknown>) => string,
): string {
  if (!peer.online) return t("replicas.offlineLabel");
  if (peer.current === false) {
    return peer.lag != null
      ? t("replicas.laggingWithLag", { lag: peer.lag })
      : t("replicas.lagging");
  }
  return t("replicas.onlineLabel");
}

type MetricCellProps = {
  peer: ReplicaPeer;
  scope: "routeLink" | "varzHealth";
  hasValue: boolean;
  children: ReactNode;
  naTitle: string;
  empty: string;
};

function MetricCell({ peer, scope, hasValue, children, naTitle, empty }: MetricCellProps) {
  if (hasValue) return <>{children}</>;
  if (isExpectedMetricNa(peer, scope)) {
    return (
      <span className="replicas-na" title={naTitle}>
        n/a
      </span>
    );
  }
  return <>{empty}</>;
}

export default function ReplicasPage() {
  const { t } = useTranslation();
  const { clusterId: routeCluster } = useParams();
  const { clusterId: contextClusterId } = useCluster();
  const clusterId = routeCluster ?? contextClusterId;
  const [selectedPeerName, setSelectedPeerName] = useState<string | null>(null);

  const { live } = useReplicasEvents(clusterId ?? null);

  const replicasQuery = useQuery({
    queryKey: clusterQueryKey(clusterId ?? null, "replicas"),
    queryFn: async () => {
      const incoming = (
        await api<ReplicasSnapshot>(clusterPath(clusterId!, "/replicas"))
      ).data;
      const prev = queryClient.getQueryData<ReplicasSnapshot>(
        clusterQueryKey(clusterId!, "replicas"),
      );
      return isReplicasSnapshotNewer(incoming, prev) ? incoming : (prev ?? incoming);
    },
    enabled: Boolean(clusterId),
    staleTime: 5_000,
    // While SSE is down, poll REST so online/offline does not freeze on the last frame.
    refetchInterval: live ? false : visibilityAwareInterval(5_000),
    refetchOnWindowFocus: !live,
  });

  const snap = replicasQuery.data;
  const stickyRef = useRef<ReplicaPeer[]>([]);
  const stickyClusterRef = useRef<string | null>(null);

  useEffect(() => {
    stickyRef.current = [];
    stickyClusterRef.current = clusterId ?? null;
    setSelectedPeerName(null);
  }, [clusterId]);

  const peers = useMemo(() => {
    if (stickyClusterRef.current !== (clusterId ?? null)) {
      stickyRef.current = [];
      stickyClusterRef.current = clusterId ?? null;
    }
    const merged = mergeStickyPeers(
      snap?.peers ?? [],
      stickyRef.current,
      snap?.jetstreamLeader,
      snap?.clusterSize ?? 0,
    );
    stickyRef.current = merged;
    return merged;
  }, [snap, clusterId]);

  useEffect(() => {
    if (selectedPeerName && !peers.some((p) => p.name === selectedPeerName)) {
      setSelectedPeerName(null);
    }
  }, [peers, selectedPeerName]);

  useEffect(() => {
    if (!selectedPeerName) return;
    const onKey = (ev: KeyboardEvent) => {
      if (ev.key === "Escape") setSelectedPeerName(null);
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [selectedPeerName]);

  const electionPeers = useMemo(
    () =>
      peers.map((p) => ({
        name: p.name,
        online: p.online,
        leader: p.leader,
        current: p.current,
      })),
    [peers],
  );

  const election = useRaftElection(electionPeers, snap?.jetstreamLeader, clusterId);

  const caption = t(election.captionKey, {
    from: election.captionParams.from || t("common.emDash"),
    to: election.captionParams.to || t("common.emDash"),
    candidate: election.captionParams.candidate || t("common.emDash"),
  });

  const onlineCount = peers.filter((p) => p.online).length;
  const clusterSize = snap?.clusterSize || snap?.peerCount || peers.length;
  const quorum = clusterSize > 0 ? Math.floor(clusterSize / 2) + 1 : 0;
  const onlinePct =
    clusterSize > 0 ? Math.min(100, Math.round((onlineCount / clusterSize) * 100)) : 0;
  const onlineTone =
    clusterSize === 0 || onlineCount === 0
      ? "danger"
      : onlineCount < quorum
        ? "warn"
        : "ok";
  const jsLeader = snap?.jetstreamLeader || t("common.emDash");
  const monitored = snap?.monitoredServer || t("common.emDash");
  const emDash = t("common.emDash");
  const naMonitored = t("replicas.naMonitored");
  const naRoute = t("replicas.naRoute");

  const showError = replicasQuery.isError && !snap;
  const showContent = Boolean(snap) || (!replicasQuery.isLoading && !replicasQuery.isError);

  const staleAt =
    snap?.capturedAt && !Number.isNaN(Date.parse(snap.capturedAt))
      ? new Date(snap.capturedAt).toLocaleString()
      : null;

  return (
    <div className="replicas-page">
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("replicas.title")}</h1>
        </div>
      </div>

      {replicasQuery.isLoading && !snap && <PageLoader />}
      {showError && (
        <QueryErrorState error={replicasQuery.error} onRetry={() => void replicasQuery.refetch()} />
      )}

      {showContent && snap && !live && (
        <Alert variant="info">
          {t("replicas.staleSnapshot")}
          {staleAt ? ` ${t("replicas.staleSnapshotAt", { time: staleAt })}` : null}
        </Alert>
      )}

      {showContent && snap && (
        <>
          <div className="replicas-summary">
            <div className="replicas-card replicas-online-card">
              <span
                className={`replicas-card__badge${onlineCount === 0 ? " replicas-card__badge--down" : ""}`}
              >
                {onlineCount === 0 ? t("replicas.down") : t("replicas.online")}
              </span>
              <div className="replicas-card__body">
                <div
                  className={`replicas-gauge replicas-gauge--${onlineTone}`}
                  style={{ ["--gauge-pct" as string]: onlinePct }}
                  role="img"
                  aria-label={t("replicas.onlineGaugeAria", {
                    online: onlineCount,
                    size: clusterSize || 0,
                    defaultValue: `${onlineCount} / ${clusterSize || 0}`,
                  })}
                >
                  <span className="replicas-gauge__value mono">
                    <ClockNumber value={onlineCount} />
                    <span className="replicas-gauge__sep">/</span>
                    <ClockNumber value={clusterSize || onlineCount} />
                  </span>
                </div>
                {clusterSize > 0 ? (
                  <p className="replicas-online-card__quorum mono">
                    {t("replicas.quorumLine", {
                      online: onlineCount,
                      size: clusterSize,
                      quorum,
                    })}
                  </p>
                ) : null}
              </div>
            </div>
            <div className="replicas-card replicas-stat-card">
              <span className="replicas-card__badge">{t("replicas.clusterSize")}</span>
              <div className="replicas-card__body">
                <div className="replicas-stat-card__value mono">
                  {clusterSize > 0 ? <ClockNumber value={clusterSize} /> : emDash}
                </div>
              </div>
            </div>
            <div className="replicas-card replicas-stat-card">
              <span className="replicas-card__badge">{t("replicas.jsLeader")}</span>
              <div className="replicas-card__body">
                <div className="replicas-stat-card__value mono">{jsLeader}</div>
              </div>
            </div>
            <div className="replicas-card replicas-stat-card">
              <span className="replicas-card__badge">{t("replicas.monitored")}</span>
              <div className="replicas-card__body">
                <div className="replicas-stat-card__value mono">{monitored}</div>
              </div>
            </div>
          </div>

          {peers.length > 0 && (
            <RaftElectionStage
              nodes={electionPeers}
              peers={peers}
              visualRoles={election.visualRoles}
              phase={election.phase}
              caption={caption}
              selectedName={selectedPeerName}
              onSelect={setSelectedPeerName}
              formatMem={formatMem}
            />
          )}

          {peers.length === 0 ? (
            <p className="text-muted">{t("replicas.empty")}</p>
          ) : (
            <div className="replicas-panel replicas-table-panel">
              <div className="replicas-panel__head">
                <h2 className="replicas-panel__title">{t("replicas.peersTitle")}</h2>
              </div>
              <div className="table-wrap replicas-table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>{t("replicas.colName")}</th>
                      <th>{t("replicas.colRaft")}</th>
                      <th>{t("replicas.colRole")}</th>
                      <th>{t("replicas.colUptime")}</th>
                      <th>{t("replicas.colRtt")}</th>
                      <th>{t("replicas.colIdle")}</th>
                      <th>{t("replicas.colPending")}</th>
                      <th>{t("replicas.colConnections")}</th>
                      <th>{t("replicas.colCpu")}</th>
                      <th>{t("replicas.colMem")}</th>
                      <th>{t("replicas.colMsgs")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {peers.map((peer) => {
                      const raftRole = monitoringRaftRole(peer, snap.jetstreamLeader);
                      const hasMsgs = peer.inMsgs != null || peer.outMsgs != null;
                      return (
                        <tr key={peer.name}>
                          <td className="mono">
                            <span className="replicas-peer-name">
                              <span
                                className={`replicas-status-dot replicas-status-dot--${peerStatusTone(peer)}`}
                                title={peerStatusLabel(peer, t)}
                                aria-label={peerStatusLabel(peer, t)}
                              />
                              {peer.name}
                            </span>
                          </td>
                          <td>
                            {raftRole
                              ? t(`replicas.election.role.${raftRole}`)
                              : emDash}
                          </td>
                          <td>{t(`replicas.role.${peer.role}`, { defaultValue: peer.role })}</td>
                          <td className="mono">{peer.uptime || emDash}</td>
                          <td className="mono">
                            <MetricCell
                              peer={peer}
                              scope="routeLink"
                              hasValue={Boolean(peer.rtt)}
                              naTitle={naMonitored}
                              empty={emDash}
                            >
                              {peer.rtt}
                            </MetricCell>
                          </td>
                          <td className="mono">
                            <MetricCell
                              peer={peer}
                              scope="routeLink"
                              hasValue={Boolean(peer.idle)}
                              naTitle={naMonitored}
                              empty={emDash}
                            >
                              {peer.idle}
                            </MetricCell>
                          </td>
                          <td className="mono">
                            <MetricCell
                              peer={peer}
                              scope="routeLink"
                              hasValue={peer.pending != null}
                              naTitle={naMonitored}
                              empty={emDash}
                            >
                              <ClockNumber value={peer.pending!} />
                            </MetricCell>
                          </td>
                          <td className="mono">
                            <MetricCell
                              peer={peer}
                              scope="varzHealth"
                              hasValue={peer.connections != null}
                              naTitle={naRoute}
                              empty={emDash}
                            >
                              <ClockNumber value={peer.connections!} />
                            </MetricCell>
                          </td>
                          <td className="mono">
                            <MetricCell
                              peer={peer}
                              scope="varzHealth"
                              hasValue={peer.cpu != null}
                              naTitle={naRoute}
                              empty={emDash}
                            >
                              <>
                                <ClockNumber
                                  value={Math.round(peer.cpu! * 10)}
                                  format={(n) => (n / 10).toFixed(1)}
                                />
                                %
                              </>
                            </MetricCell>
                          </td>
                          <td className="mono">
                            <MetricCell
                              peer={peer}
                              scope="varzHealth"
                              hasValue={peer.mem != null}
                              naTitle={naRoute}
                              empty={emDash}
                            >
                              {formatMem(peer.mem)}
                            </MetricCell>
                          </td>
                          <td className="mono">
                            <MetricCell
                              peer={peer}
                              scope="routeLink"
                              hasValue={hasMsgs}
                              naTitle={naMonitored}
                              empty={emDash}
                            >
                              <>
                                <ClockNumber value={peer.inMsgs ?? 0} />
                                {" / "}
                                <ClockNumber value={peer.outMsgs ?? 0} />
                              </>
                            </MetricCell>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}

      <footer className="replicas-terms-wrap">
        <p className="replicas-terms__eyebrow">{t("replicas.terms.title")}</p>
        <dl className="replicas-terms" aria-label={t("replicas.terms.title")}>
          {(
            [
              ["leader", "leaderDef"],
              ["follower", "followerDef"],
              ["candidate", "candidateDef"],
              ["route", "routeDef"],
              ["monitored", "monitoredDef"],
              ["raft", "raftDef"],
              ["rtt", "rttDef"],
              ["conns", "connsDef"],
              ["idle", "idleDef"],
              ["pending", "pendingDef"],
            ] as const
          ).map(([term, def]) => (
            <div key={term} className={`replicas-terms__item replicas-terms__item--${term}`}>
              <dt>{t(`replicas.terms.${term}`)}</dt>
              <dd>{t(`replicas.terms.${def}`)}</dd>
            </div>
          ))}
        </dl>
      </footer>
    </div>
  );
}
