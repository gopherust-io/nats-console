import { FormEvent, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import CreateObjectBucketPanel, { ObjectBucketConfigPayload } from "../components/CreateObjectBucketPanel";
import Pager, { DEFAULT_PAGE_SIZE, pageQuery } from "../components/Pager";
import VirtualTable from "../components/VirtualTable";
import Alert from "../components/ui/Alert";
import PageHeader from "../components/ui/PageHeader";
import { useConfirmDialog } from "../hooks/useConfirmDialog";
import { api, clusterPath, decodeBase64, jetStreamUIBase, ObjectBucketInfo, ObjectInfo, tryParseJSON } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { HUB_LIST_POLL_MS } from "../lib/constants";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";

const PREVIEW_LIMIT = 8192;

function encodeData(raw: string): string {
  return btoa(unescape(encodeURIComponent(raw)));
}

export default function ObjectBucketPage() {
  const { t } = useTranslation();
  const { askConfirm, confirmDialog } = useConfirmDialog();
  const { bucket = "", accountName, clusterId: routeCluster } = useParams();
  const { clusterId: contextClusterId } = useCluster();
  const clusterId = routeCluster ?? contextClusterId;
  const { canManageJetStream } = useAuth();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const jsBase = clusterId ? jetStreamUIBase(clusterId, accountName) : "";
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<ObjectInfo | null>(null);
  const [showFull, setShowFull] = useState(false);
  const [actionError, setActionError] = useState("");
  const [editOpen, setEditOpen] = useState(false);
  const [panelError, setPanelError] = useState("");
  const [showUpload, setShowUpload] = useState(false);
  const [objectName, setObjectName] = useState("");
  const [objectData, setObjectData] = useState("");
  const [uploading, setUploading] = useState(false);
  const limit = DEFAULT_PAGE_SIZE;

  const bucketQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `object-bucket:${bucket}`),
    queryFn: async () =>
      (await api<ObjectBucketInfo>(clusterPath(clusterId!, `/objects/buckets/${encodeURIComponent(bucket)}`))).data,
    enabled: Boolean(clusterId && bucket),
    refetchInterval: visibilityAwareInterval(HUB_LIST_POLL_MS),
  });

  const objectsQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `objects:${bucket}:${offset}`),
    queryFn: async () => {
      const r = await api<string[]>(
        clusterPath(clusterId!, `/objects/buckets/${encodeURIComponent(bucket)}/objects${pageQuery(offset, limit)}`),
      );
      return { objects: r.data ?? [], total: r.meta?.total ?? 0 };
    },
    enabled: Boolean(clusterId && bucket),
    refetchInterval: visibilityAwareInterval(HUB_LIST_POLL_MS),
  });

  const updateMutation = useMutation({
    mutationFn: async (body: ObjectBucketConfigPayload) => {
      if (!clusterId) throw new Error("No system");
      return api(clusterPath(clusterId, `/objects/buckets/${encodeURIComponent(bucket)}`), {
        method: "PUT",
        body: JSON.stringify({ ...body, bucket }),
      });
    },
    onSuccess: async () => {
      setEditOpen(false);
      setPanelError("");
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `object-bucket:${bucket}`) });
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "object-buckets") });
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "objects") });
    },
    onError: (e: Error) => setPanelError(e.message),
  });

  const deleteBucketMutation = useMutation({
    mutationFn: async () => {
      if (!clusterId) throw new Error("No system");
      await api(clusterPath(clusterId, `/objects/buckets/${encodeURIComponent(bucket)}`), { method: "DELETE" });
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "object-buckets") });
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "objects") });
      navigate(`${jsBase}/objects`);
    },
    onError: (e: Error) => setActionError(e.message),
  });

  const objects = objectsQuery.data?.objects ?? [];
  const total = objectsQuery.data?.total ?? 0;
  const error =
    actionError ||
    (objectsQuery.error instanceof Error ? objectsQuery.error.message : "") ||
    (bucketQuery.error instanceof Error ? bucketQuery.error.message : "");

  async function loadObject(name: string) {
    if (!clusterId) return;
    try {
      const info = (
        await api<ObjectInfo>(
          clusterPath(clusterId, `/objects/buckets/${encodeURIComponent(bucket)}/objects/${encodeURIComponent(name)}`),
        )
      ).data;
      setSelected(info);
      setShowFull(false);
      setActionError("");
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t("objects.loadFailed"));
    }
  }

  async function onUpload(event: FormEvent) {
    event.preventDefault();
    if (!clusterId || !objectName.trim()) return;
    setUploading(true);
    setActionError("");
    try {
      await api(
        clusterPath(
          clusterId,
          `/objects/buckets/${encodeURIComponent(bucket)}/objects/${encodeURIComponent(objectName.trim())}`,
        ),
        { method: "PUT", body: JSON.stringify({ data: encodeData(objectData) }) },
      );
      setShowUpload(false);
      setObjectName("");
      setObjectData("");
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `objects:${bucket}`) });
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t("objects.putFailed"));
    } finally {
      setUploading(false);
    }
  }

  function onDeleteObject(name: string) {
    if (!clusterId || !canManageJetStream) return;
    askConfirm({
      title: t("objects.confirmDeleteObjectTitle"),
      description: t("objects.confirmDeleteObject", { name }),
      action: async () => {
        setActionError("");
        try {
          await api(
            clusterPath(
              clusterId,
              `/objects/buckets/${encodeURIComponent(bucket)}/objects/${encodeURIComponent(name)}`,
            ),
            { method: "DELETE" },
          );
          if (selected?.name === name) setSelected(null);
          await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `objects:${bucket}`) });
        } catch (err) {
          setActionError(err instanceof Error ? err.message : t("objects.deleteFailed"));
        }
      },
    });
  }

  const payload = useMemo(() => (selected ? decodeBase64(selected.data) : ""), [selected]);
  const parsed = useMemo(() => tryParseJSON(payload), [payload]);
  const truncated = !showFull && payload.length > PREVIEW_LIMIT;
  const displayPayload = truncated ? `${payload.slice(0, PREVIEW_LIMIT)}\n…` : payload;

  return (
    <div className="page">
      {confirmDialog}
      <PageHeader
        eyebrow="Object store"
        title={bucket}
        subtitle={bucketQuery.data?.description || "Browse objects in this bucket and inspect payloads."}
        actions={
          <div className="actions">
            {canManageJetStream && (
              <>
                <button
                  type="button"
                  className="btn"
                  onClick={() => {
                    setActionError("");
                    setShowUpload(true);
                  }}
                >
                  {t("objects.upload")}
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
                  onClick={() =>
                    askConfirm({
                      title: t("objects.confirmDeleteBucketTitle"),
                      description: t("objects.confirmDeleteBucket", { name: bucket }),
                      action: () => deleteBucketMutation.mutate(),
                    })
                  }
                >
                  {t("common.delete")}
                </button>
              </>
            )}
            <Link to={`${jsBase}/objects`} className="btn btn--secondary">
              ← {t("objects.allBuckets")}
            </Link>
          </div>
        }
      />

      <Alert variant="error">{error}</Alert>

      {showUpload && (
        <form className="nc-settings-section" onSubmit={onUpload}>
          <h4>{t("objects.upload")}</h4>
          <div className="nc-form-row">
            <label>{t("common.name")}</label>
            <input required value={objectName} onChange={(e) => setObjectName(e.target.value)} />
          </div>
          <div className="nc-form-row">
            <label>{t("objects.data")}</label>
            <textarea rows={8} className="mono" value={objectData} onChange={(e) => setObjectData(e.target.value)} />
          </div>
          <div className="actions">
            <button className="btn" type="submit" disabled={uploading}>
              {t("common.save")}
            </button>
            <button className="btn secondary" type="button" onClick={() => setShowUpload(false)}>
              {t("common.cancel")}
            </button>
          </div>
        </form>
      )}

      <CreateObjectBucketPanel
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

      {objectsQuery.isLoading && <div className="skeleton skeleton--table" />}

      {!objectsQuery.isLoading && (
        <div className={selected ? "split-view" : undefined}>
          <div className="table-wrap">
            <VirtualTable
              columns={[
                { id: "object", header: "Object", width: "minmax(0, 1fr)" },
                { id: "actions", header: "", width: "180px", align: "right" },
              ]}
              items={objects}
              empty="No objects in this bucket"
              getKey={(name) => name}
              renderCell={(name, columnId) => {
                if (columnId === "object") {
                  return (
                    <button type="button" className="link-btn mono virtual-table__truncate" onClick={() => loadObject(name)}>
                      {name}
                    </button>
                  );
                }
                return (
                  <div className="actions">
                    <button className="btn secondary btn--small" type="button" onClick={() => loadObject(name)}>
                      {t("common.view")}
                    </button>
                    {canManageJetStream && (
                      <button
                        className="btn danger btn--small"
                        type="button"
                        onClick={() => void onDeleteObject(name)}
                      >
                        {t("common.delete")}
                      </button>
                    )}
                  </div>
                );
              }}
            />
            <Pager total={total} offset={offset} limit={limit} onPageChange={setOffset} />
          </div>

          {selected && (
            <div className="card">
              <div className="card-label">
                {selected.name} · {selected.size} bytes · {selected.modified}
              </div>
              {truncated && (
                <button className="btn secondary btn--small" type="button" onClick={() => setShowFull(true)}>
                  Show full payload
                </button>
              )}
              <pre className="mono">{parsed.isJSON && !truncated ? JSON.stringify(parsed.parsed, null, 2) : displayPayload}</pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
