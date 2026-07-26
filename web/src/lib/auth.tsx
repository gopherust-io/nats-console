import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { api, AccessRules, clearAuth, setAuth } from "./api";

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
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(async () => {
    let authEnabled = true;
    try {
      const config = await api<{
        authEnabled: boolean;
      }>("/api/v1/auth/config");
      authEnabled = config.authEnabled;
      if (!authEnabled) {
        setUser({ username: "dev", roles: ["admin"], isRoot: true });
        return;
      }
    } catch {
      // fall through to /me
    }

    try {
      const me = await api<AuthUser>("/api/v1/auth/me");
      setUser(me);
    } catch {
      setUser(null);
    }
  }, []);

  useEffect(() => {
    reload().finally(() => setLoading(false));
  }, [reload]);

  const login = useCallback(async (username: string, password: string) => {
    setAuth(username, password);
    try {
      const me = await api<AuthUser>("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      });
      setUser(me);
    } catch (err) {
      clearAuth();
      throw err;
    }
  }, []);

  const logout = useCallback(async () => {
    try {
      await api("/api/v1/auth/logout", { method: "POST" });
    } catch {
      // ignore
    }
    clearAuth();
    setUser(null);
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
  }, [user, loading, login, logout, reload]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return ctx;
}
