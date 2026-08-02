import { useCallback, useEffect, useMemo, useState } from "react";
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
  connectionTLSCipher,
  connectionUsername,
  formatConnectedSince,
  formatRttDisplay,
  formatTLSVersion,
  isSlowConsumerConnection,
  parseRttMs,
} from "../lib/connectionInspector";
import { clusterQueryKey } from "../lib/query";

const CONN_TABLE_MAX_HEIGHT = 720;
const COL_NARROW = new Set(["published", "received", "tls", "account"]);
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
  tls_version?: string;
  tls_cipher?: string;
  tls_cipher_suite?: string;
  tls_first?: boolean;
  slow_consumer?: boolean;
  is_slow_consumer?: boolean;
  reason?: string;
};

type ConnzResponse = {
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
  const [offset, setOffset] = useState(0);
  const limit = DEFAULT_PAGE_SIZE;
  const isNarrow = useMediaQuery("(max-width: 720px)");
  const isCompact = useMediaQuery("(max-width: 1100px)");

  useConnzEvents(clusterId);

  useEffect(() => {
    setOffset(0);
  }, [clusterId, filter, sortBy, groupBy]);

  const connzQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, "connz"),
    queryFn: async () =>
      (await api<ConnzResponse>(clusterPath(clusterId!, "/monitoring/connz?limit=1024&auth=1"))).data,
    enabled: Boolean(clusterId),
    staleTime: 5_000,
    refetchOnWindowFocus: false,
    // Do not keepPreviousData across cluster keys — auth identities from the
    // prior cluster would flash under the new system URL.
  });

  const connections = useMemo(() => {
    let list = [...(connzQuery.data?.connections ?? [])];
    const q = filter.trim().toLowerCase();
    if (q) {
      list = list.filter((c) => {
        const user = connectionUsername(c)?.toLowerCase() ?? "";
        return (
          c.name?.toLowerCase().includes(q) ||
          c.ip?.toLowerCase().includes(q) ||
          c.lang?.toLowerCase().includes(q) ||
          c.account?.toLowerCase().includes(q) ||
          user.includes(q) ||
          String(c.cid ?? "").includes(q)
        );
      });
    }
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
  }, [connzQuery.data, filter, sortBy]);

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
    const all: VirtualTableColumn[] = [
      { id: "name", header: t("common.name"), width: "minmax(0, 1.4fr)", cellClassName: "virtual-table__truncate" },
      { id: "slow", header: t("connections.status"), width: "104px" },
      { id: "ip", header: t("connections.ip"), width: "192px", cellClassName: "mono" },
      { id: "rtt", header: t("connections.rtt"), width: "88px", cellClassName: "mono nc-conn-rtt" },
      { id: "tls", header: t("connections.tlsVersion"), width: "64px" },
      { id: "user", header: t("common.username"), width: "minmax(0, 0.9fr)", cellClassName: "virtual-table__truncate" },
      { id: "account", header: t("connections.account"), width: "minmax(0, 0.9fr)", cellClassName: "virtual-table__truncate" },
      { id: "connected", header: t("connections.connectedSince"), width: "minmax(0, 140px)", cellClassName: "virtual-table__truncate" },
      { id: "published", header: t("connections.messagesPublished"), width: "80px", align: "right", cellClassName: "mono" },
      { id: "received", header: t("connections.messagesReceived"), width: "80px", align: "right", cellClassName: "mono" },
    ];
    return hide ? all.filter((c) => !hide.has(c.id)) : all;
  }, [isCompact, isNarrow, t]);

  const renderCell = useCallback(
    (c: ConnzConnection, columnId: string) => {
      const username = connectionUsername(c);
      switch (columnId) {
        case "slow": {
          const slow = isSlowConsumerConnection(c);
          return (
            <span className={`nc-conn-status${slow ? " nc-conn-status--warn" : " nc-conn-status--ok"}`}>
              <span className="nc-conn-status__dot" aria-hidden="true" />
              {slow ? t("connections.slow") : t("connections.healthy")}
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
        case "tls": {
          const cipher = connectionTLSCipher(c);
          return (
            <span title={cipher || formatTLSVersion(c)}>
              {c.tls_version || t("common.emDash")}
            </span>
          );
        }
        case "user":
          return (
            <span className="virtual-table__truncate" title={username}>
              {username || t("common.emDash")}
            </span>
          );
        case "account":
          return (
            <span className="virtual-table__truncate" title={c.account || undefined}>
              {c.account || t("common.emDash")}
            </span>
          );
        case "connected":
          return (
            <span className="virtual-table__truncate" title={c.start ? new Date(c.start).toLocaleString() : undefined}>
              {formatConnectedSince(c.start)}
            </span>
          );
        case "published":
          return String(c.in_msgs ?? 0);
        case "received":
          return String(c.out_msgs ?? 0);
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
  const isLive = connzQuery.isFetching || connzQuery.isSuccess;
  const tableMaxHeight = Math.min(
    typeof window !== "undefined" ? Math.round(window.innerHeight * 0.7) : CONN_TABLE_MAX_HEIGHT,
    CONN_TABLE_MAX_HEIGHT,
  );

  if (connzQuery.isPending && !connzQuery.data) {
    return <PageLoader />;
  }

  return (
    <div className="nc-conn-page">
      <div className="nc-page-header nc-conn-header">
        <div className="nc-page-header__text">
          <div className="nc-conn-header__title-row">
            <h1 className="nc-page-title">{t("connections.title")}</h1>
            {isLive && !connzQuery.isError ? (
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

      <div className="nc-conn-toolbar">
        <div className="nc-conn-telemetry" aria-label={t("connections.filters")}>
          <div className="nc-conn-telemetry__cell">
            <span className="nc-conn-telemetry__label">{t("connections.showing")}</span>
            <span className="nc-conn-telemetry__value mono">
              <ClockNumber value={showing} />
              <span className="nc-conn-telemetry__of">
                {" / "}
                <ClockNumber value={reported} />
              </span>
            </span>
          </div>
          <div className="nc-conn-telemetry__cell">
            <span className="nc-conn-telemetry__label">{t("connections.healthy")}</span>
            <span className="nc-conn-telemetry__value mono nc-conn-telemetry__value--ok">
              <ClockNumber value={healthyCount} />
            </span>
          </div>
          <div className="nc-conn-telemetry__cell">
            <span className="nc-conn-telemetry__label">{t("connections.slow")}</span>
            <span
              className={`nc-conn-telemetry__value mono${slowCount > 0 ? " nc-conn-telemetry__value--warn" : ""}`}
            >
              <ClockNumber value={slowCount} />
            </span>
          </div>
        </div>

        <div className="nc-conn-controls">
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
              <option value="start">{t("connections.startTime")}</option>
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
        <div className="nc-empty">{t("connections.empty")}</div>
      ) : (
        <>
          {Object.entries(grouped).map(([group, rows]) => (
            <div key={group} className="table-wrap table-wrap--fit nc-conn-table-wrap mt-16">
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
  );
}
