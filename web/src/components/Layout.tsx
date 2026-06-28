import { lazy, Suspense, useEffect, useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import ThemeSwitcher from "./ThemeSwitcher";
import NavIcon from "./ui/NavIcon";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";

import { STORAGE_KEYS } from "../lib/constants";

const AssistantPanel = lazy(() => import("./AssistantPanel"));

const SIDEBAR_STORAGE_KEY = STORAGE_KEYS.sidebar;

function readSidebarOpen(): boolean {
  try {
    if (typeof window !== "undefined" && window.matchMedia("(max-width: 900px)").matches) {
      return false;
    }
    const value = localStorage.getItem(SIDEBAR_STORAGE_KEY);
    if (value === "0" || value === "false") return false;
    return true;
  } catch {
    return true;
  }
}

function MenuIcon() {
  return (
    <svg className="sidebar-toggle__icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
      <path d="M4 7h16M4 12h16M4 17h16" strokeLinecap="round" />
    </svg>
  );
}

function ChevronLeftIcon() {
  return (
    <svg className="sidebar-toggle__icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
      <path d="M15 18l-6-6 6-6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function SidebarLink({ to, end, icon, children }: { to: string; end?: boolean; icon: Parameters<typeof NavIcon>[0]["name"]; children: string }) {
  return (
    <NavLink to={to} end={end} className={({ isActive }) => `nav-link${isActive ? " active" : ""}`} title={children}>
      <NavIcon name={icon} />
      <span>{children}</span>
    </NavLink>
  );
}

export default function Layout() {
  const location = useLocation();
  const { clusters, clusterId, setClusterId, loading, error, cluster } = useCluster();
  const { user, logout, isAdmin, isRoot, canManageUsers, canViewAudit } = useAuth();
  const [sidebarOpen, setSidebarOpen] = useState(readSidebarOpen);
  const avatarInitial = (user?.username?.[0] ?? "?").toUpperCase();
  const roleLabel = isRoot
    ? "Root"
    : isAdmin
      ? "Administrator"
      : user?.roles?.includes("operator")
        ? "Operator"
        : "Viewer";

  useEffect(() => {
    localStorage.setItem(SIDEBAR_STORAGE_KEY, sidebarOpen ? "1" : "0");
  }, [sidebarOpen]);

  useEffect(() => {
    if (window.matchMedia("(max-width: 900px)").matches) {
      setSidebarOpen(false);
    }
  }, [location.pathname]);

  useEffect(() => {
    const narrowViewportMediaQuery = window.matchMedia("(max-width: 900px)");
    function onViewportChange(event: MediaQueryListEvent) {
      if (event.matches) {
        setSidebarOpen(false);
      } else {
        setSidebarOpen(readSidebarOpen());
      }
    }
    narrowViewportMediaQuery.addEventListener("change", onViewportChange);
    return () => narrowViewportMediaQuery.removeEventListener("change", onViewportChange);
  }, []);

  function toggleSidebar() {
    setSidebarOpen((isOpen) => !isOpen);
  }

  return (
    <div className={`layout${sidebarOpen ? " layout--sidebar-open" : " layout--sidebar-collapsed"}`}>
      {sidebarOpen && (
        <button
          type="button"
          className="sidebar-backdrop"
          aria-label="Close sidebar"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      <aside className="sidebar" aria-hidden={!sidebarOpen}>
        <button
          type="button"
          className="sidebar-toggle sidebar-toggle--edge"
          aria-label="Close sidebar"
          onClick={toggleSidebar}
        >
          <ChevronLeftIcon />
        </button>
        <div className="sidebar__inner">
          <div className="sidebar__header">
            <div className="brand">
              <span className="brand__icon">
                <span className="brand__mark">NC</span>
              </span>
              <div className="brand__text">
                <span className="brand__name">NATS Consol</span>
                <span className="brand__tagline">JetStream console</span>
              </div>
            </div>
          </div>

        <div className="cluster-picker">
          <label htmlFor="cluster-select">Active cluster</label>
          <div className="cluster-picker__row">
            <span className="cluster-picker__dot" aria-hidden />
            <select
              id="cluster-select"
              value={clusterId ?? ""}
              onChange={(event) => setClusterId(event.target.value)}
              disabled={loading || clusters.length === 0}
            >
              {clusters.map((cluster) => (
                <option key={cluster.id} value={cluster.id}>
                  {cluster.name}
                  {cluster.isDefault ? " · default" : ""}
                </option>
              ))}
            </select>
          </div>
          {cluster && <p className="cluster-picker__meta">{cluster.natsUrl}</p>}
        </div>

        {error && <div className="sidebar-error">{error}</div>}

        <nav className="sidebar-nav">
          <div className="nav-section">
            <div className="nav-section__label">Overview</div>
            <SidebarLink to="/" end icon="dashboard">
              Dashboard
            </SidebarLink>
            <SidebarLink to="/clusters" icon="clusters">
              Clusters
            </SidebarLink>
          </div>

          <div className="nav-section">
            <div className="nav-section__label">JetStream</div>
            <SidebarLink to="/topology" icon="topology">
              Topology
            </SidebarLink>
            <SidebarLink to="/supercluster" icon="supercluster">
              Supercluster
            </SidebarLink>
            <SidebarLink to="/streams" icon="streams">
              Streams
            </SidebarLink>
            <SidebarLink to="/kv" icon="kv">
              KV Stores
            </SidebarLink>
            <SidebarLink to="/objects" icon="objects">
              Object Stores
            </SidebarLink>
          </div>

          <div className="nav-section">
            <div className="nav-section__label">Administration</div>
            {canViewAudit && (
              <SidebarLink to="/audit" icon="audit">
                Audit Log
              </SidebarLink>
            )}
            {canManageUsers && (
              <SidebarLink to="/users" icon="users">
                Users &amp; Roles
              </SidebarLink>
            )}
            {isAdmin && (
              <SidebarLink to="/profiling" icon="profiling">
                Profiling
              </SidebarLink>
            )}
          </div>
        </nav>

        <div className="sidebar__footer">
          <div className="user-pill">
            <span className="user-pill__avatar" aria-hidden>
              {avatarInitial}
            </span>
            <div className="user-pill__info">
              <span className="user-pill__name">{user?.username}</span>
              <span className="user-pill__role">{roleLabel}</span>
            </div>
            <button
              className="btn btn--ghost btn--small"
              type="button"
              onClick={() => logout().then(() => (window.location.href = "/login"))}
            >
              Sign out
            </button>
          </div>
        </div>
        </div>
      </aside>

      <main className="content">
        <div className="content-topbar">
          <div className="content-topbar__start">
            <button
              type="button"
              className="sidebar-toggle sidebar-toggle--open"
              aria-label={sidebarOpen ? "Close sidebar" : "Open sidebar"}
              aria-expanded={sidebarOpen}
              onClick={toggleSidebar}
            >
              {sidebarOpen ? <ChevronLeftIcon /> : <MenuIcon />}
              <span className="sidebar-toggle__text">Show menu</span>
            </button>
          </div>
          <div className="content-topbar__actions">
            <ThemeSwitcher />
          </div>
        </div>
        <div key={location.pathname} className="content__inner page-enter">
          <Outlet />
        </div>
        <Suspense fallback={null}>
          <AssistantPanel />
        </Suspense>
      </main>
    </div>
  );
}
