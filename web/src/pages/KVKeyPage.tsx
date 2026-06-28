import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, clusterPath, decodeBase64, KVEntry, tryParseJSON } from "../lib/api";
import { useCluster } from "../lib/cluster";

type HistoryResponse = {
  entries: KVEntry[];
  total: number;
};

export default function KVKeyPage() {
  const { bucket = "", key = "" } = useParams();
  const decodedKey = decodeURIComponent(key);
  const { clusterId } = useCluster();
  const [entry, setEntry] = useState<KVEntry | null>(null);
  const [history, setHistory] = useState<KVEntry[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!clusterId || !bucket || !decodedKey) return;
    Promise.all([
      api<KVEntry>(clusterPath(clusterId, `/kv/buckets/${encodeURIComponent(bucket)}/keys/${encodeURIComponent(decodedKey)}`)),
      api<HistoryResponse>(
        clusterPath(clusterId, `/kv/buckets/${encodeURIComponent(bucket)}/keys/${encodeURIComponent(decodedKey)}/history`),
      ),
    ])
      .then(([entryData, historyData]) => {
        setEntry(entryData);
        setHistory(historyData.entries ?? []);
      })
      .catch((err: Error) => setError(err.message));
  }, [clusterId, bucket, decodedKey]);

  if (!clusterId) {
    return <p className="text-muted">Select a cluster to view this key.</p>;
  }

  if (!entry) {
    return <div>{error || "Loading..."}</div>;
  }

  const payload = decodeBase64(entry.value);
  const parsed = tryParseJSON(payload);

  return (
    <div>
      <div className="page-header">
        <div>
          <Link to={`/kv/${bucket}`} className="link-back">
            ← Back to {bucket}
          </Link>
          <h1>{decodedKey}</h1>
        </div>
      </div>

      {error && <div className="error">{error}</div>}

      <div className="card">
        <div className="card-label">
          Revision {entry.revision} · {entry.created}
        </div>
        <pre className="mono">{parsed.isJSON ? JSON.stringify(parsed.parsed, null, 2) : payload}</pre>
      </div>

      {history.length > 1 && (
        <>
          <h2 className="mt-24">History</h2>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Revision</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                {history.map((h) => (
                  <tr key={h.revision}>
                    <td>{h.revision}</td>
                    <td>{h.created}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
