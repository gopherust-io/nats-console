import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import CreateKVBucketPanel, { KVBucketConfigPayload } from "../components/CreateKVBucketPanel";
import { api, clusterPath, jetStreamUIBase, KVBucketInfo } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";

type BucketListResponse = {
  buckets: KVBucketInfo[];
  total: number;
};

export default function KVBucketsPage() {
  const { t } = useTranslation();
  const { accountName } = useParams();
  const { clusterId } = useCluster();
  const { canManageJetStream } = useAuth();
  const jsBase = clusterId ? jetStreamUIBase(clusterId, accountName) : "";
  const [buckets, setBuckets] = useState<KVBucketInfo[]>([]);
  const [error, setError] = useState("");
  const [panelOpen, setPanelOpen] = useState(false);
  const [panelError, setPanelError] = useState("");
  const [panelBusy, setPanelBusy] = useState(false);

  const load = useCallback(async () => {
    if (!clusterId) return;
    try {
      const data = await api<BucketListResponse>(clusterPath(clusterId, "/kv/buckets"));
      setBuckets(data.buckets ?? []);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load KV buckets");
    }
  }, [clusterId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onCreate(body: KVBucketConfigPayload) {
    if (!clusterId) return;
    setPanelBusy(true);
    setPanelError("");
    try {
      await api(clusterPath(clusterId, "/kv/buckets"), {
        method: "POST",
        body: JSON.stringify(body),
      });
      setPanelOpen(false);
      await load();
    } catch (err) {
      setPanelError(err instanceof Error ? err.message : "Failed to create bucket");
      throw err;
    } finally {
      setPanelBusy(false);
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
        {canManageJetStream && (
          <button
            className="btn"
            type="button"
            onClick={() => {
              setPanelError("");
              setPanelOpen(true);
            }}
          >
            {t("jetstream.createKvTitle")}
          </button>
        )}
      </div>

      {error && <div className="error">{error}</div>}

      <CreateKVBucketPanel
        mode="create"
        open={panelOpen}
        busy={panelBusy}
        error={panelError}
        onClose={() => {
          setPanelOpen(false);
          setPanelError("");
        }}
        onSubmit={onCreate}
      />

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
                  <Link to={`${jsBase}/kv/${encodeURIComponent(b.bucket)}`}>{b.bucket}</Link>
                </td>
                <td>{b.values}</td>
                <td>{b.history}</td>
                <td>
                  {canManageJetStream && (
                    <button className="btn danger" type="button" onClick={() => deleteBucket(b.bucket)}>
                      Delete
                    </button>
                  )}
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
