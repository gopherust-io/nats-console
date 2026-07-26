import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import CreateKVBucketPanel, { KVBucketConfigPayload } from "../components/CreateKVBucketPanel";
import CreateObjectBucketPanel, { ObjectBucketConfigPayload } from "../components/CreateObjectBucketPanel";
import CreateStreamPanel, { StreamConfigPayload } from "../components/CreateStreamPanel";
import { AccountInfo, api, clusterPath, StreamInfo } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey, invalidateJetStreamTopology } from "../lib/query";

type StreamList = { streams: StreamInfo[]; total: number };
type KVList = { buckets: { bucket: string; values: number }[]; total?: number };
type ObjList = { buckets: { bucket: string; size: number }[]; total?: number };

export default function JetStreamHubPage() {
  const { t } = useTranslation();
  const { clusterId: routeCluster, accountName } = useParams();
  const { clusterId } = useCluster();
  const { canManageJetStream } = useAuth();
  const id = routeCluster ?? clusterId;
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [menuOpen, setMenuOpen] = useState(false);
  const [menuClosing, setMenuClosing] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const menuOpenRef = useRef(menuOpen);
  const menuClosingRef = useRef(menuClosing);
  menuOpenRef.current = menuOpen;
  menuClosingRef.current = menuClosing;
  const [search, setSearch] = useState("");
  const [createMode, setCreateMode] = useState<"stream" | "mirror" | "kv" | "object" | null>(null);
  const [panelError, setPanelError] = useState("");

  const clearCloseTimer = useCallback(() => {
    if (closeTimer.current) {
      clearTimeout(closeTimer.current);
      closeTimer.current = null;
    }
  }, []);

  const requestMenuClose = useCallback(() => {
    if (!menuOpenRef.current || menuClosingRef.current) return;
    setMenuClosing(true);
    clearCloseTimer();
    closeTimer.current = setTimeout(() => {
      setMenuOpen(false);
      setMenuClosing(false);
      closeTimer.current = null;
    }, 150);
  }, [clearCloseTimer]);

  const requestMenuOpen = useCallback(() => {
    clearCloseTimer();
    setMenuClosing(false);
    setMenuOpen(true);
  }, [clearCloseTimer]);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (!menuRef.current?.contains(e.target as Node)) requestMenuClose();
    }
    document.addEventListener("mousedown", onDoc);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      clearCloseTimer();
    };
  }, [requestMenuClose, clearCloseTimer]);
  const accountQuery = useQuery({
    queryKey: clusterQueryKey(id, "account"),
    queryFn: () => api<AccountInfo>(clusterPath(id!, "/account")),
    enabled: Boolean(id),
  });

  const streamsQuery = useQuery({
    queryKey: clusterQueryKey(id, "streams"),
    queryFn: () => api<StreamList>(clusterPath(id!, "/streams?offset=0&limit=100")),
    enabled: Boolean(id),
  });

  const kvQuery = useQuery({
    queryKey: clusterQueryKey(id, "kv"),
    queryFn: () => api<KVList>(clusterPath(id!, "/kv/buckets")),
    enabled: Boolean(id),
  });

  const objQuery = useQuery({
    queryKey: clusterQueryKey(id, "objects"),
    queryFn: () => api<ObjList>(clusterPath(id!, "/objects/buckets")),
    enabled: Boolean(id),
  });

  const base = `/systems/${id}/accounts/${encodeURIComponent(accountName ?? "Default")}`;

  const filteredStreams = useMemo(() => {
    const q = search.trim().toLowerCase();
    const list = streamsQuery.data?.streams ?? [];
    if (!q) return list;
    return list.filter((s) => s.config.name.toLowerCase().includes(q));
  }, [streamsQuery.data, search]);

  async function invalidateLists() {
    await Promise.all([
      invalidateJetStreamTopology(id),
      qc.invalidateQueries({ queryKey: clusterQueryKey(id, "kv") }),
      qc.invalidateQueries({ queryKey: clusterQueryKey(id, "objects") }),
      qc.invalidateQueries({ queryKey: clusterQueryKey(id, "account") }),
    ]);
  }

  const streamPanelMutation = useMutation({
    mutationFn: async (body: StreamConfigPayload) => {
      if (!id) throw new Error("No system");
      return api(clusterPath(id, "/streams"), { method: "POST", body: JSON.stringify(body) });
    },
    onSuccess: async () => {
      setCreateMode(null);
      setPanelError("");
      await invalidateLists();
    },
    onError: (e: Error) => setPanelError(e.message),
  });

  const kvPanelMutation = useMutation({
    mutationFn: async (body: KVBucketConfigPayload) => {
      if (!id) throw new Error("No system");
      return api(clusterPath(id, "/kv/buckets"), { method: "POST", body: JSON.stringify(body) });
    },
    onSuccess: async () => {
      setCreateMode(null);
      setPanelError("");
      await invalidateLists();
      navigate(`${base}/jetstream/kv`);
    },
    onError: (e: Error) => setPanelError(e.message),
  });

  const objectPanelMutation = useMutation({
    mutationFn: async (body: ObjectBucketConfigPayload) => {
      if (!id) throw new Error("No system");
      return api(clusterPath(id, "/objects/buckets"), { method: "POST", body: JSON.stringify(body) });
    },
    onSuccess: async () => {
      setCreateMode(null);
      setPanelError("");
      await invalidateLists();
      navigate(`${base}/jetstream/objects`);
    },
    onError: (e: Error) => setPanelError(e.message),
  });

  async function deleteStream(streamName: string) {
    if (!id || !confirm(`Delete stream "${streamName}"? This removes the stream and its consumers.`)) return;
    try {
      await api(clusterPath(id, `/streams/${encodeURIComponent(streamName)}`), { method: "DELETE" });
      await invalidateLists();
    } catch (err) {
      setPanelError(err instanceof Error ? err.message : "Failed to delete stream");
    }
  }

  const account = accountQuery.data;
  function pct(used = 0, max = 0) {
    if (!max || max <= 0) return "0.0%";
    return `${Math.min(100, (used / max) * 100).toFixed(1)}%`;
  }

  const menuVisible = menuOpen || menuClosing;

  return (
    <div>
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("jetstream.title")}</h1>
          <p className="nc-page-sub">{t("jetstream.subtitle")}</p>
        </div>
        {canManageJetStream && (
          <div className="nc-dropdown" ref={menuRef}>
            <button
              type="button"
              className="btn"
              aria-expanded={menuOpen && !menuClosing}
              onClick={() => (menuVisible && !menuClosing ? requestMenuClose() : requestMenuOpen())}
            >
              {t("jetstream.createStream")}
            </button>
            {menuVisible && (
              <div className="nc-dropdown__menu" data-state={menuClosing ? "closed" : "open"}>
                <button
                  type="button"
                  onClick={() => {
                    setPanelError("");
                    setCreateMode("stream");
                    requestMenuClose();
                  }}
                >
                  {t("jetstream.stream")}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setPanelError("");
                    setCreateMode("mirror");
                    requestMenuClose();
                  }}
                >
                  {t("jetstream.mirror")}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setPanelError("");
                    setCreateMode("kv");
                    requestMenuClose();
                  }}
                >
                  {t("jetstream.kvBucket")}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setPanelError("");
                    setCreateMode("object");
                    requestMenuClose();
                  }}
                >
                  {t("jetstream.objectBucket")}
                </button>
              </div>
            )}
          </div>
        )}
      </div>
      <p className="text-muted mb-12">{t("jetstream.lifecycleHint")}</p>
      {panelError && <div className="error mb-12">{panelError}</div>}

      <div className="nc-usage-grid">
        <div className="nc-usage-card">
          <div className="nc-usage-card__label">{t("jetstream.streams")}</div>
          <div className="nc-usage-card__tier">R1</div>
          <div className="nc-usage-card__ring">{pct(account?.streams, account?.limits.maxStreams)}</div>
        </div>
        <div className="nc-usage-card">
          <div className="nc-usage-card__label">{t("jetstream.consumers")}</div>
          <div className="nc-usage-card__tier">R1</div>
          <div className="nc-usage-card__ring">{pct(account?.consumers, account?.limits.maxConsumers)}</div>
        </div>
        <div className="nc-usage-card">
          <div className="nc-usage-card__label">{t("jetstream.fileStorage")}</div>
          <div className="nc-usage-card__tier">R1</div>
          <div className="nc-usage-card__ring">{pct(account?.storage, account?.limits.maxStorage)}</div>
        </div>
        <div className="nc-usage-card">
          <div className="nc-usage-card__label">{t("jetstream.memoryStorage")}</div>
          <div className="nc-usage-card__tier">R1</div>
          <div className="nc-usage-card__ring">{pct(account?.memory, account?.limits.maxMemory)}</div>
        </div>
      </div>

      <div className="nc-toolbar">
        <input
          className="nc-search"
          placeholder={t("common.searchPlaceholder")}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <div className="actions">
          <Link className="btn secondary" to={`${base}/jetstream/kv`}>
            {t("jetstream.kvBuckets")}
          </Link>
          <Link className="btn secondary" to={`${base}/jetstream/objects`}>
            {t("jetstream.objectStores")}
          </Link>
        </div>
      </div>

      <CreateStreamPanel
        mode="create"
        variant={createMode === "mirror" ? "mirror" : "stream"}
        open={createMode === "stream" || createMode === "mirror"}
        busy={streamPanelMutation.isPending}
        error={panelError}
        onClose={() => {
          setCreateMode(null);
          setPanelError("");
        }}
        onSubmit={async (body) => {
          setPanelError("");
          await streamPanelMutation.mutateAsync(body);
        }}
      />

      <CreateKVBucketPanel
        mode="create"
        open={createMode === "kv"}
        busy={kvPanelMutation.isPending}
        error={panelError}
        onClose={() => {
          setCreateMode(null);
          setPanelError("");
        }}
        onSubmit={async (body) => {
          setPanelError("");
          await kvPanelMutation.mutateAsync(body);
        }}
      />

      <CreateObjectBucketPanel
        mode="create"
        open={createMode === "object"}
        busy={objectPanelMutation.isPending}
        error={panelError}
        onClose={() => {
          setCreateMode(null);
          setPanelError("");
        }}
        onSubmit={async (body) => {
          setPanelError("");
          await objectPanelMutation.mutateAsync(body);
        }}
      />

      {filteredStreams.length === 0 ? (
        <div className="nc-empty">{t("jetstream.empty")}</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t("common.name")}</th>
                <th>{t("common.subjects")}</th>
                <th>{t("jetstream.messages")}</th>
                <th>{t("jetstream.consumers")}</th>
                <th>{t("jetstream.storage")}</th>
                {canManageJetStream && <th />}
              </tr>
            </thead>
            <tbody>
              {filteredStreams.map((s) => (
                <tr key={s.config.name}>
                  <td>
                    <Link to={`${base}/jetstream/streams/${encodeURIComponent(s.config.name)}`}>
                      {s.config.name}
                    </Link>
                  </td>
                  <td className="mono">{(s.config.subjects ?? []).join(", ")}</td>
                  <td>{s.state.messages}</td>
                  <td>{s.state.consumerCount}</td>
                  <td>{s.config.storage}</td>
                  {canManageJetStream && (
                    <td>
                      <button
                        className="btn danger btn--small"
                        type="button"
                        onClick={() => deleteStream(s.config.name)}
                      >
                        Delete
                      </button>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {(kvQuery.data?.buckets?.length ?? 0) > 0 && (
        <p className="text-muted mt-16">
          {t("jetstream.kvSummary", { count: kvQuery.data!.buckets.length })} ·{" "}
          <Link to={`${base}/jetstream/kv`}>{t("jetstream.openKv")}</Link>
        </p>
      )}
      {(objQuery.data?.buckets?.length ?? 0) > 0 && (
        <p className="text-muted">
          {t("jetstream.objectSummary", { count: objQuery.data!.buckets.length })} ·{" "}
          <Link to={`${base}/jetstream/objects`}>{t("jetstream.openObjects")}</Link>
        </p>
      )}
    </div>
  );
}
