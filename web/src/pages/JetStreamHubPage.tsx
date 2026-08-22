import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { KVBucketConfigPayload } from "../components/CreateKVBucketPanel";
import type { ObjectBucketConfigPayload } from "../components/CreateObjectBucketPanel";
import type { StreamConfigPayload } from "../components/CreateStreamPanel";
import BlastRadiusPanel from "../components/BlastRadiusPanel";
import JetStreamSectionTabs, { parseJetStreamSection } from "../components/JetStreamSectionTabs";
import Alert from "../components/ui/Alert";
import ConfirmDialog from "../components/ui/ConfirmDialog";
import QueryErrorState from "../components/ui/QueryErrorState";
import VirtualTable, { type VirtualTableColumn } from "../components/VirtualTable";
import StreamFavoriteButton, { useFavoriteStreams } from "../components/StreamFavoriteButton";
import { AccountInfo, api, BlastRadius, clusterPath, StreamInfo } from "../lib/api";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import { HUB_LIST_POLL_MS } from "../lib/constants";
import { isFavoriteStream, sortStreamsFavoritesFirst } from "../lib/favoriteStreams";
import { clusterQueryKey, invalidateJetStreamTopology, visibilityAwareInterval } from "../lib/query";
import { fetchAllStreams } from "../lib/streams";

const CreateStreamPanel = lazy(() => import("../components/CreateStreamPanel"));
const CreateKVBucketPanel = lazy(() => import("../components/CreateKVBucketPanel"));
const CreateObjectBucketPanel = lazy(() => import("../components/CreateObjectBucketPanel"));

type KVBucketSummary = { bucket: string; values: number };
type ObjBucketSummary = { bucket: string; size: number };

export default function JetStreamHubPage() {
  const { t } = useTranslation();
  const { clusterId: routeCluster, accountName } = useParams();
  const { clusterId } = useCluster();
  const id = routeCluster ?? clusterId;
  const { canManageJetStream } = useAuth();
  const canManageJS = canManageJetStream(id);
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const section = parseJetStreamSection(searchParams.get("tab"));
  const [menuOpen, setMenuOpen] = useState(false);
  const [menuClosing, setMenuClosing] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const menuOpenRef = useRef(menuOpen);
  const menuClosingRef = useRef(menuClosing);
  menuOpenRef.current = menuOpen;
  menuClosingRef.current = menuClosing;
  const [search, setSearch] = useState("");
  const [favoritesOnly, setFavoritesOnly] = useState(false);
  const favorites = useFavoriteStreams();
  const [createMode, setCreateMode] = useState<"stream" | "mirror" | "kv" | "object" | null>(null);
  const [panelError, setPanelError] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [deleteBusy, setDeleteBusy] = useState(false);

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

  // Canonical Streams URL has no ?tab=; strip legacy ?tab=streams.
  useEffect(() => {
    const tab = searchParams.get("tab");
    if (tab === "streams") {
      const next = new URLSearchParams(searchParams);
      next.delete("tab");
      setSearchParams(next, { replace: true });
    }
  }, [searchParams, setSearchParams]);

  const streamsSection =
    section === "streams" || section === "consumers" || section === "messages";

  const accountQuery = useQuery({
    queryKey: clusterQueryKey(id, "account"),
    queryFn: async () => (await api<AccountInfo>(clusterPath(id!, "/account"))).data,
    enabled: Boolean(id),
    staleTime: HUB_LIST_POLL_MS,
    // Overview gauges need account; elsewhere keep warm without aggressive polling.
    refetchInterval:
      section === "overview" ? visibilityAwareInterval(HUB_LIST_POLL_MS) : false,
  });

  const streamsQuery = useQuery({
    queryKey: clusterQueryKey(id, "streams"),
    queryFn: async () => fetchAllStreams(id!),
    enabled: Boolean(id) && streamsSection,
    staleTime: HUB_LIST_POLL_MS,
    refetchInterval: streamsSection ? visibilityAwareInterval(HUB_LIST_POLL_MS) : false,
  });

  const kvQuery = useQuery({
    queryKey: clusterQueryKey(id, "kv"),
    queryFn: async () => (await api<KVBucketSummary[]>(clusterPath(id!, "/kv/buckets"))).data ?? [],
    enabled: Boolean(id) && section === "overview",
    staleTime: HUB_LIST_POLL_MS,
    refetchInterval: section === "overview" ? visibilityAwareInterval(HUB_LIST_POLL_MS) : false,
  });

  const objQuery = useQuery({
    queryKey: clusterQueryKey(id, "objects"),
    queryFn: async () => (await api<ObjBucketSummary[]>(clusterPath(id!, "/objects/buckets"))).data ?? [],
    enabled: Boolean(id) && section === "overview",
    staleTime: HUB_LIST_POLL_MS,
    refetchInterval: section === "overview" ? visibilityAwareInterval(HUB_LIST_POLL_MS) : false,
  });

  const impactQuery = useQuery({
    queryKey: clusterQueryKey(id, `stream-impact:${deleteTarget ?? ""}`),
    queryFn: async () =>
      (await api<BlastRadius>(clusterPath(id!, `/streams/${encodeURIComponent(deleteTarget!)}/impact`))).data,
    enabled: Boolean(id && deleteTarget),
  });

  const base = `/systems/${id}/accounts/${encodeURIComponent(accountName ?? "Default")}`;
  const jetstreamBase = `${base}/jetstream`;

  const filteredStreams = useMemo(() => {
    const q = search.trim().toLowerCase();
    let list = streamsQuery.data ?? [];
    if (favoritesOnly && id) {
      list = list.filter((s) => isFavoriteStream(id, s.config.name, favorites));
    }
    if (q) {
      list = list.filter((s) => s.config.name.toLowerCase().includes(q));
    }
    if (id) {
      list = sortStreamsFavoritesFirst(list, id, favorites);
    }
    return list;
  }, [streamsQuery.data, search, favoritesOnly, favorites, id]);

  const streamsWithConsumers = useMemo(
    () => (streamsQuery.data ?? []).filter((s) => (s.state.consumerCount ?? 0) > 0),
    [streamsQuery.data],
  );

  const streamsWithMessages = useMemo(
    () => (streamsQuery.data ?? []).filter((s) => (s.state.messages ?? 0) > 0),
    [streamsQuery.data],
  );

  const invalidateLists = useCallback(async () => {
    await Promise.all([
      invalidateJetStreamTopology(id),
      qc.invalidateQueries({ queryKey: clusterQueryKey(id, "kv") }),
      qc.invalidateQueries({ queryKey: clusterQueryKey(id, "objects") }),
      qc.invalidateQueries({ queryKey: clusterQueryKey(id, "account") }),
    ]);
  }, [id, qc]);

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

  const deleteStream = useCallback(
    async (streamName: string) => {
      if (!id) return;
      setDeleteBusy(true);
      setPanelError("");
      try {
        await api(clusterPath(id, `/streams/${encodeURIComponent(streamName)}`), { method: "DELETE" });
        setDeleteTarget(null);
        await invalidateLists();
      } catch (err) {
        setPanelError(err instanceof Error ? err.message : t("streams.deleteFailed"));
        setDeleteTarget(null);
      } finally {
        setDeleteBusy(false);
      }
    },
    [id, invalidateLists, t],
  );

  const streamColumns = useMemo<VirtualTableColumn[]>(() => {
    const cols: VirtualTableColumn[] = [
      { id: "favorite", header: "", width: "44px" },
      { id: "name", header: t("common.name"), width: "minmax(120px, 1.1fr)" },
      { id: "subjects", header: t("common.subjects"), width: "minmax(180px, 2fr)" },
      { id: "messages", header: t("jetstream.messages"), width: "96px", align: "right" },
      { id: "consumers", header: t("jetstream.consumers"), width: "108px", align: "right" },
      { id: "storage", header: t("jetstream.storage"), width: "96px" },
    ];
    if (canManageJS) {
      cols.push({ id: "actions", header: "", width: "112px", align: "right" });
    }
    return cols;
  }, [t, canManageJS]);

  const consumerColumns = useMemo<VirtualTableColumn[]>(
    () => [
      { id: "name", header: t("common.name"), width: "minmax(140px, 1.2fr)" },
      { id: "consumers", header: t("jetstream.consumers"), width: "120px", align: "right" },
      { id: "actions", header: "", width: "140px", align: "right" },
    ],
    [t],
  );

  const messageColumns = useMemo<VirtualTableColumn[]>(
    () => [
      { id: "name", header: t("common.name"), width: "minmax(140px, 1.2fr)" },
      { id: "messages", header: t("jetstream.messages"), width: "120px", align: "right" },
      { id: "actions", header: "", width: "160px", align: "right" },
    ],
    [t],
  );

  const renderStreamCell = useCallback(
    (s: StreamInfo, columnId: string) => {
      switch (columnId) {
        case "favorite":
          return id ? <StreamFavoriteButton clusterId={id} streamName={s.config.name} /> : null;
        case "name":
          return (
            <Link to={`${base}/jetstream/streams/${encodeURIComponent(s.config.name)}`}>
              {s.config.name}
              {(s.isDlq || s.config.name.endsWith("_DLQ")) && (
                <span className="dlq-badge" title={t("streams.tabDlq")}>
                  DLQ
                </span>
              )}
            </Link>
          );
        case "subjects":
          return <span className="mono virtual-table__truncate">{(s.config.subjects ?? []).join(", ")}</span>;
        case "messages":
          return s.state.messages;
        case "consumers":
          return s.state.consumerCount;
        case "storage":
          return s.config.storage;
        case "actions":
          return canManageJS ? (
            <button
              className="btn danger btn--small"
              type="button"
              onClick={() => setDeleteTarget(s.config.name)}
            >
              {t("common.delete")}
            </button>
          ) : null;
        default:
          return null;
      }
    },
    [base, canManageJS, id, t],
  );

  const renderConsumerCell = useCallback(
    (s: StreamInfo, columnId: string) => {
      switch (columnId) {
        case "name":
          return (
            <Link to={`${base}/jetstream/streams/${encodeURIComponent(s.config.name)}?tab=consumers`}>
              {s.config.name}
            </Link>
          );
        case "consumers":
          return s.state.consumerCount;
        case "actions":
          return (
            <Link
              className="btn secondary btn--small"
              to={`${base}/jetstream/streams/${encodeURIComponent(s.config.name)}?tab=consumers`}
            >
              {t("jetstream.openStreamConsumers")}
            </Link>
          );
        default:
          return null;
      }
    },
    [base, t],
  );

  const renderMessageCell = useCallback(
    (s: StreamInfo, columnId: string) => {
      switch (columnId) {
        case "name":
          return (
            <Link to={`${base}/jetstream/streams/${encodeURIComponent(s.config.name)}?tab=messages`}>
              {s.config.name}
            </Link>
          );
        case "messages":
          return s.state.messages;
        case "actions":
          return (
            <Link
              className="btn secondary btn--small"
              to={`${base}/jetstream/streams/${encodeURIComponent(s.config.name)}?tab=messages`}
            >
              {t("jetstream.openStreamMessages")}
            </Link>
          );
        default:
          return null;
      }
    },
    [base, t],
  );

  const getStreamKey = useCallback((s: StreamInfo) => s.config.name, []);

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
        {canManageJS && section === "streams" && (
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

      <JetStreamSectionTabs base={jetstreamBase} active={section} />

      <p className="text-muted mb-12">{t("jetstream.lifecycleHint")}</p>
      {panelError && !createMode && <Alert variant="error">{panelError}</Alert>}
      {accountQuery.isError && (
        <QueryErrorState error={accountQuery.error} onRetry={() => void accountQuery.refetch()} />
      )}
      {streamsQuery.isError && (
        <QueryErrorState error={streamsQuery.error} onRetry={() => void streamsQuery.refetch()} />
      )}
      {section === "overview" && kvQuery.isError && (
        <QueryErrorState error={kvQuery.error} onRetry={() => void kvQuery.refetch()} />
      )}
      {section === "overview" && objQuery.isError && (
        <QueryErrorState error={objQuery.error} onRetry={() => void objQuery.refetch()} />
      )}

      {(createMode === "stream" || createMode === "mirror") && (
        <Suspense fallback={null}>
          <CreateStreamPanel
            mode="create"
            variant={createMode === "mirror" ? "mirror" : "stream"}
            open
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
        </Suspense>
      )}

      {createMode === "kv" && (
        <Suspense fallback={null}>
          <CreateKVBucketPanel
            mode="create"
            open
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
        </Suspense>
      )}

      {createMode === "object" && (
        <Suspense fallback={null}>
          <CreateObjectBucketPanel
            mode="create"
            open
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
        </Suspense>
      )}

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title={t("streams.confirmDeleteTitle")}
        description={
          deleteTarget ? (
            <>
              <p>{t("streams.confirmDelete", { name: deleteTarget })}</p>
              <BlastRadiusPanel
                data={impactQuery.data}
                loading={impactQuery.isFetching}
                error={
                  impactQuery.error instanceof Error
                    ? impactQuery.error.message
                    : impactQuery.isError
                      ? "error"
                      : null
                }
              />
            </>
          ) : null
        }
        busy={deleteBusy}
        onCancel={() => {
          if (!deleteBusy) setDeleteTarget(null);
        }}
        onConfirm={() => {
          if (deleteTarget) void deleteStream(deleteTarget);
        }}
      />

      {section === "overview" && (
        <>
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

          <div className="actions mb-12">
            <Link className="btn secondary" to={`${base}/jetstream/kv`}>
              {t("jetstream.kvBuckets")}
            </Link>
            <Link className="btn secondary" to={`${base}/jetstream/objects`}>
              {t("jetstream.objectStores")}
            </Link>
          </div>

          {(kvQuery.data?.length ?? 0) > 0 && (
            <p className="text-muted mt-16">
              {t("jetstream.kvSummary", { count: kvQuery.data!.length })} ·{" "}
              <Link to={`${base}/jetstream/kv`}>{t("jetstream.openKv")}</Link>
            </p>
          )}
          {(objQuery.data?.length ?? 0) > 0 && (
            <p className="text-muted">
              {t("jetstream.objectSummary", { count: objQuery.data!.length })} ·{" "}
              <Link to={`${base}/jetstream/objects`}>{t("jetstream.openObjects")}</Link>
            </p>
          )}
        </>
      )}

      {section === "streams" && (
        <>
          <div className="nc-toolbar">
            <input
              className="nc-search"
              placeholder={t("common.searchPlaceholder")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            <label className="nc-toolbar__check">
              <input
                type="checkbox"
                checked={favoritesOnly}
                onChange={(e) => setFavoritesOnly(e.target.checked)}
              />
              {t("streams.favoritesOnly")}
            </label>
          </div>

          {filteredStreams.length === 0 ? (
            <div className="nc-empty">
              {favoritesOnly ? t("streams.favoritesEmpty") : t("jetstream.empty")}
            </div>
          ) : (
            <div className="table-wrap">
              <VirtualTable
                columns={streamColumns}
                items={filteredStreams}
                empty={t("jetstream.empty")}
                getKey={getStreamKey}
                renderCell={renderStreamCell}
              />
            </div>
          )}
        </>
      )}

      {section === "consumers" && (
        <>
          <p className="text-muted mb-12">{t("jetstream.consumersHint")}</p>
          {streamsWithConsumers.length === 0 ? (
            <div className="nc-empty">{t("jetstream.consumersEmpty")}</div>
          ) : (
            <div className="table-wrap">
              <VirtualTable
                columns={consumerColumns}
                items={streamsWithConsumers}
                empty={t("jetstream.consumersEmpty")}
                getKey={getStreamKey}
                renderCell={renderConsumerCell}
              />
            </div>
          )}
        </>
      )}

      {section === "messages" && (
        <>
          <p className="text-muted mb-12">{t("jetstream.messagesHint")}</p>
          {streamsWithMessages.length === 0 ? (
            <div className="nc-empty">{t("jetstream.messagesEmpty")}</div>
          ) : (
            <div className="table-wrap">
              <VirtualTable
                columns={messageColumns}
                items={streamsWithMessages}
                empty={t("jetstream.messagesEmpty")}
                getKey={getStreamKey}
                renderCell={renderMessageCell}
              />
            </div>
          )}
        </>
      )}
    </div>
  );
}
