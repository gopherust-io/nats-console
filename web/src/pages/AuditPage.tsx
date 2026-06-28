import { FormEvent, useEffect, useState } from "react";
import { api, AuditEntry } from "../lib/api";
import { useCluster } from "../lib/cluster";

export default function AuditPage() {
  const { clusterId } = useCluster();
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState("");
  const [filterCluster, setFilterCluster] = useState("");
  const [expanded, setExpanded] = useState<string | null>(null);

  async function load() {
    setError("");
    try {
      const params = new URLSearchParams({ limit: "100" });
      const cluster = filterCluster || clusterId || "";
      if (cluster) params.set("cluster_id", cluster);
      const data = await api<{ entries: AuditEntry[]; total: number }>(`/api/v1/audit?${params}`);
      setEntries(data.entries);
      setTotal(data.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load audit log");
    }
  }

  useEffect(() => {
    setFilterCluster(clusterId ?? "");
  }, [clusterId]);

  useEffect(() => {
    load();
  }, [filterCluster]);

  function onFilter(event: FormEvent) {
    event.preventDefault();
    load();
  }

  return (
    <div>
      <div className="page-header">
        <h1>Audit Log</h1>
      </div>

      {error && <div className="error">{error}</div>}

      <form className="form-grid card mb-24" onSubmit={onFilter} style={{ gridTemplateColumns: "1fr auto" }}>
        <label>
          Cluster ID filter
          <input value={filterCluster} onChange={(e) => setFilterCluster(e.target.value)} placeholder="Optional cluster UUID" />
        </label>
        <button className="btn" type="submit" style={{ alignSelf: "end" }}>
          Refresh
        </button>
      </form>

      <div className="card">
        <div className="muted mb-16">{total} entries</div>
        <table className="table">
          <thead>
            <tr>
              <th>Time</th>
              <th>Actor</th>
              <th>Action</th>
              <th>Cluster</th>
              <th>Resource</th>
              <th>IP</th>
              <th>Details</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((entry) => (
              <tr key={entry.id}>
                <td>{new Date(entry.timestamp).toLocaleString()}</td>
                <td>{entry.actor || "—"}</td>
                <td>{entry.action}</td>
                <td className="mono">{entry.cluster_id ? entry.cluster_id.slice(0, 8) : "—"}</td>
                <td>
                  {entry.resource_type}
                  {entry.resource_name ? ` / ${entry.resource_name}` : ""}
                </td>
                <td>{entry.ip || "—"}</td>
                <td>
                  <button className="btn btn--secondary btn--small" type="button" onClick={() => setExpanded(expanded === entry.id ? null : entry.id)}>
                    {expanded === entry.id ? "Hide" : "Show"}
                  </button>
                  {expanded === entry.id && (
                    <pre className="code-block mt-8">{JSON.stringify(entry.details, null, 2)}</pre>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {entries.length === 0 && <div className="muted">No audit entries yet.</div>}
      </div>
    </div>
  );
}
