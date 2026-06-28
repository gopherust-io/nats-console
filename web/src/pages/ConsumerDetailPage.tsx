import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, clusterPath, ConsumerInfo } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";

export default function ConsumerDetailPage() {
  const { name = "", consumer = "" } = useParams();
  const { clusterId } = useCluster();
  const { canWrite } = useAuth();
  const [info, setInfo] = useState<ConsumerInfo | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!clusterId || !name || !consumer) return;
    api<ConsumerInfo>(
      clusterPath(clusterId, `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}`),
    )
      .then(setInfo)
      .catch((err: Error) => setError(err.message));
  }, [clusterId, name, consumer]);

  async function deleteConsumer() {
    if (!clusterId || !confirm(`Delete consumer "${consumer}"?`)) return;
    try {
      await api(
        clusterPath(clusterId, `/streams/${encodeURIComponent(name)}/consumers/${encodeURIComponent(consumer)}`),
        { method: "DELETE" },
      );
      window.location.href = `/streams/${name}`;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete consumer");
    }
  }

  if (!clusterId) {
    return <p className="text-muted">Select a cluster to view this consumer.</p>;
  }

  if (!info) {
    return <div>{error || "Loading..."}</div>;
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <Link to={`/streams/${name}`} className="link-back">
            ← Back to {name}
          </Link>
          <h1>{info.name}</h1>
        </div>
        {canWrite && (
          <button className="btn danger" onClick={deleteConsumer}>
            Delete Consumer
          </button>
        )}
      </div>

      {error && <div className="error">{error}</div>}

      <div className="card-grid">
        <div className="card">
          <div className="card-label">Pending</div>
          <div className="card-value">{info.numPending}</div>
        </div>
        <div className="card">
          <div className="card-label">Ack Pending</div>
          <div className="card-value">{info.numAckPending}</div>
        </div>
        <div className="card">
          <div className="card-label">Deliver Policy</div>
          <div className="card-value card-value--sm">
            {info.config.deliverPolicy}
          </div>
        </div>
        <div className="card">
          <div className="card-label">Ack Policy</div>
          <div className="card-value card-value--sm">
            {info.config.ackPolicy}
          </div>
        </div>
      </div>

      <div className="card mt-24">
        <div className="card-label">Configuration</div>
        <pre className="mono">{JSON.stringify(info.config, null, 2)}</pre>
      </div>
    </div>
  );
}
