import { FormEvent, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "../components/ui/Alert";
import CreateNatsUserPanel, { NatsUserConfigPayload } from "../components/CreateNatsUserPanel";
import { api, clusterPath } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey } from "../lib/query";

type NATSUser = {
  id: string;
  name: string;
  signingGroup: string;
  publicKey: string;
  jwtIssued: boolean;
  assignedUserId?: string;
  createdAt: string;
  tags?: string[];
  pubAllow?: string[];
  pubDeny?: string[];
  subAllow?: string[];
  subDeny?: string[];
  allowedConnectionTypes?: string[];
  srcCidrs?: string[];
  timesLocale?: string;
  timeRanges?: { start: string; end: string }[];
  maxSubs?: number;
  maxPayload?: number;
  maxData?: number;
  jwtLifetimeNs?: number;
  respMaxMsgs?: number;
  respTTLNs?: number;
  bearerToken?: boolean;
  proxyRequired?: boolean;
};

type SigningGroup = {
  id: string;
  name: string;
  scoped: boolean;
  pubAllow: string[];
  pubDeny: string[];
  subAllow: string[];
  subDeny: string[];
  maxData: number;
  maxPayload: number;
  maxSubs: number;
};

type Creds = NATSUser & { seed?: string; creds?: string; jwt?: string };

export default function NatsUsersPage() {
  const { t } = useTranslation();
  const { accountName } = useParams();
  const { clusterId } = useCluster();
  const qc = useQueryClient();
  const account = accountName ?? "Default";
  const [search, setSearch] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [editUser, setEditUser] = useState<NATSUser | null>(null);
  const [panelError, setPanelError] = useState("");
  const [showGroup, setShowGroup] = useState(false);
  const [editGroup, setEditGroup] = useState<SigningGroup | null>(null);
  const [groupName, setGroupName] = useState("");
  const [scoped, setScoped] = useState(false);
  const [groupPubAllow, setGroupPubAllow] = useState("");
  const [groupSubAllow, setGroupSubAllow] = useState("");
  const [error, setError] = useState("");
  const [creds, setCreds] = useState<Creds | null>(null);
  const [detailId, setDetailId] = useState<string | null>(null);
  const [assignUserId, setAssignUserId] = useState("");

  function resetGroupForm() {
    setShowGroup(false);
    setEditGroup(null);
    setGroupName("");
    setScoped(false);
    setGroupPubAllow("");
    setGroupSubAllow("");
  }

  function openCreateGroup() {
    setEditGroup(null);
    setGroupName("");
    setScoped(false);
    setGroupPubAllow("");
    setGroupSubAllow("");
    setShowGroup(true);
  }

  function openEditGroup(g: SigningGroup) {
    setEditGroup(g);
    setGroupName(g.name);
    setScoped(g.scoped);
    setGroupPubAllow((g.pubAllow ?? []).join(", "));
    setGroupSubAllow((g.subAllow ?? []).join(", "));
    setShowGroup(true);
  }

  function parseSubjectList(raw: string): string[] {
    return raw
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
  }

  const usersQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `nats-users:${account}`),
    queryFn: () =>
      api<{ users: NATSUser[]; total: number }>(
        clusterPath(clusterId!, `/nats-users?account=${encodeURIComponent(account)}`),
      ),
    enabled: Boolean(clusterId),
  });

  const groupsQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `signing-groups:${account}`),
    queryFn: () =>
      api<{ groups: SigningGroup[] }>(
        clusterPath(clusterId!, `/signing-groups?account=${encodeURIComponent(account)}`),
      ),
    enabled: Boolean(clusterId),
  });

  const detailQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `nats-user:${detailId}`),
    queryFn: () =>
      api<NATSUser>(
        clusterPath(clusterId!, `/nats-users/${detailId}?account=${encodeURIComponent(account)}`),
      ),
    enabled: Boolean(clusterId && detailId),
  });

  const createMutation = useMutation({
    mutationFn: (body: NatsUserConfigPayload) =>
      api<Creds>(clusterPath(clusterId!, "/nats-users"), {
        method: "POST",
        body: JSON.stringify({ ...body, accountName: account }),
      }),
    onSuccess: async (data) => {
      setCreds(data);
      setShowCreate(false);
      setPanelError("");
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `nats-users:${account}`) });
    },
    onError: (e: Error) => setPanelError(e.message),
  });

  const updateMutation = useMutation({
    mutationFn: (body: NatsUserConfigPayload) => {
      if (!editUser) throw new Error("No user");
      return api<NATSUser>(
        clusterPath(clusterId!, `/nats-users/${editUser.id}?account=${encodeURIComponent(account)}`),
        {
          method: "PUT",
          body: JSON.stringify(body),
        },
      );
    },
    onSuccess: async () => {
      setEditUser(null);
      setPanelError("");
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `nats-users:${account}`) });
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `nats-user:${detailId}`) });
    },
    onError: (e: Error) => setPanelError(e.message),
  });

  const createGroupMutation = useMutation({
    mutationFn: () =>
      api(clusterPath(clusterId!, "/signing-groups"), {
        method: "POST",
        body: JSON.stringify({
          name: groupName,
          accountName: account,
          scoped,
          pubAllow: parseSubjectList(groupPubAllow),
          subAllow: parseSubjectList(groupSubAllow),
        }),
      }),
    onSuccess: async () => {
      resetGroupForm();
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `signing-groups:${account}`) });
    },
    onError: (e: Error) => setError(e.message),
  });

  const updateGroupMutation = useMutation({
    mutationFn: () => {
      if (!editGroup) throw new Error("No group");
      return api(
        clusterPath(clusterId!, `/signing-groups/${editGroup.id}?account=${encodeURIComponent(account)}`),
        {
          method: "PUT",
          body: JSON.stringify({
            scoped,
            pubAllow: parseSubjectList(groupPubAllow),
            subAllow: parseSubjectList(groupSubAllow),
            maxData: editGroup.maxData,
            maxPayload: editGroup.maxPayload,
            maxSubs: editGroup.maxSubs,
          }),
        },
      );
    },
    onSuccess: async () => {
      resetGroupForm();
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `signing-groups:${account}`) });
    },
    onError: (e: Error) => setError(e.message),
  });

  const deleteGroupMutation = useMutation({
    mutationFn: (id: string) =>
      api(clusterPath(clusterId!, `/signing-groups/${id}?account=${encodeURIComponent(account)}`), {
        method: "DELETE",
      }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `signing-groups:${account}`) });
    },
    onError: (e: Error) => setError(e.message),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) =>
      api(clusterPath(clusterId!, `/nats-users/${id}?account=${encodeURIComponent(account)}`), {
        method: "DELETE",
      }),
    onSuccess: async () => {
      setDetailId(null);
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `nats-users:${account}`) });
    },
    onError: (e: Error) => setError(e.message),
  });

  function confirmDeleteUser(id: string, name: string) {
    if (!window.confirm(t("natsUsers.confirmDeleteUser", { name }))) return;
    setError("");
    deleteMutation.mutate(id);
  }

  const downloadMutation = useMutation({
    mutationFn: (id: string) =>
      api<Creds>(
        clusterPath(clusterId!, `/nats-users/${id}/creds?account=${encodeURIComponent(account)}`),
      ),
    onSuccess: (data) => setCreds(data),
    onError: (e: Error) => setError(e.message),
  });

  const rotateMutation = useMutation({
    mutationFn: (id: string) =>
      api<Creds>(clusterPath(clusterId!, `/nats-users/${id}/rotate?account=${encodeURIComponent(account)}`), {
        method: "POST",
      }),
    onSuccess: (data) => setCreds(data),
    onError: (e: Error) => setError(e.message),
  });

  const mintMutation = useMutation({
    mutationFn: (id: string) =>
      api<Creds>(clusterPath(clusterId!, `/nats-users/${id}/mint-jwt?account=${encodeURIComponent(account)}`), {
        method: "POST",
      }),
    onSuccess: async (data) => {
      setCreds(data);
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `nats-users:${account}`) });
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `nats-user:${detailId}`) });
    },
    onError: (e: Error) => setError(e.message),
  });

  const assignMutation = useMutation({
    mutationFn: () =>
      api(clusterPath(clusterId!, `/nats-users/${detailId}/assign?account=${encodeURIComponent(account)}`), {
        method: "POST",
        body: JSON.stringify({ userId: assignUserId }),
      }),
    onSuccess: async () => {
      setAssignUserId("");
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `nats-user:${detailId}`) });
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `nats-users:${account}`) });
    },
    onError: (e: Error) => setError(e.message),
  });

  const users = (usersQuery.data?.users ?? []).filter((u) =>
    u.name.toLowerCase().includes(search.trim().toLowerCase()),
  );
  const groups = groupsQuery.data?.groups ?? [];

  function onCreateGroup(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (editGroup) updateGroupMutation.mutate();
    else createGroupMutation.mutate();
  }

  const panelOpen = showCreate || Boolean(editUser);
  const panelBusy = createMutation.isPending || updateMutation.isPending;
  const panelInitial = editUser ?? null;

  if (detailId && detailQuery.data) {
    const u = detailQuery.data;
    return (
      <div>
        <div className="nc-page-header">
          <div className="nc-page-header__text">
            <button type="button" className="btn secondary" onClick={() => setDetailId(null)}>
              {t("natsUsers.back")}
            </button>
            <h1 className="nc-page-title">{u.name}</h1>
            <p className="nc-page-sub">
              {t("natsUsers.signingJwt", {
                group: u.signingGroup,
                status: u.jwtIssued ? t("natsUsers.issued") : t("natsUsers.notIssued"),
              })}
            </p>
          </div>
        </div>
        {error && <Alert variant="error">{error}</Alert>}
        <div className="nc-settings-section">
          <h4>{t("common.overview")}</h4>
          <p className="mono text-sm">{u.publicKey}</p>
          <p>{t("natsUsers.assignedPerson", { id: u.assignedUserId || t("common.emDash") })}</p>
        </div>
        <div className="nc-settings-section">
          <h4>{t("natsUsers.assignTitle")}</h4>
          <div className="nc-form-row">
            <label>{t("natsUsers.consoleUserId")}</label>
            <input value={assignUserId} onChange={(e) => setAssignUserId(e.target.value)} placeholder="UUID" />
          </div>
          <button
            type="button"
            className="btn"
            disabled={!assignUserId || assignMutation.isPending}
            onClick={() => assignMutation.mutate()}
          >
            {t("natsUsers.assignPerson")}
          </button>
        </div>
        <div className="actions">
          <button
            type="button"
            className="btn secondary"
            onClick={() => {
              setPanelError("");
              setEditUser(u);
            }}
          >
            {t("natsUsers.editConfig")}
          </button>
          <button type="button" className="btn secondary" onClick={() => downloadMutation.mutate(u.id)}>
            {t("natsUsers.downloadCreds")}
          </button>
          <button type="button" className="btn secondary" onClick={() => rotateMutation.mutate(u.id)}>
            {t("natsUsers.rotateNKey")}
          </button>
          <button type="button" className="btn secondary" onClick={() => mintMutation.mutate(u.id)}>
            {t("natsUsers.mintJwt")}
          </button>
          <button type="button" className="btn danger" onClick={() => confirmDeleteUser(u.id, u.name)}>
            {t("common.delete")}
          </button>
        </div>
        {creds && (
          <div className="nc-settings-section">
            <h4>{t("natsUsers.credentials")}</h4>
            <pre className="mono" style={{ whiteSpace: "pre-wrap", fontSize: "0.8rem" }}>
              {creds.creds || creds.seed}
            </pre>
            <button type="button" className="btn secondary" onClick={() => setCreds(null)}>
              {t("common.close")}
            </button>
          </div>
        )}
        <CreateNatsUserPanel
          mode="edit"
          open={Boolean(editUser)}
          groups={groups}
          initial={panelInitial}
          busy={panelBusy}
          error={panelError}
          onClose={() => {
            setEditUser(null);
            setPanelError("");
          }}
          onSubmit={async (body) => {
            setPanelError("");
            await updateMutation.mutateAsync(body);
          }}
        />
      </div>
    );
  }

  return (
    <div>
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("natsUsers.title")}</h1>
          <p className="nc-page-sub">{t("natsUsers.subtitle")}</p>
        </div>
      </div>

      {error && <Alert variant="error">{error}</Alert>}

      <h3 className="nc-page-title" style={{ fontSize: "1.1rem" }}>
        {t("natsUsers.groupsTitle")}
      </h3>
      <div className="nc-toolbar">
        <button type="button" className="btn secondary" onClick={openCreateGroup}>
          {t("natsUsers.createGroup")}
        </button>
      </div>
      {showGroup && (
        <form className="nc-settings-section" onSubmit={onCreateGroup}>
          <h4>{editGroup ? t("natsUsers.editGroup") : t("natsUsers.createGroup")}</h4>
          <div className="nc-form-row">
            <label>{t("common.name")}</label>
            <input
              required
              value={groupName}
              disabled={Boolean(editGroup)}
              onChange={(e) => setGroupName(e.target.value)}
            />
          </div>
          <label className="role-chip">
            <input type="checkbox" checked={scoped} onChange={(e) => setScoped(e.target.checked)} />
            {t("natsUsers.scopedHelp")}
          </label>
          <div className="nc-form-row">
            <label>{t("natsUsers.pubAllow")}</label>
            <input
              value={groupPubAllow}
              onChange={(e) => setGroupPubAllow(e.target.value)}
              placeholder={t("natsUsers.subjectsPlaceholder")}
            />
          </div>
          <div className="nc-form-row">
            <label>{t("natsUsers.subAllow")}</label>
            <input
              value={groupSubAllow}
              onChange={(e) => setGroupSubAllow(e.target.value)}
              placeholder={t("natsUsers.subjectsPlaceholder")}
            />
          </div>
          <div className="actions">
            <button
              className="btn"
              type="submit"
              disabled={createGroupMutation.isPending || updateGroupMutation.isPending}
            >
              {editGroup ? t("common.save") : t("common.create")}
            </button>
            <button className="btn secondary" type="button" onClick={resetGroupForm}>
              {t("common.cancel")}
            </button>
          </div>
        </form>
      )}
      <div className="table-wrap mb-16">
        <table>
          <thead>
            <tr>
              <th>{t("common.name")}</th>
              <th>{t("natsUsers.scoped")}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {groups.map((g) => (
              <tr key={g.id}>
                <td>{g.name}</td>
                <td>{g.scoped ? t("common.yes") : t("common.no")}</td>
                <td>
                  <div className="actions">
                    <button type="button" className="btn secondary" onClick={() => openEditGroup(g)}>
                      {t("common.edit")}
                    </button>
                    {g.name !== "Default" && (
                      <button
                        type="button"
                        className="btn danger"
                        disabled={deleteGroupMutation.isPending}
                        onClick={() => {
                          if (!window.confirm(t("natsUsers.confirmDeleteGroup", { name: g.name }))) return;
                          setError("");
                          deleteGroupMutation.mutate(g.id);
                        }}
                      >
                        {t("common.delete")}
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <h3 className="nc-page-title" style={{ fontSize: "1.1rem" }}>
        {t("natsUsers.usersTitle")}
      </h3>
      <div className="nc-toolbar">
        <input
          className="nc-search"
          placeholder={t("common.searchPlaceholder")}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <button
          type="button"
          className="btn"
          onClick={() => {
            setPanelError("");
            setShowCreate(true);
          }}
        >
          {t("natsUsers.createUser")}
        </button>
      </div>

      <CreateNatsUserPanel
        mode="create"
        open={panelOpen && !editUser}
        groups={groups}
        busy={panelBusy}
        error={panelError}
        onClose={() => {
          setShowCreate(false);
          setPanelError("");
        }}
        onSubmit={async (body) => {
          setPanelError("");
          await createMutation.mutateAsync(body);
        }}
      />

      {creds && (
        <div className="nc-settings-section">
          <h4>{t("natsUsers.credentialsNamed", { name: creds.name })}</h4>
          <pre className="mono" style={{ whiteSpace: "pre-wrap", fontSize: "0.8rem" }}>
            {creds.creds || creds.seed}
          </pre>
          <button type="button" className="btn secondary" onClick={() => setCreds(null)}>
            {t("common.close")}
          </button>
        </div>
      )}

      {users.length === 0 ? (
        <div className="nc-empty">{t("natsUsers.empty")}</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("common.name")}</th>
                <th>{t("common.created")}</th>
                <th>{t("natsUsers.signingGroup")}</th>
                <th>{t("natsUsers.jwtIssued")}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id}>
                  <td>
                    <button type="button" className="btn secondary" onClick={() => setDetailId(u.id)}>
                      {u.name}
                    </button>
                  </td>
                  <td>{u.createdAt ? new Date(u.createdAt).toLocaleString() : t("common.emDash")}</td>
                  <td>{u.signingGroup}</td>
                  <td>{u.jwtIssued ? t("common.yes") : "--"}</td>
                  <td>
                    <div className="actions">
                      <button type="button" className="btn secondary" onClick={() => downloadMutation.mutate(u.id)}>
                        {t("natsUsers.creds")}
                      </button>
                      <button type="button" className="btn danger" onClick={() => confirmDeleteUser(u.id, u.name)}>
                        {t("common.delete")}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
