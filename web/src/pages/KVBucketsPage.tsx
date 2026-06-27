import { FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, clusterPath, KVBucketInfo } from "../lib/api";
import { useCluster } from "../lib/cluster";

type BucketListResponse = {
  buckets: KVBucketInfo[];
  total: number;
};

export default function KVBucketsPage() {
  const { clusterId } = useCluster();
  const [buckets, setBuckets] = useState<KVBucketInfo[]>([]);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [bucket, setBucket] = useState("");

  async function load() {
    if (!clusterId) return;
    try {
      const data = await api<BucketListResponse>(clusterPath(clusterId, "/kv/buckets"));
      setBuckets(data.buckets);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load KV buckets");
    }
  }

  useEffect(() => {
    load();
  }, [clusterId]);

  async function createBucket(event: FormEvent) {
    event.preventDefault();
    if (!clusterId) return;
    try {
      await api(clusterPath(clusterId, "/kv/buckets"), {
        method: "POST",
        body: JSON.stringify({ bucket }),
      });
      setShowForm(false);
      setBucket("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create bucket");
    }
  }

  async function deleteBucket(name: string) {
    if (!clusterId || !confirm(`Delete KV bucket "${name}"?`)) return;
    try {
      await api(clusterPath(clusterId, `/kv/buckets/${encodeURIComponent(name)}`), { method: "DELETE" });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete bucket");
    }
  }

  return (
    <div>
      <div className="page-header">
        <h1>KV Stores</h1>
        <button className="btn" onClick={() => setShowForm((v) => !v)}>
          {showForm ? "Cancel" : "Create Bucket"}
        </button>
      </div>

      {error && <div className="error">{error}</div>}

      {showForm && (
        <form className="form-grid card mb-24" onSubmit={createBucket}>
          <label>
            Bucket Name
            <input value={bucket} onChange={(e) => setBucket(e.target.value)} required />
          </label>
          <button className="btn" type="submit">
            Create
          </button>
        </form>
      )}

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Bucket</th>
              <th>Values</th>
              <th>History</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {buckets.map((b) => (
              <tr key={b.bucket}>
                <td>
                  <Link to={`/kv/${b.bucket}`}>{b.bucket}</Link>
                </td>
                <td>{b.values}</td>
                <td>{b.history}</td>
                <td>
                  <button className="btn danger" onClick={() => deleteBucket(b.bucket)}>
                    Delete
                  </button>
                </td>
              </tr>
            ))}
            {buckets.length === 0 && (
              <tr>
                <td colSpan={4} className="text-muted">
                  No KV buckets
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
