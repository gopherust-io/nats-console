import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, clusterPath, decodeBase64, ObjectInfo, tryParseJSON } from "../lib/api";
import { useCluster } from "../lib/cluster";

type ObjectListResponse = {
  objects: string[];
  total: number;
};

export default function ObjectBucketPage() {
  const { bucket = "" } = useParams();
  const { clusterId } = useCluster();
  const [objects, setObjects] = useState<string[]>([]);
  const [selected, setSelected] = useState<ObjectInfo | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!clusterId || !bucket) return;
    api<ObjectListResponse>(clusterPath(clusterId, `/objects/buckets/${encodeURIComponent(bucket)}/objects`))
      .then((data) => setObjects(data.objects))
      .catch((err: Error) => setError(err.message));
  }, [clusterId, bucket]);

  async function loadObject(name: string) {
    if (!clusterId) return;
    try {
      const info = await api<ObjectInfo>(
        clusterPath(clusterId, `/objects/buckets/${encodeURIComponent(bucket)}/objects/${encodeURIComponent(name)}`),
      );
      setSelected(info);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load object");
    }
  }

  const payload = selected ? decodeBase64(selected.data) : "";
  const parsed = tryParseJSON(payload);

  return (
    <div>
      <div className="page-header">
        <div>
          <Link to="/objects" className="link-back">
            ← Back to Object Stores
          </Link>
          <h1>{bucket}</h1>
        </div>
      </div>

      {error && <div className="error">{error}</div>}

      <div className="split-view">
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Object</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {objects.map((name) => (
                <tr key={name}>
                  <td>{name}</td>
                  <td>
                    <button className="btn secondary" onClick={() => loadObject(name)}>
                      View
                    </button>
                  </td>
                </tr>
              ))}
              {objects.length === 0 && (
                <tr>
                  <td colSpan={2} className="text-muted">
                    No objects
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {selected && (
          <div className="card">
            <div className="card-label">
              {selected.name} · {selected.size} bytes · {selected.modified}
            </div>
            <pre className="mono">{parsed.isJSON ? JSON.stringify(parsed.parsed, null, 2) : payload}</pre>
          </div>
        )}
      </div>
    </div>
  );
}
