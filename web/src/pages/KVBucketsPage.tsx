import { useState } from "react";
import { Link, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import CreateKVBucketPanel, { KVBucketConfigPayload } from "../components/CreateKVBucketPanel";
import Alert from "../components/ui/Alert";
import QueryErrorState from "../components/ui/QueryErrorState";
import { useConfirmDialog } from "../hooks/useConfirmDialog";
import { api, clusterPath, jetStreamUIBase, KVBucketInfo } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { HUB_LIST_POLL_MS } from "../lib/constants";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";

export default function KVBucketsPage() {
  const { t } = useTranslation();
  const { askConfirm, confirmDialog } = useConfirmDialog();
  const { accountName, clusterId: routeCluster } = useParams();
  const { clusterId: contextClusterId } = useCluster();
  const clusterId = routeCluster ?? contextClusterId;
  const { canManageJetStream } = useAuth();
  const queryClient = useQueryClient();
  const jsBase = clusterId ? jetStreamUIBase(clusterId, accountName) : "";
  const [actionError, setActionError] = useState("");
  const [panelOpen, setPanelOpen] = useState(false);
  const [panelError, setPanelError] = useState("");
  const [panelBusy, setPanelBusy] = useState(false);

  const bucketsQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "kv"),
    queryFn: async () => (await api<KVBucketInfo[]>(clusterPath(clusterId!, "/kv/buckets"))).data ?? [],
    enabled: Boolean(clusterId),
    refetchInterval: visibilityAwareInterval(HUB_LIST_POLL_MS),
  });

  const buckets = bucketsQuery.data ?? [];

  async function invalidateBuckets() {
    await queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "kv") });
  }

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
      setActionError("");
      await invalidateBuckets();
    } catch (err) {
      setPanelError(err instanceof Error ? err.message : "Failed to create bucket");
      throw err;
    } finally {
      setPanelBusy(false);
    }
  }

  function deleteBucket(name: string) {
    if (!clusterId) return;
    askConfirm({
      title: t("kv.confirmDeleteTitle"),
      description: t("kv.confirmDelete", { name }),
      action: async () => {
        try {
          await api(clusterPath(clusterId, `/kv/buckets/${encodeURIComponent(name)}`), { method: "DELETE" });
          setActionError("");
          await invalidateBuckets();
        } catch (err) {
          setActionError(err instanceof Error ? err.message : "Failed to delete bucket");
        }
      },
    });
  }

  return (
    <div>
      {confirmDialog}
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

      {actionError && <Alert variant="error">{actionError}</Alert>}
      {bucketsQuery.isError && (
        <QueryErrorState error={bucketsQuery.error} onRetry={() => void bucketsQuery.refetch()} />
      )}

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
                    <button className="btn danger btn--small" type="button" onClick={() => deleteBucket(b.bucket)}>
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
