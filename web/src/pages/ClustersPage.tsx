import { FormEvent, useState } from "react";
import { api, Cluster } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { DEFAULT_MONITORING_URL, DEFAULT_NATS_URL } from "../lib/constants";

type TestResponse = {
  ok: boolean;
  message: string;
  serverName?: string;
  jetstream?: boolean;
};

export default function ClustersPage() {
  const { clusters, reload } = useCluster();
  const { canWrite, isAdmin } = useAuth();
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState("");
  const [natsURL, setNatsURL] = useState(DEFAULT_NATS_URL);
  const [monitoringURL, setMonitoringURL] = useState(DEFAULT_MONITORING_URL);
  const [testResults, setTestResults] = useState<Record<string, TestResponse>>({});

  async function createCluster(event: FormEvent) {
    event.preventDefault();
    try {
      await api("/api/v1/clusters", {
        method: "POST",
        body: JSON.stringify({
          name,
          natsUrl: natsURL,
          monitoringUrl: monitoringURL,
        }),
      });
      setShowForm(false);
      setName("");
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create cluster");
    }
  }

  async function deleteCluster(cluster: Cluster) {
    if (!confirm(`Delete cluster "${cluster.name}"?`)) return;
    try {
      await api(`/api/v1/clusters/${cluster.id}`, { method: "DELETE" });
      await reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete cluster");
    }
  }

  async function testCluster(cluster: Cluster) {
    try {
      const result = await api<TestResponse>(`/api/v1/clusters/${cluster.id}/test`, { method: "POST" });
      setTestResults((prev) => ({ ...prev, [cluster.id]: result }));
    } catch (err) {
      setTestResults((prev) => ({
        ...prev,
        [cluster.id]: { ok: false, message: err instanceof Error ? err.message : "test failed" },
      }));
    }
  }

  return (
    <div>
      <div className="page-header">
        <h1>Clusters</h1>
        {canWrite && (
          <button className="btn" onClick={() => setShowForm((v) => !v)}>
            {showForm ? "Cancel" : "Add Cluster"}
          </button>
        )}
      </div>

      {error && <div className="error">{error}</div>}

      {showForm && (
        <form className="form-grid card mb-24" onSubmit={createCluster}>
          <label>
            Name
            <input value={name} onChange={(e) => setName(e.target.value)} required />
          </label>
          <label>
            NATS URL
            <input value={natsURL} onChange={(e) => setNatsURL(e.target.value)} required />
          </label>
          <label>
            Monitoring URL
            <input value={monitoringURL} onChange={(e) => setMonitoringURL(e.target.value)} />
          </label>
          <button className="btn" type="submit">
            Save Cluster
          </button>
        </form>
      )}

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>NATS URL</th>
              <th>Monitoring</th>
              <th>Default</th>
              <th>Test</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {clusters.map((cluster) => (
              <tr key={cluster.id}>
                <td>{cluster.name}</td>
                <td className="mono">{cluster.natsUrl}</td>
                <td className="mono">{cluster.monitoringUrl || "-"}</td>
                <td>{cluster.isDefault ? "yes" : "no"}</td>
                <td>
                  <button className="btn secondary" onClick={() => testCluster(cluster)}>
                    Test
                  </button>
                  {testResults[cluster.id] && (
                    <span className={testResults[cluster.id].ok ? "text-success" : "text-error"} style={{ marginLeft: 8 }}>
                      {testResults[cluster.id].message}
                    </span>
                  )}
                </td>
                <td>
                  {!cluster.isDefault && isAdmin && (
                    <button className="btn danger" onClick={() => deleteCluster(cluster)}>
                      Delete
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
