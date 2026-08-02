import { FormEvent, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams, useSearchParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "../components/ui/Alert";
import QueryErrorState from "../components/ui/QueryErrorState";
import CreateNatsUserPanel, { NatsUserConfigPayload } from "../components/CreateNatsUserPanel";
import { useConfirmDialog } from "../hooks/useConfirmDialog";
import { api, clusterPath } from "../lib/api";
import { useAuth } from "../lib/auth";
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

type SubjectPermissionEntry = {
  userId: string;
  name: string;
  signingGroup: string;
  assignedUserId?: string;
  source: string;
  matchedPattern?: string;
};

type SubjectPermissionsResult = {
  subject: string;
  publish: SubjectPermissionEntry[];
  subscribe: SubjectPermissionEntry[];
  queueSubscribe: SubjectPermissionEntry[];
};

type ViewMode = "users" | "subjectLookup";

function permSourceLabel(source: string, t: (key: string) => string) {
  switch (source) {
    case "user":
      return t("natsUsers.permSourceUser");
    case "signing-group":
      return t("natsUsers.permSourceSigningGroup");
    case "unrestricted":
      return t("natsUsers.permSourceUnrestricted");
    default:
      return source;
  }
}

function SubjectPermissionTable({
  title,
  empty,
  hint,
  entries,
  onSelectUser,
  t,
}: {
  title: string;
  empty: string;
  hint?: string;
  entries: SubjectPermissionEntry[];
  onSelectUser: (id: string) => void;
  t: (key: string, opts?: Record<string, string>) => string;
}) {
  return (
    <div className="nc-settings-section">
      <h4>{title}</h4>
      {hint ? <p>{hint}</p> : null}
      {entries.length === 0 ? (
        <p className="nc-settings-section__empty">{empty}</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("common.name")}</th>
                <th>{t("natsUsers.signingGroup")}</th>
                <th>{t("natsUsers.permSource")}</th>
                <th>{t("natsUsers.matchedPattern")}</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((entry) => (
                <tr key={entry.userId}>
                  <td>
                    <button type="button" className="link-btn" onClick={() => onSelectUser(entry.userId)}>
                      {entry.name}
                    </button>
                  </td>
                  <td>{entry.signingGroup}</td>
                  <td>{permSourceLabel(entry.source, t)}</td>
                  <td className="mono">{entry.matchedPattern || t("common.emDash")}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

export default function NatsUsersPage() {
  const { t } = useTranslation();
  const { askConfirm, confirmDialog } = useConfirmDialog();
  const { accountName, clusterId: routeCluster } = useParams();
  const { clusterId: contextClusterId } = useCluster();
  const clusterId = routeCluster ?? contextClusterId;
  const { canManageAccountAccess, canDownloadCreds, canWriteCluster } = useAuth();
  const qc = useQueryClient();
  const account = accountName ?? "Default";
  const canManageAccount = Boolean(clusterId && canManageAccountAccess(clusterId, account));
  const canMutateUsers = canManageAccount || Boolean(clusterId && canWriteCluster(clusterId));
  const canCreds = (natsUserId: string) =>
    Boolean(clusterId && canDownloadCreds(clusterId, account, natsUserId));
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
  const [searchParams, setSearchParams] = useSearchParams();
  const [viewMode, setViewMode] = useState<ViewMode>(() =>
    searchParams.get("subject") ? "subjectLookup" : "users",
  );
  const [subjectDraft, setSubjectDraft] = useState(() => searchParams.get("subject") ?? "");
  const [lookupSubject, setLookupSubject] = useState(() => searchParams.get("subject")?.trim() ?? "");

  useEffect(() => {
    setCreds(null);
    setDetailId(null);
    setShowCreate(false);
    setEditUser(null);
    setPanelError("");
    setShowGroup(false);
    setEditGroup(null);
    setError("");
    setAssignUserId("");
  }, [clusterId, account]);

  useEffect(() => {
    return () => {
      setCreds(null);
    };
  }, []);

  useEffect(() => {
    setCreds(null);
  }, [detailId]);

  useEffect(() => {
    const subject = searchParams.get("subject")?.trim() ?? "";
    if (!subject) return;
    setViewMode("subjectLookup");
    setSubjectDraft(subject);
    setLookupSubject(subject);
  }, [searchParams]);

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
    queryFn: async () =>
      (await api<NATSUser[]>(
        clusterPath(clusterId!, `/nats-users?account=${encodeURIComponent(account)}`),
      )).data ?? [],
    enabled: Boolean(clusterId),
  });

  const groupsQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `signing-groups:${account}`),
    queryFn: async () =>
      (await api<SigningGroup[]>(
        clusterPath(clusterId!, `/signing-groups?account=${encodeURIComponent(account)}`),
      )).data ?? [],
    enabled: Boolean(clusterId),
  });

  const streamsQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "streams:subjects"),
    queryFn: async () =>
      (await api<{ config?: { subjects?: string[] } }[]>(
        clusterPath(clusterId!, "/streams?offset=0&limit=200"),
      )).data ?? [],
    enabled: Boolean(clusterId && viewMode === "subjectLookup"),
  });

  const subjectPermissionsQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `subject-permissions:${account}:${lookupSubject}`),
    queryFn: async () =>
      (await api<SubjectPermissionsResult>(
        clusterPath(
          clusterId!,
          `/subject-permissions?account=${encodeURIComponent(account)}&subject=${encodeURIComponent(lookupSubject)}`,
        ),
      )).data,
    enabled: Boolean(clusterId && lookupSubject),
  });

  const streamSubjectSuggestions = useMemo(() => {
    const seen = new Set<string>();
    for (const stream of streamsQuery.data ?? []) {
      for (const subject of stream.config?.subjects ?? []) {
        if (subject) seen.add(subject);
      }
    }
    return Array.from(seen).sort();
  }, [streamsQuery.data]);

  const detailQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, `nats-user:${detailId}`),
    queryFn: async () =>
      (await api<NATSUser>(
        clusterPath(clusterId!, `/nats-users/${detailId}?account=${encodeURIComponent(account)}`),
      )).data,
    enabled: Boolean(clusterId && detailId),
  });

  const createMutation = useMutation({
    mutationFn: async (body: NatsUserConfigPayload) =>
      (await api<Creds>(clusterPath(clusterId!, "/nats-users"), {
        method: "POST",
        body: JSON.stringify({ ...body, accountName: account }),
      })).data,
    onSuccess: async (data) => {
      setCreds(data);
      setShowCreate(false);
      setPanelError("");
      await qc.invalidateQueries({ queryKey: clusterQueryKey(clusterId, `nats-users:${account}`) });
    },
    onError: (e: Error) => setPanelError(e.message),
  });

  const updateMutation = useMutation({
    mutationFn: async (body: NatsUserConfigPayload) => {
      if (!editUser) throw new Error("No user");
      return (await api<NATSUser>(
        clusterPath(clusterId!, `/nats-users/${editUser.id}?account=${encodeURIComponent(account)}`),
        {
          method: "PUT",
          body: JSON.stringify(body),
        },
      )).data;
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
    askConfirm({
      title: t("natsUsers.confirmDeleteUserTitle"),
      description: t("natsUsers.confirmDeleteUser", { name }),
      action: () => {
        setError("");
        deleteMutation.mutate(id);
      },
    });
  }

  const downloadMutation = useMutation({
    mutationFn: async (id: string) =>
      (await api<Creds>(
        clusterPath(clusterId!, `/nats-users/${id}/creds?account=${encodeURIComponent(account)}`),
      )).data,
    onSuccess: (data) => setCreds(data),
    onError: (e: Error) => setError(e.message),
  });

  const rotateMutation = useMutation({
    mutationFn: async (id: string) =>
      (await api<Creds>(clusterPath(clusterId!, `/nats-users/${id}/rotate?account=${encodeURIComponent(account)}`), {
        method: "POST",
      })).data,
    onSuccess: (data) => setCreds(data),
    onError: (e: Error) => setError(e.message),
  });

  const mintMutation = useMutation({
    mutationFn: async (id: string) =>
      (await api<Creds>(clusterPath(clusterId!, `/nats-users/${id}/mint-jwt?account=${encodeURIComponent(account)}`), {
        method: "POST",
      })).data,
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

  const users = (usersQuery.data ?? []).filter((u) =>
    u.name.toLowerCase().includes(search.trim().toLowerCase()),
  );
  const groups = groupsQuery.data ?? [];

  function onCreateGroup(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (editGroup) updateGroupMutation.mutate();
    else createGroupMutation.mutate();
  }

  function runSubjectLookup() {
    const subject = subjectDraft.trim();
    setLookupSubject(subject);
    if (subject) {
      setSearchParams({ subject }, { replace: true });
    } else {
      setSearchParams({}, { replace: true });
    }
  }

  function switchViewMode(mode: ViewMode) {
    setViewMode(mode);
    if (mode === "users") {
      setSearchParams({}, { replace: true });
    }
  }

  const panelOpen = showCreate || Boolean(editUser);
  const panelBusy = createMutation.isPending || updateMutation.isPending;
  const panelInitial = editUser ?? null;

  if (detailId && detailQuery.data) {
    const u = detailQuery.data;
    return (
      <div>
        {confirmDialog}
        <div className="nc-page-header">
          <div className="nc-page-header__text">
            <button type="button" className="link-back" onClick={() => setDetailId(null)}>
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
          <p className="mono">{u.publicKey}</p>
          <p className="nc-settings-section__empty">
            {t("natsUsers.assignedPerson", { id: u.assignedUserId || t("common.emDash") })}
          </p>
        </div>
        {canManageAccount && (
          <div className="nc-settings-section">
            <h4>{t("natsUsers.assignTitle")}</h4>
            <div className="nc-form-row">
              <label htmlFor="assign-console-user">{t("natsUsers.consoleUserId")}</label>
              <div className="nc-form-actions">
                <input
                  id="assign-console-user"
                  value={assignUserId}
                  onChange={(e) => setAssignUserId(e.target.value)}
                  placeholder="UUID"
                />
                <button
                  type="button"
                  className="btn"
                  disabled={!assignUserId || assignMutation.isPending}
                  onClick={() => assignMutation.mutate()}
                >
                  {t("natsUsers.assignPerson")}
                </button>
              </div>
            </div>
          </div>
        )}
        <div className="nc-settings-section">
          <h4>{t("common.actions")}</h4>
          <div className="actions">
            {canMutateUsers && (
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
            )}
            {canCreds(u.id) && (
              <>
                <button type="button" className="btn secondary" onClick={() => downloadMutation.mutate(u.id)}>
                  {t("natsUsers.downloadCreds")}
                </button>
                <button type="button" className="btn secondary" onClick={() => rotateMutation.mutate(u.id)}>
                  {t("natsUsers.rotateNKey")}
                </button>
                <button type="button" className="btn secondary" onClick={() => mintMutation.mutate(u.id)}>
                  {t("natsUsers.mintJwt")}
                </button>
              </>
            )}
            {canMutateUsers && (
              <button type="button" className="btn danger" onClick={() => confirmDeleteUser(u.id, u.name)}>
                {t("common.delete")}
              </button>
            )}
          </div>
        </div>
        {creds && (
          <div className="nc-settings-section">
            <h4>{t("natsUsers.credentials")}</h4>
            <pre className="mono text-sm" style={{ whiteSpace: "pre-wrap" }}>
              {creds.creds || creds.seed}
            </pre>
            <div className="actions">
              <button type="button" className="btn secondary" onClick={() => setCreds(null)}>
                {t("common.close")}
              </button>
            </div>
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
      {confirmDialog}
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("natsUsers.title")}</h1>
          <p className="nc-page-sub">{t("natsUsers.subtitle")}</p>
        </div>
      </div>

      {(usersQuery.isError || groupsQuery.isError) && (
        <QueryErrorState
          error={usersQuery.error ?? groupsQuery.error}
          onRetry={() => {
            void usersQuery.refetch();
            void groupsQuery.refetch();
          }}
        />
      )}
      {error && <Alert variant="error">{error}</Alert>}

      <h3 className="nc-section-title">{t("natsUsers.groupsTitle")}</h3>
      {canMutateUsers && (
        <div className="nc-toolbar">
          <button type="button" className="btn secondary" onClick={openCreateGroup}>
            {t("natsUsers.createGroup")}
          </button>
        </div>
      )}
      {showGroup && canMutateUsers && (
        <form className="nc-settings-section" onSubmit={onCreateGroup}>
          <h4>{editGroup ? t("natsUsers.editGroup") : t("natsUsers.createGroup")}</h4>
          <div className="nc-form-row">
            <label htmlFor="group-name">{t("common.name")}</label>
            <input
              id="group-name"
              required
              value={groupName}
              disabled={Boolean(editGroup)}
              onChange={(e) => setGroupName(e.target.value)}
            />
          </div>
          <div className="nc-form-row">
            <label className="role-chip">
              <input type="checkbox" checked={scoped} onChange={(e) => setScoped(e.target.checked)} />
              {t("natsUsers.scopedHelp")}
            </label>
          </div>
          <div className="nc-form-row">
            <label htmlFor="group-pub-allow">{t("natsUsers.pubAllow")}</label>
            <input
              id="group-pub-allow"
              value={groupPubAllow}
              onChange={(e) => setGroupPubAllow(e.target.value)}
              placeholder={t("natsUsers.subjectsPlaceholder")}
            />
          </div>
          <div className="nc-form-row">
            <label htmlFor="group-sub-allow">{t("natsUsers.subAllow")}</label>
            <input
              id="group-sub-allow"
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
      <div className="nc-settings-section">
        <div className="table-wrap">
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
                    {canMutateUsers && (
                      <div className="actions">
                        <button type="button" className="btn secondary btn--small" onClick={() => openEditGroup(g)}>
                          {t("common.edit")}
                        </button>
                        {g.name !== "Default" && (
                          <button
                            type="button"
                            className="btn danger btn--small"
                            disabled={deleteGroupMutation.isPending}
                            onClick={() =>
                              askConfirm({
                                title: t("natsUsers.confirmDeleteGroupTitle"),
                                description: t("natsUsers.confirmDeleteGroup", { name: g.name }),
                                action: () => {
                                  setError("");
                                  deleteGroupMutation.mutate(g.id);
                                },
                              })
                            }
                          >
                            {t("common.delete")}
                          </button>
                        )}
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <h3 className="nc-section-title">{t("natsUsers.usersTitle")}</h3>
      <div className="nc-toolbar">
        <div className="metrics-range" role="group" aria-label={t("natsUsers.subjectLookupTitle")}>
          <button
            type="button"
            className={`metrics-range__btn${viewMode === "users" ? " metrics-range__btn--active" : ""}`}
            onClick={() => switchViewMode("users")}
          >
            {t("natsUsers.viewUsers")}
          </button>
          <button
            type="button"
            className={`metrics-range__btn${viewMode === "subjectLookup" ? " metrics-range__btn--active" : ""}`}
            onClick={() => switchViewMode("subjectLookup")}
          >
            {t("natsUsers.viewSubjectLookup")}
          </button>
        </div>
        {viewMode === "users" ? (
          <>
            <input
              className="nc-search"
              placeholder={t("common.searchPlaceholder")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            {canMutateUsers && (
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
            )}
          </>
        ) : null}
      </div>

      {viewMode === "subjectLookup" ? (
        <>
          <div className="nc-settings-section">
            <h4>{t("natsUsers.subjectLookupTitle")}</h4>
            <div className="nc-form-row">
              <label htmlFor="subject-lookup-input">{t("natsUsers.subjectInputLabel")}</label>
              <div className="nc-form-actions">
                <input
                  id="subject-lookup-input"
                  className="mono"
                  value={subjectDraft}
                  onChange={(e) => setSubjectDraft(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      runSubjectLookup();
                    }
                  }}
                  placeholder="orders.new"
                  autoComplete="off"
                />
                {streamSubjectSuggestions.length > 0 ? (
                  <select
                    aria-label={t("natsUsers.subjectSuggestions")}
                    className="mono nc-subject-suggestions"
                    value=""
                    onChange={(e) => {
                      const next = e.target.value;
                      if (!next) return;
                      setSubjectDraft(next);
                    }}
                  >
                    <option value="">{t("natsUsers.subjectSuggestions")}</option>
                    {streamSubjectSuggestions.map((subject) => (
                      <option key={subject} value={subject}>
                        {subject}
                      </option>
                    ))}
                  </select>
                ) : null}
                <button
                  type="button"
                  className="btn"
                  disabled={!subjectDraft.trim() || subjectPermissionsQuery.isFetching}
                  onClick={runSubjectLookup}
                >
                  {t("natsUsers.subjectLookup")}
                </button>
              </div>
            </div>
            {subjectPermissionsQuery.isError && (
              <QueryErrorState
                error={subjectPermissionsQuery.error}
                onRetry={() => void subjectPermissionsQuery.refetch()}
              />
            )}
          </div>
          {lookupSubject && subjectPermissionsQuery.data ? (
            <>
              <SubjectPermissionTable
                title={t("natsUsers.publishSection")}
                empty={t("natsUsers.noUsersPublish")}
                entries={subjectPermissionsQuery.data.publish}
                onSelectUser={setDetailId}
                t={t}
              />
              <SubjectPermissionTable
                title={t("natsUsers.subscribeSection")}
                empty={t("natsUsers.noUsersSubscribe")}
                entries={subjectPermissionsQuery.data.subscribe}
                onSelectUser={setDetailId}
                t={t}
              />
              <SubjectPermissionTable
                title={t("natsUsers.queueSubscribeSection")}
                empty={t("natsUsers.noUsersQueueSubscribe")}
                hint={t("natsUsers.queueSubscribeHint")}
                entries={subjectPermissionsQuery.data.queueSubscribe}
                onSelectUser={setDetailId}
                t={t}
              />
            </>
          ) : null}
        </>
      ) : (
        <>
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
          <pre className="mono text-sm" style={{ whiteSpace: "pre-wrap" }}>
            {creds.creds || creds.seed}
          </pre>
          <div className="actions">
            <button type="button" className="btn secondary" onClick={() => setCreds(null)}>
              {t("common.close")}
            </button>
          </div>
        </div>
      )}

      <div className="nc-settings-section">
        {users.length === 0 ? (
          <p className="nc-settings-section__empty">{t("natsUsers.empty")}</p>
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
                      <button type="button" className="link-btn" onClick={() => setDetailId(u.id)}>
                        {u.name}
                      </button>
                    </td>
                    <td>{u.createdAt ? new Date(u.createdAt).toLocaleString() : t("common.emDash")}</td>
                    <td>{u.signingGroup}</td>
                    <td>{u.jwtIssued ? t("common.yes") : "--"}</td>
                    <td>
                      <div className="actions">
                        {canCreds(u.id) && (
                          <button type="button" className="btn secondary btn--small" onClick={() => downloadMutation.mutate(u.id)}>
                            {t("natsUsers.creds")}
                          </button>
                        )}
                        {canMutateUsers && (
                          <button type="button" className="btn danger btn--small" onClick={() => confirmDeleteUser(u.id, u.name)}>
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
        )}
      </div>
        </>
      )}
    </div>
  );
}
