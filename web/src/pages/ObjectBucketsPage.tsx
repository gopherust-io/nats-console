import { FormEvent, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import VirtualTable from "../components/VirtualTable";
import Alert from "../components/ui/Alert";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import { api, clusterPath, ObjectBucketInfo } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

type BucketListResponse = {
  buckets: ObjectBucketInfo[];
  total: number;
};

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  return `${(value / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

export default function ObjectBucketsPage() {
  const { clusterId } = useCluster();
  const { canWrite } = useAuth();
  const queryClient = useQueryClient();
  const [actionError, setActionError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [bucket, setBucket] = useState("");

  const bucketsQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "object-buckets"),
    queryFn: () => api<BucketListResponse>(clusterPath(clusterId!, "/objects/buckets")),
    enabled: Boolean(clusterId),
  });

  const buckets = bucketsQuery.data?.buckets ?? [];
  const error =
    actionError || (bucketsQuery.error instanceof Error ? bucketsQuery.error.message : "");

  async function invalidateBuckets() {
    await queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "object-buckets") });
  }

  async function createBucket(event: FormEvent) {
    event.preventDefault();
    if (!clusterId) return;
    try {
      await api(clusterPath(clusterId, "/objects/buckets"), {
        method: "POST",
        body: JSON.stringify({ bucket }),
      });
      setShowForm(false);
      setBucket("");
      setActionError("");
      await invalidateBuckets();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to create bucket");
    }
  }

  async function deleteBucket(name: string) {
    if (!clusterId || !confirm(`Delete object bucket "${name}"?`)) return;
    try {
      await api(clusterPath(clusterId, `/objects/buckets/${encodeURIComponent(name)}`), { method: "DELETE" });
      setActionError("");
      await invalidateBuckets();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to delete bucket");
    }
  }

  return (
    <div className="page">
      <PageHeader
        eyebrow="JetStream"
        title="Object Stores"
        subtitle="Store and retrieve opaque blobs — ideal for backups, artifacts, and large payloads."
        actions={
          canWrite ? (
            <button className="btn" type="button" onClick={() => setShowForm((visible) => !visible)}>
              {showForm ? "Cancel" : "Create bucket"}
            </button>
          ) : undefined
        }
      />

      <Alert variant="error">{error}</Alert>

      {showForm && canWrite && (
        <form className="form-grid card mb-24" onSubmit={createBucket}>
          <label>
            Bucket name
            <input value={bucket} onChange={(event) => setBucket(event.target.value)} required />
          </label>
          <button className="btn" type="submit">
            Create bucket
          </button>
        </form>
      )}

      {bucketsQuery.isLoading && <div className="skeleton skeleton--table" />}

      {!bucketsQuery.isLoading && buckets.length === 0 && (
        <EmptyState
          title="No object buckets yet"
          description="Create a bucket to start storing files and binary payloads in JetStream object storage."
          action={
            canWrite ? (
              <button className="btn" type="button" onClick={() => setShowForm(true)}>
                Create bucket
              </button>
            ) : undefined
          }
        />
      )}

      {!bucketsQuery.isLoading && buckets.length > 0 && (
        <div className="table-wrap">
          <VirtualTable
            columns={[
              { id: "bucket", header: "Bucket", width: "minmax(140px, 1.4fr)" },
              { id: "description", header: "Description", width: "minmax(160px, 2fr)" },
              { id: "size", header: "Size", width: "120px", align: "right", cellClassName: "mono" },
              { id: "actions", header: "", width: "112px", align: "right" },
            ]}
            items={buckets}
            getKey={(item) => item.bucket}
            renderCell={(item, columnId) => {
              switch (columnId) {
                case "bucket":
                  return <Link to={`/objects/${encodeURIComponent(item.bucket)}`}>{item.bucket}</Link>;
                case "description":
                  return item.description || "—";
                case "size":
                  return formatBytes(item.size);
                case "actions":
                  return canWrite ? (
                    <button className="btn danger btn--small" type="button" onClick={() => deleteBucket(item.bucket)}>
                      Delete
                    </button>
                  ) : null;
                default:
                  return null;
              }
            }}
          />
        </div>
      )}
    </div>
  );
}
