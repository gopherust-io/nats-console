import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, clusterPath } from "../lib/api";
import { useCluster } from "../lib/cluster";

type KeyListResponse = {
  keys: string[];
  total: number;
};

export default function KVBucketPage() {
  const { bucket = "" } = useParams();
  const { clusterId } = useCluster();
  const [keys, setKeys] = useState<string[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!clusterId || !bucket) return;
    api<KeyListResponse>(clusterPath(clusterId, `/kv/buckets/${encodeURIComponent(bucket)}/keys`))
      .then((data) => setKeys(data.keys))
      .catch((err: Error) => setError(err.message));
  }, [clusterId, bucket]);

  return (
    <div>
      <div className="page-header">
        <div>
          <Link to="/kv" className="link-back">
            ← Back to KV Stores
          </Link>
          <h1>{bucket}</h1>
        </div>
      </div>

      {error && <div className="error">{error}</div>}

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Key</th>
            </tr>
          </thead>
          <tbody>
            {keys.map((key) => (
              <tr key={key}>
                <td>
                  <Link to={`/kv/${bucket}/${encodeURIComponent(key)}`}>{key}</Link>
                </td>
              </tr>
            ))}
            {keys.length === 0 && (
              <tr>
                <td className="text-muted">No keys</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
