import { useCallback, useDeferredValue, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import Pager, { DEFAULT_PAGE_SIZE } from "../components/Pager";
import Alert from "../components/ui/Alert";
import ClockNumber from "../components/ui/ClockNumber";
import PageLoader from "../components/ui/PageLoader";
import QueryErrorState from "../components/ui/QueryErrorState";
import VirtualTable, { type VirtualTableColumn } from "../components/VirtualTable";
import { useConnzEvents } from "../hooks/useConnzEvents";
import { useMediaQuery } from "../hooks/useMediaQuery";
import { api, clusterPath } from "../lib/api";
import { useCluster } from "../lib/cluster";
import {
  connectionUsername,
  formatConnectedSince,
  formatRttDisplay,
  isSlowConsumerConnection,
  parseRttMs,
} from "../lib/connectionInspector";
import { clusterQueryKey, queryClient, visibilityAwareInterval } from "../lib/query";
import "../styles/replicas.css";
import "../styles/connections.css";

const CONN_TABLE_MAX_HEIGHT = 720;
const COL_NARROW = new Set(["published", "received"]);
const COL_MEDIUM = new Set(["published", "received"]);

type ConnzConnection = {
  cid?: number;
  ip?: string;
  port?: number;
  start?: string;
  last_activity?: string;
  rtt?: string;
  uptime?: string;
  idle?: string;
  pending_bytes?: number;
  stalls?: number;
  in_msgs?: number;
  out_msgs?: number;
  in_bytes?: number;
  out_bytes?: number;
  lang?: string;
  version?: string;
  name?: string;
  account?: string;
  user?: string;
  authorized_user?: string;
  slow_consumer?: boolean;
  is_slow_consumer?: boolean;
  reason?: string;
};

type ConnzResponse = {
  now?: string;
  connections?: ConnzConnection[];
  num_connections?: number;
  total?: number;
};

function connectionKey(c: ConnzConnection, index: number) {
  return String(c.cid ?? index);
}

export default function ConnectionsPage() {
  const { t } = useTranslation();
  const { clusterId: routeCluster } = useParams();
  const { clusterId: contextClusterId } = useCluster();
  const clusterId = routeCluster ?? contextClusterId;
  const [groupBy, setGroupBy] = useState("none");
  const [sortBy, setSortBy] = useState("start");
  const [filter, setFilter] = useState("");
  const deferredFilter = useDeferredValue(filter);
  const [offset, setOffset] = useState(0);
  const limit = DEFAULT_PAGE_SIZE;
  const isNarrow = useMediaQuery("(max-width: 720px)");
  const isCompact = useMediaQuery("(max-width: 1100px)");

  const { live } = useConnzEvents(clusterId);

  useEffect(() => {
    setOffset(0);
  }, [clusterId, filter, sortBy, groupBy]);

  const connzQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "connz"),
    queryFn: async () => {
      const next = (
        await api<ConnzResponse>(
          // Prefer hub/SSE cache; omit fresh=1 so monitoring hub can serve.
          clusterPath(clusterId!, "/monitoring/connz?limit=1024&auth=1"),
          { cache: "no-store" },
        )
      ).data;
      const current = queryClient.getQueryData<ConnzResponse>(clusterQueryKey(clusterId, "connz"));
      const curMs = current?.now ? Date.parse(current.now) : 0;
      const nextMs = next?.now ? Date.parse(next.now) : 0;
      // Do not let a slow REST response rewind a newer SSE connect/disconnect frame.
      if (curMs > 0 && nextMs > 0 && nextMs < curMs) {
        return current!;
      }
      return next;
    },
    enabled: Boolean(clusterId),
    staleTime: 1_000,
    // While SSE is live, do not REST-poll — a slow response can overwrite a
    // newer connect/disconnect frame. useConnzEvents resumes REST on SSE death.
    refetchInterval: live ? false : visibilityAwareInterval(2_000),
    refetchOnWindowFocus: !live,
    // Do not keepPreviousData across cluster keys — auth identities from the
    // prior cluster would flash under the new system URL.
  });

  const connections = useMemo(() => {
    const source = connzQuery.data?.connections ?? [];
    const q = deferredFilter.trim().toLowerCase();
    const list = q ? source.filter((c) => {
        const user = connectionUsername(c)?.toLowerCase() ?? "";
        return (
          c.name?.toLowerCase().includes(q) ||
          c.ip?.toLowerCase().includes(q) ||
          c.lang?.toLowerCase().includes(q) ||
          c.account?.toLowerCase().includes(q) ||
          user.includes(q) ||
          String(c.cid ?? "").includes(q)
        );
      }) : source.slice();
    list.sort((a, b) => {
      if (sortBy === "rtt") {
        const aMs = parseRttMs(a.rtt);
        const bMs = parseRttMs(b.rtt);
        if (aMs == null && bMs == null) return 0;
        if (aMs == null) return 1;
        if (bMs == null) return -1;
        return bMs - aMs;
      }
      if (sortBy === "name") return String(a.name ?? "").localeCompare(String(b.name ?? ""));
      if (sortBy === "published") return (b.in_msgs ?? 0) - (a.in_msgs ?? 0);
      if (sortBy === "received") return (b.out_msgs ?? 0) - (a.out_msgs ?? 0);
      return String(b.start ?? "").localeCompare(String(a.start ?? ""));
    });
    return list;
  }, [connzQuery.data, deferredFilter, sortBy]);

  const pageRows = useMemo(() => {
    if (groupBy !== "none") return connections;
    return connections.slice(offset, offset + limit);
  }, [connections, groupBy, limit, offset]);

  const grouped = useMemo(() => {
    if (groupBy === "none") return { All: pageRows };
    const map: Record<string, ConnzConnection[]> = {};
    for (const c of connections) {
      const key =
        groupBy === "lang"
          ? c.lang || "unknown"
          : groupBy === "account"
            ? c.account || "default"
            : c.name || "unnamed";
      (map[key] ??= []).push(c);
    }
    return map;
  }, [connections, groupBy, pageRows]);

  const slowCount = useMemo(
    () => connections.filter((c) => isSlowConsumerConnection(c)).length,
    [connections],
  );

  const showing = connections.length;

  const columns = useMemo<VirtualTableColumn[]>(() => {
    const hide = isNarrow ? COL_NARROW : isCompact ? COL_MEDIUM : null;
    // Share leftover width across tracks so columns stay evenly spaced (name alone
    // must not absorb the whole free space as a single fr).
    const all: VirtualTableColumn[] = [
      { id: "name", header: t("common.name"), width: "minmax(10rem, 1.35fr)", cellClassName: "virtual-table__truncate" },
      { id: "slow", header: t("connections.status"), width: "minmax(5.5rem, 0.7fr)" },
      { id: "ip", header: t("connections.ip"), width: "minmax(9.5rem, 1.1fr)", cellClassName: "mono" },
      { id: "rtt", header: t("connections.rtt"), width: "minmax(4.5rem, 0.55fr)", cellClassName: "mono nc-conn-rtt" },
      { id: "connected", header: t("connections.connectedSince"), width: "minmax(7rem, 0.9fr)", cellClassName: "virtual-table__truncate" },
      { id: "published", header: t("connections.messagesPublished"), width: "minmax(4.75rem, 0.55fr)", align: "right", cellClassName: "mono" },
      { id: "received", header: t("connections.messagesReceived"), width: "minmax(5rem, 0.55fr)", align: "right", cellClassName: "mono" },
    ];
    return hide ? all.filter((c) => !hide.has(c.id)) : all;
  }, [isCompact, isNarrow, t]);

  const renderCell = useCallback(
    (c: ConnzConnection, columnId: string) => {
      switch (columnId) {
        case "slow": {
          const slow = isSlowConsumerConnection(c);
          const label = slow ? t("connections.slow") : t("connections.healthy");
          return (
            <span className="replicas-peer-name">
              <span
                className={`replicas-status-dot replicas-status-dot--${slow ? "warn" : "ok"}`}
                title={label}
                aria-label={label}
              />
              {label}
            </span>
          );
        }
        case "name": {
          const meta = [c.lang, c.version].filter(Boolean).join(" ");
          return (
            <div className="nc-conn-identity">
              <div className="nc-conn-identity__primary">
                <span className="nc-conn-identity__name" title={c.name || undefined}>
                  {c.name || t("common.emDash")}
                </span>
              </div>
              {meta ? <span className="nc-conn-identity__meta mono">{meta}</span> : null}
            </div>
          );
        }
        case "ip": {
          const endpoint = c.ip ? `${c.ip}:${c.port}` : t("common.emDash");
          return (
            <span className="mono" title={endpoint}>
              {endpoint}
            </span>
          );
        }
        case "rtt":
          return formatRttDisplay(c.rtt);
        case "connected":
          return (
            <span className="virtual-table__truncate" title={c.start ? new Date(c.start).toLocaleString() : undefined}>
              {formatConnectedSince(c.start)}
            </span>
          );
        case "published":
          return <ClockNumber value={c.in_msgs ?? 0} />;
        case "received":
          return <ClockNumber value={c.out_msgs ?? 0} />;
        default:
          return null;
      }
    },
    [t],
  );

  const getKey = useCallback((c: ConnzConnection, index: number) => connectionKey(c, index), []);

  const reported = connzQuery.data?.num_connections ?? connzQuery.data?.total ?? connections.length;
  const fetched = connzQuery.data?.connections?.length ?? 0;
  const truncated = fetched > 0 && reported > fetched && !filter.trim();
  const healthyCount = Math.max(0, showing - slowCount);
  const healthyPct =
    showing > 0 ? Math.min(100, Math.round((healthyCount / showing) * 100)) : 0;
  const healthyTone =
    showing === 0
      ? "ok"
      : slowCount === 0
        ? "ok"
        : slowCount >= showing / 2
          ? "danger"
          : "warn";
  const tableMaxHeight = Math.min(
    typeof window !== "undefined" ? Math.round(window.innerHeight * 0.7) : CONN_TABLE_MAX_HEIGHT,
    CONN_TABLE_MAX_HEIGHT,
  );

  if (connzQuery.isPending && !connzQuery.data) {
    return <PageLoader />;
  }

  return (
    <div className="replicas-page connections-page">
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <div className="nc-conn-header__title-row">
            <h1 className="nc-page-title">{t("connections.title")}</h1>
            {live && !connzQuery.isError ? (
              <span className="nc-conn-live" title={t("connections.liveHint")}>
                <span className="nc-conn-live__dot" aria-hidden="true" />
                {t("connections.live")}
              </span>
            ) : null}
          </div>
          <p className="nc-page-sub">{t("connections.subtitle")}</p>
        </div>
      </div>

      {connzQuery.isError && (
        <QueryErrorState error={connzQuery.error} onRetry={() => void connzQuery.refetch()} />
      )}
      {truncated && (
        <Alert variant="info">
          {t("connections.truncatedBanner", { showing: fetched, total: reported })}
        </Alert>
      )}

      <div className="replicas-summary">
        <div className="replicas-card replicas-online-card">
          <span
            className={`replicas-card__badge${healthyTone === "danger" ? " replicas-card__badge--down" : ""}`}
          >
            {healthyTone === "danger" ? t("connections.slow") : t("connections.healthy")}
          </span>
          <div className="replicas-card__body">
            <div
              className={`replicas-gauge replicas-gauge--${healthyTone}`}
              style={{ ["--gauge-pct" as string]: healthyPct }}
              role="img"
              aria-label={t("connections.healthyGaugeAria", {
                healthy: healthyCount,
                total: showing,
              })}
            >
              <span className="replicas-gauge__value mono">
                <ClockNumber value={healthyCount} />
                <span className="replicas-gauge__sep">/</span>
                <ClockNumber value={showing} />
              </span>
            </div>
            {showing > 0 && slowCount > 0 ? (
              <p className="replicas-online-card__quorum mono">
                {t("connections.slowLine", { slow: slowCount })}
              </p>
            ) : null}
          </div>
        </div>
        <div className="replicas-card replicas-stat-card">
          <span className="replicas-card__badge">{t("connections.showing")}</span>
          <div className="replicas-card__body">
            <div className="replicas-stat-card__value mono">
              <ClockNumber value={showing} />
              <span className="connections-stat__of">
                {" / "}
                <ClockNumber value={reported} />
              </span>
            </div>
          </div>
        </div>
        <div className="replicas-card replicas-stat-card">
          <span className="replicas-card__badge">{t("connections.healthy")}</span>
          <div className="replicas-card__body">
            <div className="replicas-stat-card__value mono">
              <ClockNumber value={healthyCount} />
            </div>
          </div>
        </div>
        <div className="replicas-card replicas-stat-card">
          <span className="replicas-card__badge">{t("connections.slow")}</span>
          <div className="replicas-card__body">
            <div
              className={`replicas-stat-card__value mono${slowCount > 0 ? " connections-stat__value--warn" : ""}`}
            >
              <ClockNumber value={slowCount} />
            </div>
          </div>
        </div>
      </div>

      <div className="replicas-panel replicas-table-panel">
        <div className="replicas-panel__head connections-panel__head">
          <h2 className="replicas-panel__title">{t("connections.listTitle")}</h2>
          <div className="nc-conn-controls connections-panel__controls">
            <label>
              {t("connections.groupBy")}
              <select value={groupBy} onChange={(e) => setGroupBy(e.target.value)}>
                <option value="none">{t("connections.none")}</option>
                <option value="lang">{t("connections.language")}</option>
                <option value="account">{t("connections.account")}</option>
                <option value="name">{t("common.name")}</option>
              </select>
            </label>
            <label>
              {t("connections.sortBy")}
              <select value={sortBy} onChange={(e) => setSortBy(e.target.value)}>
                <option value="start">{t("connections.connectedSince")}</option>
                <option value="rtt">{t("connections.rtt")}</option>
                <option value="name">{t("common.name")}</option>
                <option value="published">{t("connections.messagesPublished")}</option>
                <option value="received">{t("connections.messagesReceived")}</option>
              </select>
            </label>
            <label className="nc-conn-controls__search">
              {t("connections.search")}
              <input
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                placeholder={t("connections.searchPlaceholder")}
              />
            </label>
          </div>
        </div>

        {connections.length === 0 ? (
          <p className="text-muted">{t("connections.empty")}</p>
        ) : (
          <>
            {Object.entries(grouped).map(([group, rows]) => (
              <div key={group} className="table-wrap table-wrap--fit replicas-table-wrap nc-conn-table-wrap">
                {groupBy !== "none" && <div className="card-label">{group}</div>}
                <VirtualTable
                  columns={columns}
                  items={rows}
                  empty={t("connections.empty")}
                  getKey={getKey}
                  renderCell={renderCell}
                  rowHeight={64}
                  maxHeight={tableMaxHeight}
                  overflowX="hidden"
                />
              </div>
            ))}
            {groupBy === "none" ? (
              <Pager total={connections.length} offset={offset} limit={limit} onPageChange={setOffset} />
            ) : null}
          </>
        )}
      </div>
    </div>
  );
}
