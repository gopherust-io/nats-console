import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import Alert from "../components/ui/Alert";
import { useConfirmDialog } from "../hooks/useConfirmDialog";
import { api, clusterPath, decodeBase64, jetStreamUIBase, KVEntry, tryParseJSON } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";

function encodeValue(raw: string): string {
  return btoa(unescape(encodeURIComponent(raw)));
}

export default function KVKeyPage() {
  const { t } = useTranslation();
  const { askConfirm, confirmDialog } = useConfirmDialog();
  const { bucket = "", key = "", accountName, clusterId: routeCluster } = useParams();
  const decodedKey = key;
  const { clusterId: contextClusterId } = useCluster();
  const clusterId = routeCluster ?? contextClusterId;
  const { canManageJetStream } = useAuth();
  const canManageJS = canManageJetStream(clusterId);
  const navigate = useNavigate();
  const jsBase = clusterId ? jetStreamUIBase(clusterId, accountName) : "";
  const [entry, setEntry] = useState<KVEntry | null>(null);
  const [history, setHistory] = useState<KVEntry[]>([]);
  const [error, setError] = useState("");
  const [editValue, setEditValue] = useState("");
  const [saving, setSaving] = useState(false);
  const [missing, setMissing] = useState(false);
  const loadGenRef = useRef(0);

  const load = useCallback(async () => {
    if (!clusterId || !bucket || !decodedKey) return;
    const gen = ++loadGenRef.current;
    setError("");
    try {
      const [entryRes, historyRes] = await Promise.all([
        api<KVEntry>(
          clusterPath(
            clusterId,
            `/kv/buckets/${encodeURIComponent(bucket)}/keys/${encodeURIComponent(decodedKey)}`,
          ),
        ),
        api<KVEntry[]>(
          clusterPath(
            clusterId,
            `/kv/buckets/${encodeURIComponent(bucket)}/keys/${encodeURIComponent(decodedKey)}/history`,
          ),
        ),
      ]);
      if (gen !== loadGenRef.current) return;
      const entryData = entryRes.data;
      if (!entryData) {
        setEntry(null);
        setHistory(historyRes.data ?? []);
        setEditValue("");
        setMissing(true);
        setError(t("errors.not_found"));
        return;
      }
      setEntry(entryData);
      setHistory(historyRes.data ?? []);
      setEditValue(decodeBase64(entryData.value ?? ""));
      setMissing(false);
    } catch (err) {
      if (gen !== loadGenRef.current) return;
      const message = err instanceof Error ? err.message : "Failed to load key";
      setError(message);
      setMissing(true);
      setEntry(null);
    }
  }, [clusterId, bucket, decodedKey, t]);

  useEffect(() => {
    loadGenRef.current += 1;
    setEntry(null);
    setHistory([]);
    setEditValue("");
    setMissing(false);
    setError("");
    void load();
  }, [load]);

  async function onSave(event: FormEvent) {
    event.preventDefault();
    if (!clusterId || !canManageJS) return;
    setSaving(true);
    setError("");
    try {
      await api(
        clusterPath(
          clusterId,
          `/kv/buckets/${encodeURIComponent(bucket)}/keys/${encodeURIComponent(decodedKey)}`,
        ),
        { method: "PUT", body: JSON.stringify({ value: encodeValue(editValue) }) },
      );
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("kv.putFailed"));
    } finally {
      setSaving(false);
    }
  }

  function onDelete() {
    if (!clusterId || !canManageJS) return;
    askConfirm({
      title: t("kv.confirmDeleteKeyTitle"),
      description: t("kv.confirmDeleteKey", { key: decodedKey }),
      action: async () => {
        setSaving(true);
        setError("");
        try {
          await api(
            clusterPath(
              clusterId,
              `/kv/buckets/${encodeURIComponent(bucket)}/keys/${encodeURIComponent(decodedKey)}`,
            ),
            { method: "DELETE" },
          );
          navigate(`${jsBase}/kv/${encodeURIComponent(bucket)}`);
        } catch (err) {
          setError(err instanceof Error ? err.message : t("kv.deleteFailed"));
          setSaving(false);
        }
      },
    });
  }

  if (!clusterId) {
    return <p className="text-muted">{t("kv.selectCluster")}</p>;
  }

  if (!entry && !missing) {
    return <div>{error || t("common.loading")}</div>;
  }

  const payload = entry ? decodeBase64(entry.value) : "";
  const parsed = entry ? tryParseJSON(payload) : { parsed: "", isJSON: false };

  return (
    <div>
      {confirmDialog}
      <div className="page-header">
        <div>
          <Link to={`${jsBase}/kv/${encodeURIComponent(bucket)}`} className="link-back">
            ← {t("kv.backToBucket", { bucket })}
          </Link>
          <h1>{decodedKey}</h1>
        </div>
        {canManageJS && entry && (
          <button type="button" className="btn danger" disabled={saving} onClick={onDelete}>
            {t("common.delete")}
          </button>
        )}
      </div>

      {error && <Alert variant="error">{error}</Alert>}

      {entry && (
        <div className="card">
          <div className="card-label">
            {t("common.revision")} {entry.revision} · {entry.created}
          </div>
          <pre className="mono">{parsed.isJSON ? JSON.stringify(parsed.parsed, null, 2) : payload}</pre>
        </div>
      )}

      {canManageJS && (
        <form className="nc-settings-section mt-24" onSubmit={onSave}>
          <h3>{entry ? t("kv.editValue") : t("kv.putValue")}</h3>
          <div className="nc-form-row">
            <label>{t("kv.value")}</label>
            <textarea
              rows={8}
              className="mono"
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              required
            />
          </div>
          <div className="actions">
            <button className="btn" type="submit" disabled={saving}>
              {t("common.save")}
            </button>
          </div>
        </form>
      )}

      {history.length > 1 && (
        <>
          <h2 className="nc-section-title mt-24">{t("common.history")}</h2>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{t("common.revision")}</th>
                  <th>{t("common.created")}</th>
                </tr>
              </thead>
              <tbody>
                {history.map((h) => (
                  <tr key={h.revision}>
                    <td>{h.revision}</td>
                    <td>{h.created}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
