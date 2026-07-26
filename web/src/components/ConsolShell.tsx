import { lazy, Suspense, useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, NavLink, Outlet, useLocation, useNavigate, useParams } from "react-router";
import ThemeSwitcher from "./ThemeSwitcher";
import NotificationsBell from "./NotificationsBell";
import { useAccount } from "../lib/account";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import "../styles/consol-shell.css";

const AssistantPanel = lazy(() => import("./AssistantPanel"));

function isTopologyPath(pathname: string) {
  return pathname === "/admin/topology" || pathname === "/topology";
}

function UserMenu() {
  const { t } = useTranslation();
  const { user, logout, canManageUsers, canViewAudit, canManageAlertRules } = useAuth();
  const [open, setOpen] = useState(false);
  const [closing, setClosing] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const openRef = useRef(open);
  const closingRef = useRef(closing);
  openRef.current = open;
  closingRef.current = closing;

  const clearCloseTimer = useCallback(() => {
    if (closeTimer.current) {
      clearTimeout(closeTimer.current);
      closeTimer.current = null;
    }
  }, []);

  const requestClose = useCallback(() => {
    if (!openRef.current || closingRef.current) return;
    setClosing(true);
    clearCloseTimer();
    closeTimer.current = setTimeout(() => {
      setOpen(false);
      setClosing(false);
      closeTimer.current = null;
    }, 150);
  }, [clearCloseTimer]);

  const requestOpen = useCallback(() => {
    clearCloseTimer();
    setClosing(false);
    setOpen(true);
  }, [clearCloseTimer]);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (!ref.current?.contains(e.target as Node)) requestClose();
    }
    document.addEventListener("mousedown", onDoc);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      clearCloseTimer();
    };
  }, [requestClose, clearCloseTimer]);

  const visible = open || closing;

  return (
    <div className="nc-menu" ref={ref}>
      <button
        type="button"
        className="nc-icon-btn"
        aria-label={t("nav.openUserMenu")}
        aria-expanded={open && !closing}
        onClick={() => (visible && !closing ? requestClose() : requestOpen())}
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
          <circle cx="12" cy="8" r="3.5" />
          <path d="M5 19c1.5-3.5 4-5 7-5s5.5 1.5 7 5" strokeLinecap="round" />
        </svg>
      </button>
      {visible && (
        <div className="nc-menu__panel" role="menu" data-state={closing ? "closed" : "open"}>
          <div className="nc-menu__item" style={{ cursor: "default", opacity: 0.8 }}>
            {user?.username}
          </div>
          <div className="nc-menu__sep" />
          {canViewAudit && (
            <Link className="nc-menu__item" to="/admin/audit" onClick={requestClose}>
              {t("nav.auditLog")}
            </Link>
          )}
          <Link className="nc-menu__item" to="/admin/alerts" onClick={requestClose}>
            {t("nav.alerts")}
          </Link>
          {canManageAlertRules && (
            <Link className="nc-menu__item" to="/admin/alert-rules" onClick={requestClose}>
              {t("nav.alertRules")}
            </Link>
          )}
          {canManageUsers && (
            <Link className="nc-menu__item" to="/admin/users" onClick={requestClose}>
              {t("nav.people")}
            </Link>
          )}
          <a className="nc-menu__item" href="/api/openapi.yaml" target="_blank" rel="noreferrer">
            {t("nav.apiDocs")}
          </a>
          <div className="nc-menu__sep" />
          <button
            type="button"
            className="nc-menu__item"
            onClick={() => logout().then(() => (window.location.href = "/login"))}
          >
            {t("nav.signOut")}
          </button>
        </div>
      )}
    </div>
  );
}

function Breadcrumbs() {
  const { t } = useTranslation();
  const { clusters, clusterId, setClusterId } = useCluster();
  const { accounts, accountName, setAccountName } = useAccount();
  const { clusterId: routeClusterId, accountName: routeAccount } = useParams();
  const navigate = useNavigate();
  const location = useLocation();

  const onSystems = location.pathname === "/systems" || location.pathname === "/";
  const onTopology = isTopologyPath(location.pathname);
  const onAdmin = location.pathname.startsWith("/admin") && !onTopology;
  // Topology lives under /admin but belongs with account JetStream; keep system/account crumbs.
  const activeClusterId = routeClusterId ?? (onTopology ? clusterId : null);
  const activeAccount = routeAccount ?? (onTopology ? accountName : null);
  const onSystem = Boolean(activeClusterId) && !activeAccount;
  const onAccount = Boolean(activeClusterId && activeAccount);

  return (
    <div className="nc-breadcrumbs">
      <div className="nc-crumb">
        <Link to="/systems">{t("common.brand")}</Link>
      </div>
      {onAdmin && (
        <>
          <span className="nc-crumb__sep">/</span>
          <div className="nc-crumb">{t("common.admin")}</div>
        </>
      )}
      {(onSystem || onAccount) && (
        <>
          <span className="nc-crumb__sep">/</span>
          <div className="nc-crumb">
            <select
              aria-label={t("nav.systemSelect")}
              value={activeClusterId ?? ""}
              onChange={(e) => {
                setClusterId(e.target.value);
                navigate(`/systems/${e.target.value}`);
              }}
            >
              {clusters.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          </div>
        </>
      )}
      {onAccount && (
        <>
          <span className="nc-crumb__sep">/</span>
          <div className="nc-crumb">
            <select
              aria-label={t("nav.accountSelect")}
              value={activeAccount ?? accountName}
              onChange={(e) => {
                setAccountName(e.target.value);
                navigate(`/systems/${activeClusterId}/accounts/${encodeURIComponent(e.target.value)}`);
              }}
            >
              {accounts.map((a) => (
                <option key={a.name} value={a.name}>
                  {a.name}
                </option>
              ))}
            </select>
          </div>
        </>
      )}
      {onSystems && clusters.length === 0 && <span className="text-muted">{t("nav.noSystems")}</span>}
    </div>
  );
}

function LevelTabs() {
  const { t } = useTranslation();
  const { clusterId: routeClusterId, accountName: routeAccount } = useParams();
  const { clusterId } = useCluster();
  const { accountName } = useAccount();
  const location = useLocation();
  const onTopology = isTopologyPath(location.pathname);

  // Prefer route params; on Topology fall back to selected context so account tabs stay visible.
  const effectiveClusterId = routeClusterId ?? (onTopology ? clusterId ?? undefined : undefined);
  const effectiveAccount = routeAccount ?? (onTopology ? accountName : undefined);

  if (location.pathname.startsWith("/admin") && !onTopology) {
    return (
      <nav className="nc-tabs" aria-label={t("nav.ariaAdmin")}>
        <NavLink to="/admin/users" className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
          {t("nav.people")}
        </NavLink>
        <NavLink to="/admin/audit" className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
          {t("nav.audit")}
        </NavLink>
        <NavLink to="/admin/alerts" className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
          {t("nav.alerts")}
        </NavLink>
      </nav>
    );
  }

  if (!effectiveClusterId) {
    return (
      <nav className="nc-tabs" aria-label={t("nav.ariaTeam")}>
        <NavLink to="/systems" end className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
          {t("nav.systems")}
        </NavLink>
        <NavLink to="/systems/clusters" className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
          {t("nav.clusters")}
        </NavLink>
        <NavLink to="/admin/alerts" className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
          {t("nav.settings")}
        </NavLink>
      </nav>
    );
  }

  if (!effectiveAccount) {
    const base = `/systems/${effectiveClusterId}`;
    return (
      <nav className="nc-tabs" aria-label={t("nav.ariaSystem")}>
        <NavLink to={base} end className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
          {t("nav.overview")}
        </NavLink>
        <NavLink to={`${base}/usage`} className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
          {t("nav.usage")}
        </NavLink>
        <NavLink to={`${base}/access`} className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
          {t("nav.access")}
        </NavLink>
      </nav>
    );
  }

  const base = `/systems/${effectiveClusterId}/accounts/${encodeURIComponent(effectiveAccount)}`;
  return (
    <nav className="nc-tabs" aria-label={t("nav.ariaAccount")}>
      <NavLink to={base} end className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
        {t("nav.overview")}
      </NavLink>
      <NavLink to={`${base}/connections`} className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
        {t("nav.connections")}
      </NavLink>
      <NavLink to={`${base}/jetstream`} className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
        {t("nav.jetstream")}
      </NavLink>
      <NavLink to="/admin/topology" className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
        {t("nav.topology")}
      </NavLink>
      <NavLink to={`${base}/users`} className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
        {t("nav.users")}
      </NavLink>
      <NavLink to={`${base}/access`} className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
        {t("nav.access")}
      </NavLink>
      <NavLink to={`${base}/sharing`} className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
        {t("nav.sharing")}
      </NavLink>
      <NavLink to={`${base}/settings`} className={({ isActive }) => `nc-tab${isActive ? " active" : ""}`}>
        {t("nav.settings")}
      </NavLink>
    </nav>
  );
}

export default function ConsolShell() {
  const { t } = useTranslation();
  const location = useLocation();
  const { clusterId, setClusterId } = useCluster();
  const { accountName, setAccountName } = useAccount();
  const params = useParams();

  useEffect(() => {
    if (params.clusterId && params.clusterId !== clusterId) {
      setClusterId(params.clusterId);
    }
  }, [params.clusterId, clusterId, setClusterId]);

  useEffect(() => {
    if (params.accountName && params.accountName !== accountName) {
      setAccountName(decodeURIComponent(params.accountName));
    }
  }, [params.accountName, accountName, setAccountName]);

  return (
    <div className="nc-shell">
      <header className="nc-topbar">
        <Link to="/systems" className="nc-topbar__brand">
          <span className="brand__icon">
            <span className="brand__mark">NC</span>
          </span>
          <span>{t("common.brand")}</span>
        </Link>
        <div className="nc-topbar__actions">
          <ThemeSwitcher />
          <NotificationsBell />
          <UserMenu />
        </div>
      </header>

      <div className="nc-context">
        <Breadcrumbs />
        <LevelTabs />
      </div>

      <main className="nc-main">
        <div key={location.pathname} className="page-enter">
          <Outlet />
        </div>
      </main>

      <Suspense fallback={null}>
        <AssistantPanel />
      </Suspense>
    </div>
  );
}
