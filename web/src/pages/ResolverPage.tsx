import { FormEvent, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import PageHeader from "../components/ui/PageHeader";
import Alert from "../components/ui/Alert";
import VirtualTable from "../components/VirtualTable";
import { api, clusterPath } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { useAuth } from "../lib/auth";
import { clusterQueryKey } from "../lib/query";

type JWTAccount = {
  id: string;
  clusterId: string;
  name: string;
  hasJwt: boolean;
  expiresAt?: string;
  createdAt: string;
  updatedAt: string;
};

type JWTAccountList = {
  accounts: JWTAccount[];
  total: number;
};

export default function ResolverPage() {
  const { clusterId } = useCluster();
  const { canWrite, isAdmin } = useAuth();
  const queryClient = useQueryClient();
  const [jwtInput, setJwtInput] = useState("");
  const [nameOverride, setNameOverride] = useState("");
  const [error, setError] = useState("");

  const accountsQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "resolver-accounts"),
    queryFn: () => api<JWTAccountList>(clusterPath(clusterId!, "/resolver/accounts")),
    enabled: Boolean(clusterId),
  });

  async function importAccount(event: FormEvent) {
    event.preventDefault();
    if (!clusterId) return;
    try {
      await api(clusterPath(clusterId, "/resolver/accounts"), {
        method: "POST",
        body: JSON.stringify({
          jwt: jwtInput.trim(),
          name: nameOverride.trim() || undefined,
        }),
      });
      setJwtInput("");
      setNameOverride("");
      setError("");
      await queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "resolver-accounts") });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to import account JWT");
    }
  }

  async function deleteAccount(name: string) {
    if (!clusterId || !confirm(`Delete account JWT "${name}"?`)) return;
    try {
      await api(clusterPath(clusterId, `/resolver/accounts/${encodeURIComponent(name)}`), { method: "DELETE" });
      await queryClient.invalidateQueries({ queryKey: clusterQueryKey(clusterId, "resolver-accounts") });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete account JWT");
    }
  }

  async function exportAccounts() {
    if (!clusterId) return;
    try {
      const data = await api<{ accounts: { name: string; jwt: string }[] }>(
        clusterPath(clusterId, "/resolver/export"),
      );
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = `resolver-${clusterId}.json`;
      anchor.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to export resolver bundle");
    }
  }

  if (!clusterId) {
    return <p className="text-muted">Select a cluster to manage JWT resolver accounts.</p>;
  }

  const accounts = accountsQuery.data?.accounts ?? [];

  return (
    <div>
      <PageHeader
        eyebrow="JetStream"
        title="JWT Resolver"
        subtitle="Import and manage NATS account JWTs for resolver-backed auth."
        actions={
          isAdmin ? (
            <button className="btn btn--secondary" type="button" onClick={exportAccounts}>
              Export bundle
            </button>
          ) : null
        }
      />

      <Alert variant="error">{error}</Alert>

      {canWrite && (
        <form className="form-grid card mb-16" onSubmit={importAccount}>
          <h3 className="section-title">Import account JWT</h3>
          <label className="form-grid__full">
            Account JWT
            <textarea rows={8} value={jwtInput} onChange={(e) => setJwtInput(e.target.value)} required />
          </label>
          <label>
            Name override (optional)
            <input value={nameOverride} onChange={(e) => setNameOverride(e.target.value)} />
          </label>
          <button className="btn" type="submit">
            Import
          </button>
        </form>
      )}

      <VirtualTable
        columns={[
          { id: "name", header: "Account", width: "minmax(140px, 1fr)" },
          { id: "expires", header: "Expires", width: "200px" },
          { id: "actions", header: "", width: "120px" },
        ]}
        items={accounts}
        empty="No account JWTs imported yet"
        getKey={(item) => item.id}
        renderCell={(item, columnId) => {
          if (columnId === "name") return item.name;
          if (columnId === "expires") return item.expiresAt ? new Date(item.expiresAt).toLocaleString() : "—";
          if (columnId === "actions" && canWrite) {
            return (
              <button className="btn btn--ghost btn--small" type="button" onClick={() => deleteAccount(item.name)}>
                Delete
              </button>
            );
          }
          return null;
        }}
      />
    </div>
  );
}
