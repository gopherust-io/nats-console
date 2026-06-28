import { useQuery } from "@tanstack/react-query";
import Alert from "../components/ui/Alert";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import StatCard from "../components/ui/StatCard";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";
import { fetchSupercluster, hasSuperclusterFeatures } from "../lib/supercluster";

function StatusBadge({ ok, label }: { ok: boolean; label: string }) {
  return (
    <span className={`badge ${ok ? "badge--live" : "badge--muted"}`}>{label}</span>
  );
}

export default function SuperclusterPage() {
  const { clusterId, cluster } = useCluster();

  const superclusterQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "supercluster"),
    queryFn: () => fetchSupercluster(clusterId!),
    enabled: Boolean(clusterId),
    refetchInterval: 30_000,
  });

  const error = superclusterQuery.error instanceof Error ? superclusterQuery.error.message : "";
  const data = superclusterQuery.data ?? null;
  const gateways = data?.gateways ?? [];
  const routes = data?.routes ?? [];
  const leafnodes = data?.leafnodes ?? [];
  const streamReplication = data?.streamReplication ?? [];
  const standalone = data && !hasSuperclusterFeatures(data);

  return (
    <div className="page">
      <PageHeader
        eyebrow="Infrastructure"
        title="Supercluster"
        subtitle="NATS server mesh (routes, gateways, leaf nodes) and JetStream meta / cross-cluster replication."
        badge={
          cluster ? (
            <span className="badge badge--live">
              {cluster.name}
              {superclusterQuery.isFetching && <span className="badge__pulse" aria-label="Refreshing" />}
            </span>
          ) : undefined
        }
        actions={
          <button
            className="btn btn--secondary"
            type="button"
            onClick={() => superclusterQuery.refetch()}
            disabled={superclusterQuery.isFetching}
          >
            Refresh
          </button>
        }
      />

      <Alert variant="error">{error}</Alert>

      {superclusterQuery.isLoading && <div className="skeleton skeleton--panel" />}

      {superclusterQuery.isError && !error && (
        <EmptyState
          title="Failed to load supercluster"
          description="Could not fetch supercluster data for this cluster. Check NATS monitoring URL and try Refresh."
        />
      )}

      {data && (
        <>
          <div className="stat-grid">
            <StatCard label="Server" value={data.serverName || "—"} accent="sky" />
            <StatCard label="Routes" value={data.routeCount ?? routes.length} accent="violet" />
            <StatCard label="Gateways" value={gateways.length} accent="emerald" />
            <StatCard label="Leaf nodes" value={data.leafCount ?? leafnodes.length} accent="amber" />
          </div>

          {standalone && (
            <EmptyState
              title="Standalone cluster"
              description="This NATS server is not part of a supercluster (no gateway/route/leaf links detected). Configure routes and gateways on your NATS servers to federate clusters, then refresh."
            />
          )}

          {(data.metaCluster || streamReplication.length > 0) && (
            <section className="panel supercluster-section">
              <h2 className="panel__title">JetStream meta cluster</h2>
              {data.metaCluster ? (
                <>
                  <p className="panel__desc">
                    Meta leader: <strong>{data.metaCluster.leader || "unknown"}</strong>
                    {data.jetstreamDomain ? ` · domain ${data.jetstreamDomain}` : ""}
                  </p>
                  {data.metaCluster.replicas && data.metaCluster.replicas.length > 0 && (
                    <div className="table-wrap">
                      <table>
                        <thead>
                          <tr>
                            <th>Server</th>
                            <th>Leader</th>
                            <th>Online</th>
                            <th>Lag</th>
                          </tr>
                        </thead>
                        <tbody>
                          {data.metaCluster.replicas.map((rep) => (
                            <tr key={rep.id || rep.name}>
                              <td className="mono">{rep.name}</td>
                              <td>{rep.leader ? "yes" : "—"}</td>
                              <td>
                                <StatusBadge ok={rep.online} label={rep.online ? "online" : "offline"} />
                              </td>
                              <td>{rep.lag ?? 0}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </>
              ) : (
                <p className="panel__desc muted">No JetStream meta cluster information returned.</p>
              )}

              {streamReplication.length > 0 && (
                <>
                  <h3 className="supercluster-section__subtitle">Stream replication</h3>
                  <div className="table-wrap">
                    <table>
                      <thead>
                        <tr>
                          <th>Stream</th>
                          <th>Kind</th>
                          <th>Target</th>
                          <th>Domain</th>
                          <th>Active</th>
                          <th>Lag</th>
                          <th>Error</th>
                        </tr>
                      </thead>
                      <tbody>
                        {streamReplication.map((row) => (
                          <tr key={`${row.streamName}-${row.kind}-${row.targetName}`}>
                            <td className="mono">{row.streamName}</td>
                            <td>{row.kind}</td>
                            <td className="mono">{row.targetName}</td>
                            <td className="mono">{row.targetDomain || "—"}</td>
                            <td>
                              <StatusBadge ok={row.active} label={row.active ? "yes" : "no"} />
                            </td>
                            <td>{row.lag ?? 0}</td>
                            <td className="muted">{row.error || "—"}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </>
              )}
            </section>
          )}

          {gateways.length > 0 && (
            <section className="panel supercluster-section">
              <h2 className="panel__title">Gateway links</h2>
              <p className="panel__desc">Inter-cluster gateway connections that form the NATS supercluster.</p>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Name</th>
                      <th>Direction</th>
                      <th>Host</th>
                      <th>Connections</th>
                      <th>Subscriptions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {gateways.map((gw) => (
                      <tr key={`${gw.direction}-${gw.name}-${gw.host}`}>
                        <td className="mono">{gw.name}</td>
                        <td>{gw.direction}</td>
                        <td className="mono">
                          {gw.host}
                          {gw.port ? `:${gw.port}` : ""}
                        </td>
                        <td>{gw.connections ?? 0}</td>
                        <td>{gw.subscriptions ?? 0}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          )}

          {routes.length > 0 && (
            <section className="panel supercluster-section">
              <h2 className="panel__title">Route mesh</h2>
              <p className="panel__desc">Intra-cluster server routes (full mesh within a NATS cluster).</p>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Remote ID</th>
                      <th>URL</th>
                      <th>Status</th>
                      <th>In msgs</th>
                      <th>Out msgs</th>
                    </tr>
                  </thead>
                  <tbody>
                    {routes.map((route) => (
                      <tr key={`${route.remoteId}-${route.url}`}>
                        <td className="mono">{route.remoteId || "—"}</td>
                        <td className="mono">{route.url || "—"}</td>
                        <td>
                          <StatusBadge ok={route.connected} label={route.connected ? "connected" : "down"} />
                        </td>
                        <td>{route.inMsgs ?? 0}</td>
                        <td>{route.outMsgs ?? 0}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          )}

          {leafnodes.length > 0 && (
            <section className="panel supercluster-section">
              <h2 className="panel__title">Leaf nodes</h2>
              <p className="panel__desc">Edge leaf node connections attached to this cluster.</p>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Name</th>
                      <th>Remote</th>
                      <th>Status</th>
                      <th>RTT</th>
                    </tr>
                  </thead>
                  <tbody>
                    {leafnodes.map((leaf) => (
                      <tr key={`${leaf.name}-${leaf.remote}`}>
                        <td className="mono">{leaf.name || "—"}</td>
                        <td className="mono">{leaf.remote || "—"}</td>
                        <td>
                          <StatusBadge ok={leaf.connected} label={leaf.connected ? "connected" : "down"} />
                        </td>
                        <td>{leaf.rtt || "—"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          )}
        </>
      )}
    </div>
  );
}
