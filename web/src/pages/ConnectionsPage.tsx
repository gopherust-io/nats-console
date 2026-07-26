import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import Alert from "../components/ui/Alert";
import { api, clusterPath } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { clusterQueryKey, visibilityAwareInterval } from "../lib/query";

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
  in_msgs?: number;
  out_msgs?: number;
  in_bytes?: number;
  out_bytes?: number;
  subscriptions?: number;
  lang?: string;
  version?: string;
  name?: string;
  account?: string;
};

type ConnzResponse = {
  connections?: ConnzConnection[];
  num_connections?: number;
  total?: number;
};

export default function ConnectionsPage() {
  const { t } = useTranslation();
  const { clusterId } = useCluster();
  const [groupBy, setGroupBy] = useState("none");
  const [colorBy, setColorBy] = useState("rtt");
  const [sortBy, setSortBy] = useState("start");
  const [filter, setFilter] = useState("");
  const [showSubs, setShowSubs] = useState(false);

  const connzQuery = useQuery({
    queryKey: clusterQueryKey(clusterId, showSubs ? "connz-subs" : "connz"),
    queryFn: () =>
      api<ConnzResponse>(
        clusterPath(
          clusterId!,
          showSubs ? "/monitoring/connz?limit=1024&subs=1" : "/monitoring/connz?limit=1024",
        ),
      ),
    enabled: Boolean(clusterId),
    refetchInterval: visibilityAwareInterval(30_000),
  });

  const connections = useMemo(() => {
    let list = [...(connzQuery.data?.connections ?? [])];
    const q = filter.trim().toLowerCase();
    if (q) {
      list = list.filter(
        (c) =>
          c.name?.toLowerCase().includes(q) ||
          c.ip?.toLowerCase().includes(q) ||
          c.lang?.toLowerCase().includes(q) ||
          String(c.cid ?? "").includes(q),
      );
    }
    list.sort((a, b) => {
      if (sortBy === "rtt") return String(a.rtt ?? "").localeCompare(String(b.rtt ?? ""));
      if (sortBy === "name") return String(a.name ?? "").localeCompare(String(b.name ?? ""));
      return String(a.start ?? "").localeCompare(String(b.start ?? ""));
    });
    return list;
  }, [connzQuery.data, filter, sortBy]);

  const grouped = useMemo(() => {
    if (groupBy === "none") return { All: connections };
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
  }, [connections, groupBy]);

  return (
    <div>
      <div className="nc-page-header">
        <div className="nc-page-header__text">
          <h1 className="nc-page-title">{t("connections.title")}</h1>
          <p className="nc-page-sub">{t("connections.subtitle")}</p>
        </div>
      </div>

      {connzQuery.error instanceof Error && <Alert variant="error">{connzQuery.error.message}</Alert>}

      <div className="nc-conn-panel">
        <div className="nc-honeycomb" aria-hidden />
        <div className="nc-conn-controls">
          <h3 style={{ marginTop: 0 }}>{t("connections.view")}</h3>
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
            {t("connections.colorBy")}
            <select value={colorBy} onChange={(e) => setColorBy(e.target.value)}>
              <option value="rtt">{t("connections.rtt")}</option>
              <option value="status">{t("connections.status")}</option>
            </select>
          </label>
          <label>
            {t("connections.sortBy")}
            <select value={sortBy} onChange={(e) => setSortBy(e.target.value)}>
              <option value="start">{t("connections.startTime")}</option>
              <option value="rtt">{t("connections.rtt")}</option>
              <option value="name">{t("common.name")}</option>
            </select>
          </label>
          <div className="nc-menu__sep" />
          <h3>{t("connections.filters")}</h3>
          <label>
            {t("connections.search")}
            <input
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder={t("connections.searchPlaceholder")}
            />
          </label>
          <label style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <input
              type="checkbox"
              checked={showSubs}
              onChange={(e) => setShowSubs(e.target.checked)}
            />
            {t("connections.includeSubs", { defaultValue: "Include subscriptions" })}
          </label>
          <p className="text-muted" style={{ fontSize: "0.8rem" }}>
            {t("connections.colorModeNote", {
              mode: colorBy,
              count: connzQuery.data?.num_connections ?? connections.length,
            })}
          </p>
        </div>
      </div>

      {connections.length === 0 ? (
        <div className="nc-empty">{t("connections.empty")}</div>
      ) : (
        Object.entries(grouped).map(([group, rows]) => (
          <div key={group} className="table-wrap mt-16">
            {groupBy !== "none" && <div className="card-label" style={{ padding: "12px 16px" }}>{group}</div>}
            <table>
              <thead>
                <tr>
                  <th>{t("connections.cid")}</th>
                  <th>{t("common.name")}</th>
                  <th>{t("connections.ip")}</th>
                  <th>{t("connections.lang")}</th>
                  <th>{t("connections.rtt")}</th>
                  <th>{t("connections.subs")}</th>
                  <th>{t("connections.inOutMsgs")}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((c) => (
                  <tr key={c.cid}>
                    <td>{c.cid}</td>
                    <td>{c.name || t("common.emDash")}</td>
                    <td>
                      {c.ip}:{c.port}
                    </td>
                    <td>
                      {c.lang} {c.version}
                    </td>
                    <td>{c.rtt || t("common.emDash")}</td>
                    <td>{c.subscriptions ?? 0}</td>
                    <td>
                      {c.in_msgs ?? 0} / {c.out_msgs ?? 0}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ))
      )}
    </div>
  );
}
