import { FormEvent, useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api, AccessRules, UserRecord } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";

const ROLES = ["admin", "operator", "viewer"];

const emptyRules: AccessRules = {
  clusterIds: [],
  manageUsers: false,
  viewAudit: false,
  deleteClusters: false,
  assignableRoles: ["viewer"],
};

export default function UsersPage() {
  const { t } = useTranslation();
  const { user: currentUser, canManageUsers, isRoot } = useAuth();
  const { clusters } = useCluster();
  const [users, setUsers] = useState<UserRecord[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({
    username: "",
    email: "",
    password: "",
    roles: ["admin"] as string[],
    unscopedAdmin: false,
    accessRules: { ...emptyRules, manageUsers: true, assignableRoles: ["operator", "viewer"] },
  });

  const [inviteForm, setInviteForm] = useState({ username: "", email: "" });
  const [inviteUrl, setInviteUrl] = useState("");
  const [inviting, setInviting] = useState(false);

  function toggleClusterSelection(clusterIds: string[], clusterId: string, checked: boolean) {
    const next = new Set(clusterIds);
    if (checked) next.add(clusterId);
    else next.delete(clusterId);
    return Array.from(next).sort();
  }

  function ClusterAccessPicker({
    clusterIds,
    onChange,
    disabled,
  }: {
    clusterIds: string[];
    onChange: (ids: string[]) => void;
    disabled?: boolean;
  }) {
    return (
      <div className="role-grid">
        {clusters.map((cluster) => (
          <label key={cluster.id} className="role-chip">
            <input
              type="checkbox"
              checked={clusterIds.includes(cluster.id)}
              disabled={disabled}
              onChange={(e) => onChange(toggleClusterSelection(clusterIds, cluster.id, e.target.checked))}
            />
            {cluster.name}
          </label>
        ))}
      </div>
    );
  }

  const load = useCallback(async () => {
    setError("");
    try {
      const data = await api<{ users: UserRecord[] }>("/api/v1/users");
      setUsers(data.users ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("users.loadFailed"));
    }
  }, [t]);

  useEffect(() => {
    if (canManageUsers) {
      void load();
    }
  }, [canManageUsers, load]);

  async function updateRoles(user: UserRecord, roles: string[]) {
    setSaving(user.id);
    setError("");
    try {
      await api(`/api/v1/users/${user.id}/roles`, {
        method: "PUT",
        body: JSON.stringify({ roles }),
      });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("users.updateRolesFailed"));
    } finally {
      setSaving(null);
    }
  }

  async function updateAccessRules(user: UserRecord, accessRules: AccessRules) {
    setSaving(user.id);
    setError("");
    try {
      await api(`/api/v1/users/${user.id}`, {
        method: "PUT",
        body: JSON.stringify({ accessRules }),
      });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("users.updateAccessFailed"));
    } finally {
      setSaving(null);
    }
  }

  async function deleteUser(user: UserRecord) {
    if (!window.confirm(t("users.confirmDelete", { username: user.username }))) return;
    setSaving(user.id);
    setError("");
    try {
      await api(`/api/v1/users/${user.id}`, { method: "DELETE" });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("users.deleteFailed"));
    } finally {
      setSaving(null);
    }
  }

  async function createUser(event: FormEvent) {
    event.preventDefault();
    setCreating(true);
    setError("");
    try {
      await api("/api/v1/users", {
        method: "POST",
        body: JSON.stringify({
          username: form.username,
          email: form.email,
          password: form.password,
          roles: form.roles,
          accessRules:
            isRoot && form.roles.length === 1 && form.roles[0] === "admin" && form.unscopedAdmin
              ? undefined
              : { ...form.accessRules, clusterIds: form.accessRules.clusterIds },
        }),
      });
      setForm({
        username: "",
        email: "",
        password: "",
        roles: ["admin"],
        unscopedAdmin: false,
        accessRules: { ...emptyRules, manageUsers: true, assignableRoles: ["operator", "viewer"] },
      });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("users.createFailed"));
    } finally {
      setCreating(false);
    }
  }

  function onRoleChange(user: UserRecord, role: string, checked: boolean, event: FormEvent) {
    event.preventDefault();
    if (user.isRoot) return;
    const next = new Set(user.roles);
    if (checked) next.add(role);
    else next.delete(role);
    if (next.size === 0) return;
    updateRoles(user, Array.from(next).sort());
  }

  function canEditUser(user: UserRecord) {
    if (user.isRoot) return false;
    if (isRoot) return true;
    if (user.id === currentUser?.id) return true;
    return !user.accessRules?.manageUsers;
  }

  function canDeleteUser(user: UserRecord) {
    return canEditUser(user) && user.id !== currentUser?.id;
  }

  if (!canManageUsers) {
    return (
      <div>
        <div className="page-header">
          <h1>{t("users.title")}</h1>
        </div>
        <div className="muted">{t("users.noPermission")}</div>
      </div>
    );
  }

  async function invitePerson(event: FormEvent) {
    event.preventDefault();
    setInviting(true);
    setError("");
    setInviteUrl("");
    try {
      const res = await api<{ inviteUrl: string }>("/api/v1/people/invite", {
        method: "POST",
        body: JSON.stringify({
          username: inviteForm.username,
          email: inviteForm.email,
          roles: ["viewer"],
        }),
      });
      setInviteUrl(res.inviteUrl);
      setInviteForm({ username: "", email: "" });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("users.inviteFailed"));
    } finally {
      setInviting(false);
    }
  }

  return (
    <div>
      <div className="page-header">
        <h1>{t("users.title")}</h1>
      </div>

      {error && <div className="error">{error}</div>}

      <div className="card" style={{ marginBottom: "1rem" }}>
        <h2>{t("users.inviteTitle")}</h2>
        <p className="muted">{t("users.inviteHelp")}</p>
        <form className="form-grid" onSubmit={invitePerson}>
          <label>
            {t("common.username")}
            <input
              value={inviteForm.username}
              onChange={(e) => setInviteForm((f) => ({ ...f, username: e.target.value }))}
              required
            />
          </label>
          <label>
            {t("common.email")}
            <input
              type="email"
              value={inviteForm.email}
              onChange={(e) => setInviteForm((f) => ({ ...f, email: e.target.value }))}
            />
          </label>
          <button className="btn btn--primary" type="submit" disabled={inviting}>
{inviting ? t("users.inviting") : t("users.createInvite")}
          </button>
        </form>
        {inviteUrl && (
          <p className="mt-16">
{t("users.inviteUrl")}{" "}
            <code style={{ wordBreak: "break-all" }}>{inviteUrl}</code>
          </p>
        )}
      </div>

      {isRoot && (
        <div className="card" style={{ marginBottom: "1rem" }}>
          <h2>{t("users.createAdminTitle")}</h2>
          <form className="form-grid" onSubmit={createUser}>
            <label>
              {t("common.username")}
              <input
                value={form.username}
                onChange={(e) => setForm((f) => ({ ...f, username: e.target.value }))}
                required
              />
            </label>
            <label>
              {t("common.email")}
              <input
                type="email"
                value={form.email}
                onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))}
              />
            </label>
            <label>
              {t("common.password")}
              <input
                type="password"
                value={form.password}
                onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))}
                required
              />
            </label>
            <label>
              {t("common.roles")}
              <select
                multiple
                value={form.roles}
                onChange={(e) =>
                  setForm((f) => ({
                    ...f,
                    roles: Array.from(e.target.selectedOptions, (o) => o.value),
                  }))
                }
              >
                {ROLES.map((role) => (
                  <option key={role} value={role}>
                    {role}
                  </option>
                ))}
              </select>
            </label>
            {form.roles.includes("admin") && (
              <div className="role-grid">
                <label className="role-chip">
                  <input
                    type="checkbox"
                    checked={form.unscopedAdmin}
                    onChange={(e) => setForm((f) => ({ ...f, unscopedAdmin: e.target.checked }))}
                  />
                  {t("users.unscopedAdmin")}
                </label>
                <label className="role-chip">
                  <input
                    type="checkbox"
                    checked={form.accessRules.manageUsers}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        accessRules: { ...f.accessRules, manageUsers: e.target.checked },
                      }))
                    }
                  />
                  {t("users.manageUsers")}
                </label>
                <label className="role-chip">
                  <input
                    type="checkbox"
                    checked={form.accessRules.viewAudit}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        accessRules: { ...f.accessRules, viewAudit: e.target.checked },
                      }))
                    }
                  />
                  {t("users.viewAudit")}
                </label>
                <label className="role-chip">
                  <input
                    type="checkbox"
                    checked={form.accessRules.deleteClusters}
                    onChange={(e) =>
                      setForm((f) => ({
                        ...f,
                        accessRules: { ...f.accessRules, deleteClusters: e.target.checked },
                      }))
                    }
                  />
                  {t("users.deleteClusters")}
                </label>
              </div>
            )}
            {!(form.roles.length === 1 && form.roles[0] === "admin" && form.unscopedAdmin) && (
              <label>
                {t("users.clusterAccess")}
                <ClusterAccessPicker
                  clusterIds={form.accessRules.clusterIds ?? []}
                  onChange={(clusterIds) =>
                    setForm((f) => ({
                      ...f,
                      accessRules: { ...f.accessRules, clusterIds },
                    }))
                  }
                />
              </label>
            )}
            <button className="btn btn--primary" type="submit" disabled={creating}>
{creating ? t("users.creating") : t("users.createUser")}
            </button>
          </form>
        </div>
      )}

      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th>{t("common.username")}</th>
              <th>{t("common.email")}</th>
              <th>{t("common.roles")}</th>
              <th>{t("users.access")}</th>
              <th>{t("common.created")}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {users.map((user) => (
              <tr key={user.id}>
                <td>
                  {user.username}
{user.isRoot && <span className="badge">{t("users.root")}</span>}
                </td>
                <td>{user.email || t("common.emDash")}</td>
                <td>
                  <div className="role-grid">
                    {ROLES.map((role) => (
                      <label key={role} className="role-chip">
                        <input
                          type="checkbox"
                          checked={user.roles.includes(role)}
                          disabled={saving === user.id || !canEditUser(user)}
                          onChange={(e) => onRoleChange(user, role, e.target.checked, e)}
                        />
                        {role}
                      </label>
                    ))}
                  </div>
                </td>
                <td>
                  {user.isRoot ? (
<span className="muted">{t("users.fullAccess")}</span>
                  ) : user.accessRules ? (
                    <div className="role-grid">
                      {user.roles.includes("admin") && (
                        <>
                          <label className="role-chip">
                            <input
                              type="checkbox"
                              checked={user.accessRules.manageUsers}
                              disabled={saving === user.id || !canEditUser(user)}
                              onChange={(e) =>
                                updateAccessRules(user, {
                                  ...user.accessRules!,
                                  manageUsers: e.target.checked,
                                })
                              }
                            />
                            {t("users.usersShort")}
                          </label>
                          <label className="role-chip">
                            <input
                              type="checkbox"
                              checked={user.accessRules.viewAudit}
                              disabled={saving === user.id || !canEditUser(user)}
                              onChange={(e) =>
                                updateAccessRules(user, {
                                  ...user.accessRules!,
                                  viewAudit: e.target.checked,
                                })
                              }
                            />
                            {t("users.auditShort")}
                          </label>
                          <label className="role-chip">
                            <input
                              type="checkbox"
                              checked={user.accessRules.deleteClusters}
                              disabled={saving === user.id || !canEditUser(user)}
                              onChange={(e) =>
                                updateAccessRules(user, {
                                  ...user.accessRules!,
                                  deleteClusters: e.target.checked,
                                })
                              }
                            />
                            {t("users.clustersShort")}
                          </label>
                        </>
                      )}
                      <ClusterAccessPicker
                        clusterIds={user.accessRules.clusterIds ?? []}
                        disabled={saving === user.id || !canEditUser(user)}
                        onChange={(clusterIds) =>
                          updateAccessRules(user, {
                            ...user.accessRules!,
                            clusterIds,
                          })
                        }
                      />
                    </div>
                  ) : (
<span className="muted">{t("users.unscopedAdmin")}</span>
                  )}
                </td>
                <td>{new Date(user.createdAt).toLocaleDateString()}</td>
                <td>
                  {canDeleteUser(user) && (
                    <button
                      className="btn btn--ghost btn--small"
                      type="button"
                      disabled={saving === user.id}
                      onClick={() => deleteUser(user)}
                    >
                      {t("common.delete")}
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
{users.length === 0 && <div className="muted">{t("users.empty")}</div>}
      </div>
    </div>
  );
}
