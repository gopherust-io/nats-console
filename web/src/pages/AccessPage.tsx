import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams, useSearchParams } from "react-router";
import Alert from "../components/ui/Alert";
import SelectMenu from "../components/ui/SelectMenu";
import { useConfirmDialog } from "../hooks/useConfirmDialog";
import { useAccount } from "../lib/account";
import { api, clusterPath, UserRecord } from "../lib/api";
import { useAuth } from "../lib/auth";
import "../styles/access.css";

type AccessGrant = {
  id: string;
  userId: string;
  username?: string;
  email?: string;
  resourceType: string;
  resourceKey: string;
  role: string;
};

type AccessScope = "system" | "account";

function roleToneClass(role: string): string {
  if (role === "admin") return "access-role-chip--admin";
  if (role === "credential_downloader") return "access-role-chip--cred";
  return "access-role-chip--observer";
}

/** Canonical Access lives at /systems/:clusterId/access — scope via ?scope=&account=. */
export default function AccessPage() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const { askConfirm, confirmDialog } = useConfirmDialog();
  const { clusterId } = useParams();
  const { accounts, accountName: ctxAccount } = useAccount();
  const { canManageSystemAccess, canManageAccountAccess } = useAuth();

  const scope: AccessScope = searchParams.get("scope") === "account" ? "account" : "system";
  const selectedAccount = searchParams.get("account") || ctxAccount || "Default";

  const [grants, setGrants] = useState<AccessGrant[]>([]);
  const [people, setPeople] = useState<UserRecord[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(true);
  const [form, setForm] = useState({ userId: "", role: "observer" });

  const account = scope === "account" ? selectedAccount : "";

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

  const canMutateAccess = Boolean(
    clusterId &&
      (scope === "system"
        ? canManageSystemAccess(clusterId)
        : account && canManageAccountAccess(clusterId, account)),
  );

  const accessPath = useCallback(() => {
    if (!clusterId) return "";
    if (scope === "system") return clusterPath(clusterId, "/access");
    if (!account) return "";
    return clusterPath(clusterId, `/accounts/${encodeURIComponent(account)}/access`);
  }, [clusterId, scope, account]);

  const loadGenRef = useRef(0);

  const load = useCallback(async () => {
    if (!clusterId || !accessPath()) return;
    const gen = ++loadGenRef.current;
    setError("");
    setLoading(true);
    try {
      const [grantRes, peopleOutcome] = await Promise.all([
        api<AccessGrant[]>(accessPath()),
        api<UserRecord[]>("/api/v1/people").then(
          (res) => ({ ok: true as const, res }),
          (err: unknown) => ({ ok: false as const, err }),
        ),
      ]);
      if (gen !== loadGenRef.current) return;
      setGrants(grantRes.data ?? []);
      if (peopleOutcome.ok) {
        setPeople(peopleOutcome.res.data ?? []);
      } else {
        setPeople([]);
        const peopleErr =
          peopleOutcome.err instanceof Error ? peopleOutcome.err.message : t("access.loadFailed");
        setError(peopleErr);
      }
    } catch (err) {
      if (gen !== loadGenRef.current) return;
      setError(err instanceof Error ? err.message : t("access.loadFailed"));
    } finally {
      if (gen === loadGenRef.current) setLoading(false);
    }
  }, [clusterId, accessPath, t]);

  useEffect(() => {
    loadGenRef.current += 1;
    setGrants([]);
    setPeople([]);
    setError("");
    setForm({ userId: "", role: "observer" });
    void load();
  }, [load]);

  function goScope(next: AccessScope) {
    if (next === "system") {
      setSearchParams({}, { replace: true });
      return;
    }
    const acct = selectedAccount || ctxAccount || "Default";
    setSearchParams({ scope: "account", account: acct }, { replace: true });
  }

  function onAccountChange(name: string) {
    setSearchParams({ scope: "account", account: name }, { replace: true });
  }

  async function onAdd(event: FormEvent) {
    event.preventDefault();
    if (!form.userId || !accessPath()) return;
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
          await api(
            `${accessPath()}/${encodeURIComponent(grant.id)}?userId=${encodeURIComponent(grant.userId)}`,
            { method: "DELETE" },
          );
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

  const personOptions = useMemo(
    () =>
      people
        .filter((p) => !p.isRoot)
        .map((p) => ({
          value: p.id,
          label: p.username,
          description: p.email || undefined,
        })),
    [people],
  );

  const accountOptions = useMemo(() => {
    const opts = accounts.map((a) => ({ value: a.name, label: a.name }));
    if (opts.every((o) => o.value !== selectedAccount)) {
      opts.push({ value: selectedAccount, label: selectedAccount });
    }
    return opts;
  }, [accounts, selectedAccount]);

  const roleOptions = useMemo(
    () => roles.map((r) => ({ value: r.value, label: r.label })),
    [roles],
  );

  return (
    <div className="access-page">
      {confirmDialog}
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("access.title")}</h1>
          <p className="nc-page-sub">{t("access.subtitleUnified")}</p>
        </div>
      </div>

      {error && <Alert variant="error">{error}</Alert>}

      <div className="nc-toolbar">
        <h3 className="nc-section-title access-toolbar-title">
          {t("access.grantsTitle")}
          {!loading && (
            <span className="access-count" aria-label={t("access.grantCount", { count: grants.length })}>
              {grants.length}
            </span>
          )}
        </h3>
      </div>

      <div className="access-scope-bar">
        <div className="nc-subtabs" role="tablist" aria-label={t("access.scopeTabs")}>
          <button
            type="button"
            role="tab"
            aria-selected={scope === "system"}
            className={`nc-subtab${scope === "system" ? " active" : ""}`}
            onClick={() => goScope("system")}
          >
            {t("access.tabCluster")}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={scope === "account"}
            className={`nc-subtab${scope === "account" ? " active" : ""}`}
            onClick={() => goScope("account")}
          >
            {t("access.tabAccount")}
          </button>
        </div>
        {scope === "account" && (
          <div className="access-account-inline">
            <SelectMenu
              id="access-account"
              aria-label={t("access.account")}
              value={selectedAccount}
              options={accountOptions}
              onChange={onAccountChange}
              size="sm"
            />
          </div>
        )}
      </div>

      <p className="access-scope-hint">
        {scope === "system"
          ? t("access.subtitleSystem")
          : t("access.subtitleAccount", { account: selectedAccount })}
      </p>

      {canMutateAccess && (
        <form className="nc-settings-section access-add-form" onSubmit={onAdd}>
          <h4>{t("access.addUser")}</h4>
          <div className="access-add-grid">
            <div className="nc-form-row">
              <label htmlFor="access-person">{t("access.person")}</label>
              <SelectMenu
                id="access-person"
                value={form.userId}
                options={personOptions}
                placeholder={
                  personOptions.length === 0 ? t("access.noPeople") : t("access.selectPerson")
                }
                disabled={personOptions.length === 0}
                onChange={(userId) => setForm({ ...form, userId })}
              />
            </div>
            <div className="nc-form-row access-add-role">
              <label htmlFor="access-role">{t("access.role")}</label>
              <SelectMenu
                id="access-role"
                value={form.role}
                options={roleOptions}
                onChange={(role) => setForm({ ...form, role })}
              />
            </div>
          </div>
          <div className="actions">
            <button className="btn" type="submit" disabled={saving || !form.userId}>
              {t("access.addUser")}
            </button>
          </div>
        </form>
      )}

      <div className="nc-settings-section access-grants-section">
        {loading ? (
          <p className="nc-settings-section__empty">{t("common.loading")}</p>
        ) : grants.length === 0 ? (
          <div className="access-empty">
            <p className="access-empty__title">{t("access.empty")}</p>
            <p className="access-empty__hint">
              {canMutateAccess ? t("access.emptyHint") : t("access.emptyHintReadonly")}
            </p>
          </div>
        ) : (
          <div className="table-wrap">
            <table className="access-grants-table">
              <thead>
                <tr>
                  <th>{t("access.person")}</th>
                  <th>{t("access.role")}</th>
                  {canMutateAccess && <th className="access-col-actions" />}
                </tr>
              </thead>
              <tbody>
                {grants.map((grant) => {
                  const label = roleLabel(grant.role);
                  const grantRoleOptions = roleOptions.some((r) => r.value === grant.role)
                    ? roleOptions
                    : [...roleOptions, { value: grant.role, label }];
                  return (
                    <tr key={grant.id}>
                      <td>
                        <div className="access-person">
                          <span className="access-person__name">
                            {grant.username ?? grant.userId}
                          </span>
                          {grant.email && (
                            <span className="access-person__meta">{grant.email}</span>
                          )}
                        </div>
                      </td>
                      <td>
                        {canMutateAccess ? (
                          <SelectMenu
                            className="access-role-select"
                            size="sm"
                            value={grant.role}
                            options={grantRoleOptions}
                            disabled={saving}
                            aria-label={t("access.roleFor", {
                              name: grant.username ?? grant.userId,
                            })}
                            onChange={(role) => void onUpdateRole(grant, role)}
                          />
                        ) : (
                          <span className={`access-role-chip ${roleToneClass(grant.role)}`}>
                            {label}
                          </span>
                        )}
                      </td>
                      {canMutateAccess && (
                        <td className="access-col-actions">
                          <button
                            className="btn danger btn--small"
                            type="button"
                            disabled={saving}
                            onClick={() => void onRevoke(grant)}
                          >
                            {t("common.revoke")}
                          </button>
                        </td>
                      )}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
