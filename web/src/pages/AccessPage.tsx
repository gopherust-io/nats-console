import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import Alert from "../components/ui/Alert";
import { useConfirmDialog } from "../hooks/useConfirmDialog";
import { api, clusterPath, UserRecord } from "../lib/api";

type AccessGrant = {
  id: string;
  userId: string;
  username?: string;
  email?: string;
  resourceType: string;
  resourceKey: string;
  role: string;
};

type Props = {
  scope: "system" | "account";
};

export default function AccessPage({ scope }: Props) {
  const { t } = useTranslation();
  const { askConfirm, confirmDialog } = useConfirmDialog();
  const { clusterId, accountName } = useParams();
  const [grants, setGrants] = useState<AccessGrant[]>([]);
  const [people, setPeople] = useState<UserRecord[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ userId: "", role: scope === "system" ? "observer" : "observer" });
  const [manualUserId, setManualUserId] = useState(false);

  const roles = useMemo(
    () =>
      scope === "system"
        ? [
            { value: "admin", label: t("access.systemAdmin") },
            { value: "observer", label: t("access.systemObserver") },
          ]
        : [
            { value: "admin", label: t("access.accountAdmin") },
            { value: "observer", label: t("access.accountObserver") },
            { value: "credential_downloader", label: t("access.credentialDownloader") },
          ],
    [scope, t],
  );
  const account = accountName || t("common.default");

  const accessPath = useCallback(() => {
    if (!clusterId) return "";
    if (scope === "system") return clusterPath(clusterId, "/access");
    return clusterPath(clusterId, `/accounts/${encodeURIComponent(account)}/access`);
  }, [clusterId, scope, account]);

  const load = useCallback(async () => {
    if (!clusterId) return;
    setError("");
    try {
      const [grantRes, peopleRes] = await Promise.all([
        api<AccessGrant[]>(accessPath()),
        api<UserRecord[]>("/api/v1/people").catch(() => ({ data: [] as UserRecord[] })),
      ]);
      setGrants(grantRes.data ?? []);
      setPeople(peopleRes.data ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("access.loadFailed"));
    }
  }, [clusterId, accessPath, t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onAdd(event: FormEvent) {
    event.preventDefault();
    if (!form.userId) return;
    setSaving(true);
    setError("");
    try {
      await api(accessPath(), {
        method: "POST",
        body: JSON.stringify({ userId: form.userId, role: form.role }),
      });
      setForm((f) => ({ ...f, userId: "" }));
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("access.addFailed"));
    } finally {
      setSaving(false);
    }
  }

  function onRevoke(grant: AccessGrant) {
    const label = grant.username ?? grant.userId;
    askConfirm({
      title: t("access.confirmRevokeTitle"),
      description: t("access.confirmRevoke", { name: label }),
      confirmLabel: t("common.revoke"),
      action: async () => {
        setSaving(true);
        setError("");
        try {
          await api(`${accessPath()}/${encodeURIComponent(grant.id)}?userId=${encodeURIComponent(grant.userId)}`, {
            method: "DELETE",
          });
          await load();
        } catch (err) {
          setError(err instanceof Error ? err.message : t("access.revokeFailed"));
        } finally {
          setSaving(false);
        }
      },
    });
  }

  async function onUpdateRole(grant: AccessGrant, role: string) {
    setSaving(true);
    setError("");
    try {
      await api(accessPath(), {
        method: "PUT",
        body: JSON.stringify({ userId: grant.userId, role }),
      });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("access.updateFailed"));
    } finally {
      setSaving(false);
    }
  }

  const roleLabel = (role: string) => roles.find((r) => r.value === role)?.label ?? role;

  return (
    <div>
      {confirmDialog}
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("access.title")}</h1>
          <p className="nc-page-sub">
            {scope === "system"
              ? t("access.subtitleSystem")
              : t("access.subtitleAccount", { account })}
          </p>
        </div>
      </div>

      {error && <Alert variant="error">{error}</Alert>}

      <form className="nc-settings-section" onSubmit={onAdd}>
        <h4>{t("access.addUser")}</h4>
        <div className="nc-form-row">
          <label htmlFor="access-person">{t("access.person")}</label>
          <div className="nc-form-actions">
            {people.length > 0 && !manualUserId ? (
              <select
                id="access-person"
                value={form.userId}
                onChange={(e) => setForm({ ...form, userId: e.target.value })}
                required
              >
                <option value="">{t("access.selectPerson")}</option>
                {people
                  .filter((p) => !p.isRoot)
                  .map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.username}
                      {p.email ? ` (${p.email})` : ""}
                    </option>
                  ))}
              </select>
            ) : (
              <input
                id="access-person"
                value={form.userId}
                onChange={(e) => setForm({ ...form, userId: e.target.value })}
                placeholder={t("access.userUuid")}
                required
              />
            )}
            {people.length > 0 && (
              <button
                type="button"
                className="btn secondary btn--small"
                onClick={() => setManualUserId((v) => !v)}
              >
                {manualUserId ? t("access.pickFromList") : t("access.enterUserId")}
              </button>
            )}
          </div>
        </div>
        <div className="nc-form-row">
          <label htmlFor="access-role">{t("access.role")}</label>
          <select
            id="access-role"
            value={form.role}
            onChange={(e) => setForm({ ...form, role: e.target.value })}
          >
            {roles.map((r) => (
              <option key={r.value} value={r.value}>
                {r.label}
              </option>
            ))}
          </select>
        </div>
        <div className="actions">
          <button className="btn" type="submit" disabled={saving || !form.userId}>
            {t("access.addUser")}
          </button>
        </div>
      </form>

      <div className="nc-settings-section">
        <h4>{t("access.grantsTitle")}</h4>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("access.person")}</th>
                <th>{t("access.role")}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {grants.length === 0 && (
                <tr>
                  <td colSpan={3} className="text-muted">
                    {t("access.empty")}
                  </td>
                </tr>
              )}
              {grants.map((grant) => (
                <tr key={grant.id}>
                  <td>
                    <div>{grant.username ?? grant.userId}</div>
                    {grant.email && <div className="text-muted text-sm">{grant.email}</div>}
                  </td>
                  <td>
                    <select
                      value={grant.role}
                      disabled={saving}
                      onChange={(e) => void onUpdateRole(grant, e.target.value)}
                      aria-label={t("access.roleFor", { name: grant.username ?? grant.userId })}
                    >
                      {roles.map((r) => (
                        <option key={r.value} value={r.value}>
                          {r.label}
                        </option>
                      ))}
                      {!roles.some((r) => r.value === grant.role) && (
                        <option value={grant.role}>{roleLabel(grant.role)}</option>
                      )}
                    </select>
                  </td>
                  <td>
                    <button
                      className="btn secondary btn--small"
                      type="button"
                      disabled={saving}
                      onClick={() => void onRevoke(grant)}
                    >
                      {t("common.revoke")}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
