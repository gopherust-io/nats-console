import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import CreateObjectBucketPanel, { ObjectBucketConfigPayload } from "../components/CreateObjectBucketPanel";
import VirtualTable from "../components/VirtualTable";
import Alert from "../components/ui/Alert";
import EmptyState from "../components/ui/EmptyState";
import PageHeader from "../components/ui/PageHeader";
import { api, clusterPath, jetStreamUIBase, ObjectBucketInfo } from "../lib/api";
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
  const { t } = useTranslation();
  const { accountName } = useParams();
  const { clusterId } = useCluster();
  const { canManageJetStream } = useAuth();
  const jsBase = clusterId ? jetStreamUIBase(clusterId, accountName) : "";
  const queryClient = useQueryClient();
  const [actionError, setActionError] = useState("");
  const [panelOpen, setPanelOpen] = useState(false);
  const [panelError, setPanelError] = useState("");
  const [panelBusy, setPanelBusy] = useState(false);

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
    await queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "objects") });
  }

  async function onCreate(body: ObjectBucketConfigPayload) {
    if (!clusterId) return;
    setPanelBusy(true);
    setPanelError("");
    try {
      await api(clusterPath(clusterId, "/objects/buckets"), {
        method: "POST",
        body: JSON.stringify(body),
      });
      setPanelOpen(false);
      setActionError("");
      await invalidateBuckets();
    } catch (err) {
      setPanelError(err instanceof Error ? err.message : "Failed to create bucket");
      throw err;
    } finally {
      setPanelBusy(false);
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
          canManageJetStream ? (
            <button
              className="btn"
              type="button"
              onClick={() => {
                setPanelError("");
                setPanelOpen(true);
              }}
            >
              {t("jetstream.createObjectTitle")}
            </button>
          ) : undefined
        }
      />

      <Alert variant="error">{error}</Alert>

      <CreateObjectBucketPanel
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

      {bucketsQuery.isLoading && <div className="skeleton skeleton--table" />}

      {!bucketsQuery.isLoading && buckets.length === 0 && (
        <EmptyState
          title="No object buckets yet"
          description="Create a bucket to start storing files and binary payloads in JetStream object storage."
          action={
            canManageJetStream ? (
              <button
                className="btn"
                type="button"
                onClick={() => {
                  setPanelError("");
                  setPanelOpen(true);
                }}
              >
                {t("jetstream.createObjectTitle")}
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
                  return <Link to={`${jsBase}/objects/${encodeURIComponent(item.bucket)}`}>{item.bucket}</Link>;
                case "description":
                  return item.description || "—";
                case "size":
                  return formatBytes(item.size);
                case "actions":
                  return canManageJetStream ? (
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
