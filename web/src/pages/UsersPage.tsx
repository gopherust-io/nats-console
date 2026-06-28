import { FormEvent, useEffect, useState } from "react";
import { api, UserRecord } from "../lib/api";

const ROLES = ["admin", "operator", "viewer"];

export default function UsersPage() {
  const [users, setUsers] = useState<UserRecord[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState<string | null>(null);

  async function load() {
    setError("");
    try {
      const data = await api<{ users: UserRecord[] }>("/api/v1/users");
      setUsers(data.users);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load users");
    }
  }

  useEffect(() => {
    load();
  }, []);

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

  function onRoleChange(user: UserRecord, role: string, checked: boolean, event: FormEvent) {
    event.preventDefault();
    const next = new Set(user.roles);
    if (checked) next.add(role);
    else next.delete(role);
    if (next.size === 0) return;
    updateRoles(user, Array.from(next).sort());
  }

  return (
    <div>
      <div className="page-header">
        <h1>Users &amp; Roles</h1>
      </div>

      {error && <div className="error">{error}</div>}

      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th>Username</th>
              <th>Email</th>
              <th>Roles</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            {users.map((user) => (
              <tr key={user.id}>
                <td>{user.username}</td>
                <td>{user.email || "—"}</td>
                <td>
                  <div className="role-grid">
                    {ROLES.map((role) => (
                      <label key={role} className="role-chip">
                        <input
                          type="checkbox"
                          checked={user.roles.includes(role)}
                          disabled={saving === user.id}
                          onChange={(e) => onRoleChange(user, role, e.target.checked, e)}
                        />
                        {role}
                      </label>
                    ))}
                  </div>
                </td>
                <td>{new Date(user.created_at).toLocaleDateString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {users.length === 0 && <div className="muted">No users found.</div>}
      </div>
    </div>
  );
}
