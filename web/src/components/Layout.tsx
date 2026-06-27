import { NavLink, Outlet } from "react-router-dom";
import ThemeSwitcher from "./ThemeSwitcher";
import { useCluster } from "../lib/cluster";

export default function Layout() {
  const { clusters, clusterId, setClusterId, loading, error } = useCluster();

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand__icon">NC</span>
          NATS Consol
        </div>

        <div className="cluster-picker">
          <label htmlFor="cluster-select">Cluster</label>
          <select
            id="cluster-select"
            value={clusterId ?? ""}
            onChange={(e) => setClusterId(e.target.value)}
            disabled={loading || clusters.length === 0}
          >
            {clusters.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
                {c.is_default ? " (default)" : ""}
              </option>
            ))}
          </select>
        </div>

        {error && <div className="sidebar-error">{error}</div>}

        <nav>
          <NavLink to="/" end className={({ isActive }) => `nav-link${isActive ? " active" : ""}`}>
            Dashboard
          </NavLink>
          <NavLink to="/clusters" className={({ isActive }) => `nav-link${isActive ? " active" : ""}`}>
            Clusters
          </NavLink>
          <NavLink to="/streams" className={({ isActive }) => `nav-link${isActive ? " active" : ""}`}>
            Streams
          </NavLink>
          <NavLink to="/kv" className={({ isActive }) => `nav-link${isActive ? " active" : ""}`}>
            KV Stores
          </NavLink>
          <NavLink to="/objects" className={({ isActive }) => `nav-link${isActive ? " active" : ""}`}>
            Object Stores
          </NavLink>
        </nav>

        <div className="sidebar__footer">
          <ThemeSwitcher />
        </div>
      </aside>
      <main className="content">
        <Outlet />
      </main>
    </div>
  );
}
