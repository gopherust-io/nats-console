import { useEffect, useState } from "react";
import { AccountInfo, api, clusterPath } from "../lib/api";
import { useCluster } from "../lib/cluster";

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  return `${(value / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

type JSZResponse = {
  account_details?: Array<{
    stream_count?: number;
    consumer_count?: number;
    storage?: number;
    memory?: number;
  }>;
  total?: {
    streams?: number;
    consumers?: number;
    messages?: number;
  };
};

export default function DashboardPage() {
  const { clusterId, cluster } = useCluster();
  const [account, setAccount] = useState<AccountInfo | null>(null);
  const [varz, setVarz] = useState<Record<string, unknown> | null>(null);
  const [jsz, setJsz] = useState<JSZResponse | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!clusterId) return;
    Promise.all([
      api<AccountInfo>(clusterPath(clusterId, "/account")),
      api<Record<string, unknown>>(clusterPath(clusterId, "/monitoring/varz")),
      api<JSZResponse>(clusterPath(clusterId, "/monitoring/jsz?streams=1&consumers=1")),
    ])
      .then(([accountInfo, varzInfo, jszInfo]) => {
        setAccount(accountInfo);
        setVarz(varzInfo);
        setJsz(jszInfo);
        setError("");
      })
      .catch((err: Error) => setError(err.message));
  }, [clusterId]);

  return (
    <div>
      <div className="page-header">
        <h1>Dashboard</h1>
        {cluster && <span className="badge">{cluster.name}</span>}
      </div>

      {error && <div className="error">{error}</div>}

      {account && (
        <div className="card-grid">
          <div className="card">
            <div className="card-label">Streams</div>
            <div className="card-value">{account.streams}</div>
          </div>
          <div className="card">
            <div className="card-label">Consumers</div>
            <div className="card-value">{account.consumers}</div>
          </div>
          <div className="card">
            <div className="card-label">Storage Used</div>
            <div className="card-value">{formatBytes(account.storage)}</div>
          </div>
          <div className="card">
            <div className="card-label">Memory Used</div>
            <div className="card-value">{formatBytes(account.memory)}</div>
          </div>
        </div>
      )}

      {jsz?.total && (
        <div className="card-grid mt-16">
          <div className="card">
            <div className="card-label">JSZ Streams</div>
            <div className="card-value">{jsz.total.streams ?? 0}</div>
          </div>
          <div className="card">
            <div className="card-label">JSZ Consumers</div>
            <div className="card-value">{jsz.total.consumers ?? 0}</div>
          </div>
          <div className="card">
            <div className="card-label">JSZ Messages</div>
            <div className="card-value">{jsz.total.messages ?? 0}</div>
          </div>
        </div>
      )}

      {varz && (
        <div className="card mt-24">
          <div className="card-label">Server</div>
          <div>{String(varz.server_name ?? "unknown")}</div>
          <div className="text-muted" style={{ marginTop: 8 }}>
            version {String(varz.version ?? "-")} · uptime {String(varz.uptime ?? "-")}
          </div>
        </div>
      )}
    </div>
  );
}
