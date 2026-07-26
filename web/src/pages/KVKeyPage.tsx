import { FormEvent, useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api, clusterPath, decodeBase64, jetStreamUIBase, KVEntry, tryParseJSON } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";

type HistoryResponse = {
  entries: KVEntry[];
  total: number;
};

function encodeValue(raw: string): string {
  return btoa(unescape(encodeURIComponent(raw)));
}

export default function KVKeyPage() {
  const { t } = useTranslation();
  const { bucket = "", key = "", accountName } = useParams();
  const decodedKey = decodeURIComponent(key);
  const { clusterId } = useCluster();
  const { canManageJetStream } = useAuth();
  const navigate = useNavigate();
  const jsBase = clusterId ? jetStreamUIBase(clusterId, accountName) : "";
  const [entry, setEntry] = useState<KVEntry | null>(null);
  const [history, setHistory] = useState<KVEntry[]>([]);
  const [error, setError] = useState("");
  const [editValue, setEditValue] = useState("");
  const [saving, setSaving] = useState(false);
  const [missing, setMissing] = useState(false);

  const load = useCallback(async () => {
    if (!clusterId || !bucket || !decodedKey) return;
    setError("");
    try {
      const [entryData, historyData] = await Promise.all([
        api<KVEntry>(
          clusterPath(
            clusterId,
            `/kv/buckets/${encodeURIComponent(bucket)}/keys/${encodeURIComponent(decodedKey)}`,
          ),
        ),
        api<HistoryResponse>(
          clusterPath(
            clusterId,
            `/kv/buckets/${encodeURIComponent(bucket)}/keys/${encodeURIComponent(decodedKey)}/history`,
          ),
        ),
      ]);
      setEntry(entryData);
      setHistory(historyData.entries ?? []);
      setEditValue(decodeBase64(entryData.value));
      setMissing(false);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to load key";
      setError(message);
      setMissing(true);
      setEntry(null);
    }
  }, [clusterId, bucket, decodedKey]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onSave(event: FormEvent) {
    event.preventDefault();
    if (!clusterId || !canManageJetStream) return;
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

  async function onDelete() {
    if (!clusterId || !canManageJetStream) return;
    if (!window.confirm(t("kv.confirmDeleteKey", { key: decodedKey }))) return;
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
      <div className="page-header">
        <div>
          <Link to={`${jsBase}/kv/${encodeURIComponent(bucket)}`} className="link-back">
            ← {t("kv.backToBucket", { bucket })}
          </Link>
          <h1>{decodedKey}</h1>
        </div>
        {canManageJetStream && entry && (
          <button type="button" className="btn danger" disabled={saving} onClick={() => void onDelete()}>
            {t("common.delete")}
          </button>
        )}
      </div>

      {error && <div className="error">{error}</div>}

      {entry && (
        <div className="card">
          <div className="card-label">
            {t("common.revision")} {entry.revision} · {entry.created}
          </div>
          <pre className="mono">{parsed.isJSON ? JSON.stringify(parsed.parsed, null, 2) : payload}</pre>
        </div>
      )}

      {canManageJetStream && (
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
          <button className="btn" type="submit" disabled={saving}>
            {t("common.save")}
          </button>
        </form>
      )}

      {history.length > 1 && (
        <>
          <h2 className="mt-24">{t("common.history")}</h2>
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
