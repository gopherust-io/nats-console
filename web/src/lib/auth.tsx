import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { ApiError, api, AccessRules, clearAuth, UnauthorizedError, userFacingError } from "./api";

export type AuthGrant = {
  id?: string;
  userId?: string;
  resourceType: string;
  resourceKey: string;
  role: string;
};

export type AuthUser = {
  id?: string;
  username: string;
  email?: string;
  roles: string[];
  isRoot?: boolean;
  accessRules?: AccessRules;
  grants?: AuthGrant[];
};

type AuthContextValue = {
  user: AuthUser | null;
  loading: boolean;
  sessionError: string | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  reload: () => Promise<void>;
  canWrite: boolean;
  canWriteCluster: (clusterId?: string | null) => boolean;
  canManageJetStream: (clusterId?: string | null) => boolean;
  canManageSystemAccess: (clusterId: string) => boolean;
  canManageAccountAccess: (clusterId: string, accountName: string) => boolean;
  canDownloadCreds: (clusterId: string, accountName: string, natsUserId?: string) => boolean;
  isAdmin: boolean;
  isRoot: boolean;
  canManageUsers: boolean;
  canViewAudit: boolean;
  canDeleteClusters: boolean;
  canManageAlertRules: boolean;
};

const AuthContext = createContext<AuthContextValue | null>(null);

function hasRole(roles: string[], role: string) {
  return roles.includes(role);
}

function isLegacyFullAdmin(user: AuthUser) {
  return hasRole(user.roles, "admin") && !user.accessRules;
}

function allowsClusterRules(user: AuthUser, clusterId: string) {
  if (user.isRoot || isLegacyFullAdmin(user)) return true;
  return Boolean(user.accessRules?.clusterIds?.includes(clusterId));
}

function hasAdminGrant(user: AuthUser, resourceType: string, resourceKey: string) {
  return (user.grants ?? []).some(
    (g) => g.resourceType === resourceType && g.resourceKey === resourceKey && g.role === "admin",
  );
}

function accountResourceKey(clusterId: string, accountName: string) {
  return `${clusterId}:${accountName}`;
}

function natsUserResourceKey(clusterId: string, accountName: string, natsUserId: string) {
  return `${clusterId}:${accountName}:${natsUserId}`;
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation();
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [sessionError, setSessionError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    try {
      const me = await api<AuthUser>("/api/v1/auth/me");
      setUser(me.data);
      setSessionError(null);
    } catch (err) {
      if (err instanceof UnauthorizedError) {
        setUser(null);
        setSessionError(null);
        return;
      }
      if (err instanceof ApiError && err.code === "network") {
        setSessionError(userFacingError(err, t));
        return;
      }
      setUser(null);
      setSessionError(null);
    }
  }, [t]);

  useEffect(() => {
    reload().finally(() => setLoading(false));
  }, [reload]);

  const login = useCallback(async (username: string, password: string) => {
    const me = await api<AuthUser>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    });
    setUser(me.data);
    setSessionError(null);
  }, []);

  const logout = useCallback(async () => {
    try {
      await api("/api/v1/auth/logout", { method: "POST" });
    } catch {
      // ignore
    }
    clearAuth();
    setUser(null);
    setSessionError(null);
  }, []);

  const value = useMemo<AuthContextValue>(() => {
    const roles = user?.roles ?? [];
    const isRoot = Boolean(user?.isRoot);
    const rules = user?.accessRules;
    const legacyAdmin = hasRole(roles, "admin") && !rules;
    const canManageUsers = isRoot || rules?.manageUsers === true || legacyAdmin;
    const canViewAudit = isRoot || rules?.viewAudit === true || legacyAdmin;
    const canDeleteClusters = isRoot || rules?.deleteClusters === true || legacyAdmin;
    const canManageAlertRules = isRoot || legacyAdmin || canManageUsers;

    const canAccessCluster = (clusterId: string) => {
      if (!user) return false;
      if (allowsClusterRules(user, clusterId)) return true;
      return (user.grants ?? []).some((g) => g.resourceType === "system" && g.resourceKey === clusterId);
    };

    const canWriteCluster = (clusterId?: string | null) => {
      if (!user || !clusterId) return false;
      if (user.isRoot) return true;
      if (hasAdminGrant(user, "system", clusterId)) return true;
      const canWriteRole = hasRole(roles, "admin") || hasRole(roles, "operator");
      return canWriteRole && allowsClusterRules(user, clusterId);
    };

    const canManageJetStream = (clusterId?: string | null) => {
      if (!user || !clusterId) return false;
      if (user.isRoot) return true;
      if (hasAdminGrant(user, "system", clusterId)) return true;
      return hasRole(user.roles, "admin") && allowsClusterRules(user, clusterId);
    };

    const canManageSystemAccess = (clusterId: string) => {
      if (!user) return false;
      if (user.isRoot) return true;
      if (hasAdminGrant(user, "system", clusterId)) return true;
      return canManageUsers && canAccessCluster(clusterId);
    };

    const canManageAccountAccess = (clusterId: string, accountName: string) => {
      if (!user) return false;
      if (canManageSystemAccess(clusterId)) return true;
      return hasAdminGrant(user, "account", accountResourceKey(clusterId, accountName));
    };

    const canDownloadCreds = (clusterId: string, accountName: string, natsUserId?: string) => {
      if (!user) return false;
      if (user.isRoot) return true;
      if (hasAdminGrant(user, "system", clusterId)) return true;
      const accountKey = accountResourceKey(clusterId, accountName);
      const natsUserKey = natsUserId ? natsUserResourceKey(clusterId, accountName, natsUserId) : "";
      for (const g of user.grants ?? []) {
        if (g.role !== "admin" && g.role !== "credential_downloader") continue;
        if (g.resourceType === "account" && g.resourceKey === accountKey) return true;
        if (g.resourceType === "nats_user" && natsUserKey && g.resourceKey === natsUserKey) return true;
      }
      return hasRole(user.roles, "admin") && allowsClusterRules(user, clusterId);
    };

    return {
      user,
      loading,
      sessionError,
      login,
      logout,
      reload,
      canWrite: isRoot || hasRole(roles, "admin") || hasRole(roles, "operator"),
      canWriteCluster,
      canManageJetStream,
      canManageSystemAccess,
      canManageAccountAccess,
      canDownloadCreds,
      isAdmin: hasRole(roles, "admin"),
      isRoot,
      canManageUsers,
      canViewAudit,
      canDeleteClusters,
      canManageAlertRules,
    };
  }, [user, loading, sessionError, login, logout, reload]);

  return (
    <AuthContext.Provider value={value}>
      {sessionError && (
        <div className="nc-session-error" role="alert">
          <span>{sessionError || t("errors.sessionRecover")}</span>
          <button type="button" className="btn btn--small" onClick={() => void reload()}>
            {t("common.retry")}
          </button>
        </div>
      )}
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return ctx;
}
