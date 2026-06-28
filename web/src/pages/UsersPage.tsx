import { FormEvent, useEffect, useState } from "react";
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

  async function load() {
    setError("");
    try {
      const data = await api<{ users: UserRecord[] }>("/api/v1/users");
      setUsers(data.users ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load users");
    }
  }

  useEffect(() => {
    if (canManageUsers) {
      void load();
    }
  }, [canManageUsers]);

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
      setError(err instanceof Error ? err.message : "Failed to update roles");
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
      setError(err instanceof Error ? err.message : "Failed to update access rules");
    } finally {
      setSaving(null);
    }
  }

  async function deleteUser(user: UserRecord) {
    if (!window.confirm(`Delete user ${user.username}?`)) return;
    setSaving(user.id);
    setError("");
    try {
      await api(`/api/v1/users/${user.id}`, { method: "DELETE" });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete user");
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
      setError(err instanceof Error ? err.message : "Failed to create user");
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
          <h1>Users &amp; Roles</h1>
        </div>
        <div className="muted">You do not have permission to manage users.</div>
      </div>
    );
  }

  return (
    <div>
      <div className="page-header">
        <h1>Users &amp; Roles</h1>
      </div>

      {error && <div className="error">{error}</div>}

      {isRoot && (
        <div className="card" style={{ marginBottom: "1rem" }}>
          <h2>Create admin user</h2>
          <form className="form-grid" onSubmit={createUser}>
            <label>
              Username
              <input
                value={form.username}
                onChange={(e) => setForm((f) => ({ ...f, username: e.target.value }))}
                required
              />
            </label>
            <label>
              Email
              <input
                type="email"
                value={form.email}
                onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))}
              />
            </label>
            <label>
              Password
              <input
                type="password"
                value={form.password}
                onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))}
                required
              />
            </label>
            <label>
              Roles
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
                  Unscoped admin (all clusters)
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
                  Manage users
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
                  View audit
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
                  Delete clusters
                </label>
              </div>
            )}
            {!(form.roles.length === 1 && form.roles[0] === "admin" && form.unscopedAdmin) && (
              <label>
                Cluster access
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
              {creating ? "Creating…" : "Create user"}
            </button>
          </form>
        </div>
      )}

      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th>Username</th>
              <th>Email</th>
              <th>Roles</th>
              <th>Access</th>
              <th>Created</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {users.map((user) => (
              <tr key={user.id}>
                <td>
                  {user.username}
                  {user.isRoot && <span className="badge">root</span>}
                </td>
                <td>{user.email || "—"}</td>
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
                    <span className="muted">Full access</span>
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
                            users
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
                            audit
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
                            delete clusters
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
                    <span className="muted">Unscoped admin</span>
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
                      Delete
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {users.length === 0 && <div className="muted">No users found.</div>}
      </div>
    </div>
  );
}
