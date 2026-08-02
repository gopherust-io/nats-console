import { FormEvent, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "../components/ui/Alert";
import { useConfirmDialog } from "../hooks/useConfirmDialog";
import { api, clusterPath } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

type ExportItem = {
  id: string;
  kind: string;
  name: string;
  subject: string;
  description: string;
  createdAt: string;
};

type KindId = "service" | "stream" | "feed";

export default function SharingPage() {
  const { t } = useTranslation();
  const { askConfirm, confirmDialog } = useConfirmDialog();
  const { accountName, clusterId: routeCluster } = useParams();
  const { clusterId: contextClusterId } = useCluster();
  const clusterId = routeCluster ?? contextClusterId;
  const { canManageAccountAccess } = useAuth();
  const qc = useQueryClient();
  const account = accountName ?? "Default";
  const canMutateSharing = Boolean(clusterId && canManageAccountAccess(clusterId, account));
  const [kind, setKind] = useState<KindId>("service");
  const [showForm, setShowForm] = useState(false);
  const [editItem, setEditItem] = useState<ExportItem | null>(null);
  const [name, setName] = useState("");
  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState("");

  const kinds = [
    { id: "service" as const, label: t("sharing.services") },
    { id: "stream" as const, label: t("sharing.streams") },
    { id: "feed" as const, label: t("sharing.feeds") },
  ];

  const exportsQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `exports:${account}:${kind}`),
    queryFn: async () =>
      (
        await api<ExportItem[]>(
          clusterPath(
            clusterId!,
            `/sharing/exports?account=${encodeURIComponent(account)}&kind=${kind}`,
          ),
        )
      ).data ?? [],
    enabled: Boolean(clusterId),
  });

  function resetForm() {
    setShowForm(false);
    setEditItem(null);
    setName("");
    setSubject("");
    setDescription("");
  }

  useEffect(() => {
    resetForm();
    setError("");
  }, [clusterId, account]);

  function openCreate() {
    setEditItem(null);
    setName("");
    setSubject("");
    setDescription("");
    setShowForm(true);
  }

  function openEdit(item: ExportItem) {
    setEditItem(item);
    setName(item.name);
    setSubject(item.subject);
    setDescription(item.description ?? "");
    setShowForm(true);
  }

  const createMutation = useMutation({
    mutationFn: () =>
      api(clusterPath(clusterId!, "/sharing/exports"), {
        method: "POST",
        body: JSON.stringify({
          accountName: account,
          kind,
          name,
          subject,
          description,
        }),
      }),
    onSuccess: async () => {
      resetForm();
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `exports:${account}:${kind}`) });
    },
    onError: (e: Error) => setError(e.message),
  });

  const updateMutation = useMutation({
    mutationFn: () => {
      if (!editItem) throw new Error("No export");
      return api(
        clusterPath(clusterId!, `/sharing/exports/${editItem.id}?account=${encodeURIComponent(account)}`),
        {
          method: "PUT",
          body: JSON.stringify({ name, subject, description }),
        },
      );
    },
    onSuccess: async () => {
      resetForm();
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `exports:${account}:${kind}`) });
    },
    onError: (e: Error) => setError(e.message),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) =>
      api(
        clusterPath(clusterId!, `/sharing/exports/${id}?account=${encodeURIComponent(account)}`),
        { method: "DELETE" },
      ),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `exports:${account}:${kind}`) });
    },
    onError: (e: Error) => setError(e.message),
  });

  const items = exportsQuery.data ?? [];
  const createLabel =
    kind === "service"
      ? t("sharing.exportService")
      : kind === "stream"
        ? t("sharing.exportStream")
        : t("sharing.exportFeed");

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (editItem) updateMutation.mutate();
    else createMutation.mutate();
  }

  return (
    <div>
      {confirmDialog}
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("sharing.title")}</h1>
          <p className="nc-page-sub">{t("sharing.subtitle")}</p>
        </div>
      </div>

      {error && <Alert variant="error">{error}</Alert>}

      <div className="nc-toolbar">
        <h3 className="nc-section-title" style={{ marginBottom: 0 }}>{t("sharing.exports")}</h3>
        {canMutateSharing && (
          <button type="button" className="btn" onClick={openCreate}>
            {createLabel}
          </button>
        )}
      </div>

      <div className="nc-subtabs">
        {kinds.map((k) => (
          <button
            key={k.id}
            type="button"
            className={`nc-subtab${kind === k.id ? " active" : ""}`}
            onClick={() => setKind(k.id)}
          >
            {k.label}
          </button>
        ))}
      </div>

      {showForm && canMutateSharing && (
        <form className="nc-settings-section" onSubmit={onSubmit}>
          <h4>{editItem ? t("sharing.editExport") : createLabel}</h4>
          <div className="nc-form-row">
            <label>{t("common.name")}</label>
            <input required value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="nc-form-row">
            <label>{t("common.subject")}</label>
            <input required value={subject} onChange={(e) => setSubject(e.target.value)} placeholder="svc.orders" />
          </div>
          <div className="nc-form-row">
            <label>{t("common.description")}</label>
            <input value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          <div className="actions">
            <button
              className="btn"
              type="submit"
              disabled={createMutation.isPending || updateMutation.isPending}
            >
              {t("common.save")}
            </button>
            <button className="btn secondary" type="button" onClick={resetForm}>
              {t("common.cancel")}
            </button>
          </div>
        </form>
      )}

      <div className="nc-settings-section">
        {items.length === 0 ? (
          <p className="nc-settings-section__empty">{t("sharing.empty")}</p>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{t("common.name")}</th>
                  <th>{t("common.subject")}</th>
                  <th>{t("common.description")}</th>
                  <th>{t("common.created")}</th>
                  {canMutateSharing && <th />}
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id}>
                    <td>{item.name}</td>
                    <td className="mono">{item.subject}</td>
                    <td>{item.description || t("common.emDash")}</td>
                    <td>{new Date(item.createdAt).toLocaleString()}</td>
                    {canMutateSharing && (
                      <td>
                        <div className="actions">
                          <button type="button" className="btn secondary btn--small" onClick={() => openEdit(item)}>
                            {t("common.edit")}
                          </button>
                          <button
                            type="button"
                            className="btn danger btn--small"
                            onClick={() =>
                              askConfirm({
                                title: t("sharing.confirmDeleteTitle"),
                                description: t("sharing.confirmDelete", { name: item.name }),
                                action: () => {
                                  setError("");
                                  deleteMutation.mutate(item.id);
                                },
                              })
                            }
                          >
                            {t("common.delete")}
                          </button>
                        </div>
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
