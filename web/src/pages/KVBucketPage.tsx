import { FormEvent, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import CreateKVBucketPanel, { KVBucketConfigPayload } from "../components/CreateKVBucketPanel";
import Pager, { DEFAULT_PAGE_SIZE, pageQuery } from "../components/Pager";
import VirtualTable from "../components/VirtualTable";
import { api, clusterPath, jetStreamUIBase, KVBucketInfo } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

type KeyListResponse = {
  keys: string[];
  total: number;
  offset: number;
  limit: number;
};

function encodeValue(raw: string): string {
  return btoa(unescape(encodeURIComponent(raw)));
}

export default function KVBucketPage() {
  const { t } = useTranslation();
  const { bucket = "", accountName } = useParams();
  const { clusterId } = useCluster();
  const { canManageJetStream } = useAuth();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const jsBase = clusterId ? jetStreamUIBase(clusterId, accountName) : "";
  const [offset, setOffset] = useState(0);
  const [editOpen, setEditOpen] = useState(false);
  const [panelError, setPanelError] = useState("");
  const [showPut, setShowPut] = useState(false);
  const [newKey, setNewKey] = useState("");
  const [newValue, setNewValue] = useState("");
  const [putError, setPutError] = useState("");
  const [putting, setPutting] = useState(false);
  const limit = DEFAULT_PAGE_SIZE;

  const bucketQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `kv-bucket:${bucket}`),
    queryFn: () =>
      api<KVBucketInfo>(clusterPath(clusterId!, `/kv/buckets/${encodeURIComponent(bucket)}`)),
    enabled: Boolean(clusterId && bucket),
  });

  const keysQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `kv-keys:${bucket}:${offset}`),
    queryFn: () =>
      api<KeyListResponse>(
        clusterPath(clusterId!, `/kv/buckets/${encodeURIComponent(bucket)}/keys${pageQuery(offset, limit)}`),
      ),
    enabled: Boolean(clusterId && bucket),
  });

  const updateMutation = useMutation({
    mutationFn: async (body: KVBucketConfigPayload) => {
      if (!clusterId) throw new Error("No system");
      return api(clusterPath(clusterId, `/kv/buckets/${encodeURIComponent(bucket)}`), {
        method: "PUT",
        body: JSON.stringify({ ...body, bucket }),
      });
    },
    onSuccess: async () => {
      setEditOpen(false);
      setPanelError("");
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `kv-bucket:${bucket}`) });
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "kv") });
    },
    onError: (e: Error) => setPanelError(e.message),
  });

  const deleteBucketMutation = useMutation({
    mutationFn: async () => {
      if (!clusterId) throw new Error("No system");
      await api(clusterPath(clusterId, `/kv/buckets/${encodeURIComponent(bucket)}`), { method: "DELETE" });
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "kv") });
      navigate(`${jsBase}/kv`);
    },
    onError: (e: Error) => setPanelError(e.message),
  });

  const keys = keysQuery.data?.keys ?? [];
  const total = keysQuery.data?.total ?? 0;
  const error =
    putError ||
    (keysQuery.error instanceof Error ? keysQuery.error.message : "") ||
    (bucketQuery.error instanceof Error ? bucketQuery.error.message : "");

  async function onPutKey(event: FormEvent) {
    event.preventDefault();
    if (!clusterId || !newKey.trim()) return;
    setPutting(true);
    setPutError("");
    try {
      await api(
        clusterPath(
          clusterId,
          `/kv/buckets/${encodeURIComponent(bucket)}/keys/${encodeURIComponent(newKey.trim())}`,
        ),
        { method: "PUT", body: JSON.stringify({ value: encodeValue(newValue) }) },
      );
      setShowPut(false);
      setNewKey("");
      setNewValue("");
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `kv-keys:${bucket}`) });
    } catch (err) {
      setPutError(err instanceof Error ? err.message : t("kv.putFailed"));
    } finally {
      setPutting(false);
    }
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <Link to={`${jsBase}/kv`} className="link-back">
            ← {t("kv.backToStores")}
          </Link>
          <h1>{bucket}</h1>
          {bucketQuery.data?.description ? (
            <p className="text-muted">{bucketQuery.data.description}</p>
          ) : null}
        </div>
        {canManageJetStream && (
          <div className="actions">
            <button
              type="button"
              className="btn"
              onClick={() => {
                setPutError("");
                setShowPut(true);
              }}
            >
              {t("kv.putKey")}
            </button>
            <button
              type="button"
              className="btn secondary"
              onClick={() => {
                setPanelError("");
                setEditOpen(true);
              }}
            >
              {t("jetstream.editConfig")}
            </button>
            <button
              type="button"
              className="btn danger"
              disabled={deleteBucketMutation.isPending}
              onClick={() => {
                if (!window.confirm(t("kv.confirmDeleteBucket", { name: bucket }))) return;
                deleteBucketMutation.mutate();
              }}
            >
              {t("common.delete")}
            </button>
          </div>
        )}
      </div>

      {error && <div className="error">{error}</div>}

      {showPut && (
        <form className="nc-settings-section" onSubmit={onPutKey}>
          <h4>{t("kv.putKey")}</h4>
          <div className="nc-form-row">
            <label>{t("common.key")}</label>
            <input required value={newKey} onChange={(e) => setNewKey(e.target.value)} />
          </div>
          <div className="nc-form-row">
            <label>{t("kv.value")}</label>
            <textarea rows={6} className="mono" value={newValue} onChange={(e) => setNewValue(e.target.value)} />
          </div>
          <div className="actions">
            <button className="btn" type="submit" disabled={putting}>
              {t("common.save")}
            </button>
            <button className="btn secondary" type="button" onClick={() => setShowPut(false)}>
              {t("common.cancel")}
            </button>
          </div>
        </form>
      )}

      <CreateKVBucketPanel
        mode="edit"
        open={editOpen}
        initial={bucketQuery.data ?? { bucket }}
        busy={updateMutation.isPending}
        error={panelError}
        onClose={() => {
          setEditOpen(false);
          setPanelError("");
        }}
        onSubmit={async (body) => {
          setPanelError("");
          await updateMutation.mutateAsync(body);
        }}
      />

      <div className="table-wrap">
        <VirtualTable
          columns={[{ id: "key", header: "Key", width: "minmax(0, 1fr)" }]}
          items={keys}
          empty="No keys"
          getKey={(key) => key}
          renderCell={(key) => (
            <Link
              to={`${jsBase}/kv/${encodeURIComponent(bucket)}/${encodeURIComponent(key)}`}
              className="mono virtual-table__truncate"
            >
              {key}
            </Link>
          )}
        />
      </div>

      <Pager total={total} offset={offset} limit={limit} onPageChange={setOffset} />
    </div>
  );
}
