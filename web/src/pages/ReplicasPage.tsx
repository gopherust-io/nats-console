import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import RaftElectionStage from "../components/RaftElectionStage";
import ClockNumber from "../components/ui/ClockNumber";
import PageLoader from "../components/ui/PageLoader";
import QueryErrorState from "../components/ui/QueryErrorState";
import { useRaftElection } from "../hooks/useRaftElection";
import { useReplicasEvents } from "../hooks/useReplicasEvents";
import { api, clusterPath } from "../lib/api";
import { useCluster } from "../lib/cluster";
import {
  isReplicasSnapshotNewer,
  type ReplicaPeer,
  type ReplicasSnapshot,
} from "../lib/replicas";
import { clusterQueryKey, queryClient } from "../lib/query";
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
  const room = Math.max(clusterSize, incoming.length) - byName.size;
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

export default function ReplicasPage() {
  const { t } = useTranslation();
  const { clusterId: routeCluster } = useParams();
  const { clusterId: contextClusterId, cluster } = useCluster();
  const clusterId = routeCluster ?? contextClusterId;
  const [selectedPeerName, setSelectedPeerName] = useState<string | null>(null);

  // Demand-driven SSE (~2s scrape) while this page is open.
  useReplicasEvents(clusterId ?? null);

  const replicasQuery = useQuery({
    queryKey: clusterQueryKey(clusterId ?? null, "replicas"),
    queryFn: async () => {
      const incoming = (
        await api<ReplicasSnapshot>(clusterPath(clusterId!, "/replicas?fresh=1"))
      ).data;
      const prev = queryClient.getQueryData<ReplicasSnapshot>(
        clusterQueryKey(clusterId!, "replicas"),
      );
      return isReplicasSnapshotNewer(incoming, prev) ? incoming : (prev ?? incoming);
    },
    enabled: Boolean(clusterId),
    // SSE keeps the cache fresh; HTTP poll is only a cold-start / reconnect fallback.
    staleTime: 5_000,
    refetchInterval: false,
    refetchOnWindowFocus: false,
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
  // Sticky ghosts must not inflate reported cluster size.
  const clusterSize = snap?.clusterSize || snap?.peerCount || peers.length;
  const onlinePct =
    clusterSize > 0 ? Math.min(100, Math.round((onlineCount / clusterSize) * 100)) : 0;
  const onlineTone =
    clusterSize === 0 || onlineCount === 0
      ? "danger"
      : onlineCount < Math.floor(clusterSize / 2) + 1
        ? "warn"
        : "ok";
  const jsLeader = snap?.jetstreamLeader || t("common.emDash");
  const monitored = snap?.monitoredServer || t("common.emDash");

  const showError = replicasQuery.isError && !snap;
  const showContent = Boolean(snap) || (!replicasQuery.isLoading && !replicasQuery.isError);

  return (
    <div className="replicas-page">
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("replicas.title")}</h1>
          <p className="nc-page-sub">
            {t("replicas.subtitle", { name: cluster?.name ?? snap?.clusterName ?? t("systems.thisSystem") })}
          </p>
        </div>
      </div>

      {replicasQuery.isLoading && !snap && <PageLoader />}
      {showError && (
        <QueryErrorState error={replicasQuery.error} onRetry={() => void replicasQuery.refetch()} />
      )}

      {showContent && snap && (
        <>
          <div className="replicas-summary">
            <div className="replicas-card replicas-online-card">
              <span className="replicas-card__badge">{t("replicas.online")}</span>
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
              </div>
            </div>
            <div className="replicas-card replicas-stat-card">
              <span className="replicas-card__badge">{t("replicas.clusterSize")}</span>
              <div className="replicas-card__body">
                <div className="replicas-stat-card__value mono">
                  {clusterSize > 0 ? <ClockNumber value={clusterSize} /> : t("common.emDash")}
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
                      <th>{t("replicas.colStatus")}</th>
                      <th>{t("replicas.colUptime")}</th>
                      <th>{t("replicas.colRtt")}</th>
                      <th>{t("replicas.colVersion")}</th>
                      <th>{t("replicas.colConnections")}</th>
                      <th>{t("replicas.colCpu")}</th>
                      <th>{t("replicas.colMem")}</th>
                      <th>{t("replicas.colMsgs")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {peers.map((peer) => {
                      const visual = election.visualRoles[peer.name];
                      const raftRole =
                        !peer.online || visual === "offline"
                          ? null
                          : visual === "candidate" ||
                              visual === "leader" ||
                              visual === "follower" ||
                              visual === "hotStandby"
                            ? visual
                            : null;
                      return (
                        <tr
                          key={peer.name}
                          className={
                            selectedPeerName === peer.name ? "raft-peer-row--selected" : undefined
                          }
                          onClick={() =>
                            setSelectedPeerName(selectedPeerName === peer.name ? null : peer.name)
                          }
                          style={{ cursor: "pointer" }}
                        >
                          <td className="mono">{peer.name}</td>
                          <td>
                            {raftRole ? t(`replicas.election.role.${raftRole}`) : t("common.emDash")}
                          </td>
                          <td>{t(`replicas.role.${peer.role}`, { defaultValue: peer.role })}</td>
                          <td>
                            {peer.online ? t("replicas.onlineLabel") : t("replicas.offlineLabel")}
                            {peer.current === false && peer.online ? (
                              <span className="text-muted"> · {t("replicas.notCurrent")}</span>
                            ) : null}
                          </td>
                          <td className="mono">{peer.uptime || t("common.emDash")}</td>
                          <td className="mono">{peer.rtt || t("common.emDash")}</td>
                          <td className="mono">{peer.version || t("common.emDash")}</td>
                          <td className="mono">
                            {peer.connections != null ? (
                              <ClockNumber value={peer.connections} />
                            ) : (
                              t("common.emDash")
                            )}
                          </td>
                          <td className="mono">
                            {peer.cpu != null ? (
                              <>
                                <ClockNumber
                                  value={Math.round(peer.cpu * 10)}
                                  format={(n) => (n / 10).toFixed(1)}
                                />
                                %
                              </>
                            ) : (
                              t("common.emDash")
                            )}
                          </td>
                          <td className="mono">{formatMem(peer.mem)}</td>
                          <td className="mono">
                            {peer.inMsgs != null || peer.outMsgs != null ? (
                              <>
                                <ClockNumber value={peer.inMsgs ?? 0} />
                                {" / "}
                                <ClockNumber value={peer.outMsgs ?? 0} />
                              </>
                            ) : (
                              t("common.emDash")
                            )}
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
    </div>
  );
}
