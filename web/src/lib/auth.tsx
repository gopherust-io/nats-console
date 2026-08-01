import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { ApiError, api, AccessRules, clearAuth, UnauthorizedError, userFacingError } from "./api";

export type AuthUser = {
  id?: string;
  username: string;
  email?: string;
  roles: string[];
  isRoot?: boolean;
  accessRules?: AccessRules;
};

type AuthContextValue = {
  user: AuthUser | null;
  loading: boolean;
  sessionError: string | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  reload: () => Promise<void>;
  canWrite: boolean;
  canManageJetStream: boolean;
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
    return {
      user,
      loading,
      sessionError,
      login,
      logout,
      reload,
      canWrite: isRoot || hasRole(roles, "admin") || hasRole(roles, "operator"),
      canManageJetStream: isRoot || hasRole(roles, "admin"),
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
