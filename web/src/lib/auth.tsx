import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { api, clearAuth, getAuthHeader, setAuth } from "./api";

export type AuthUser = {
  id?: string;
  username: string;
  email?: string;
  roles: string[];
};

type AuthContextValue = {
  user: AuthUser | null;
  loading: boolean;
  oidcEnabled: boolean;
  basicEnabled: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  reload: () => Promise<void>;
  canWrite: boolean;
  isAdmin: boolean;
  isViewer: boolean;
};

const AuthContext = createContext<AuthContextValue | null>(null);

function hasRole(roles: string[], role: string) {
  return roles.includes(role);
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [oidcEnabled, setOidcEnabled] = useState(false);
  const [basicEnabled, setBasicEnabled] = useState(true);

  const reload = useCallback(async () => {
    let authEnabled = true;
    try {
      const config = await api<{ oidc_enabled: boolean; basic_enabled: boolean; auth_enabled: boolean }>(
        "/api/v1/auth/config",
      );
      setOidcEnabled(config.oidc_enabled);
      setBasicEnabled(config.basic_enabled);
      authEnabled = config.auth_enabled;
      if (!authEnabled) {
        setUser({ username: "dev", roles: ["admin"] });
        return;
      }
    } catch {
      setOidcEnabled(false);
      setBasicEnabled(true);
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

  const roles = user?.roles ?? [];
  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      loading,
      oidcEnabled,
      basicEnabled,
      login,
      logout,
      reload,
      canWrite: hasRole(roles, "admin") || hasRole(roles, "operator"),
      isAdmin: hasRole(roles, "admin"),
      isViewer: roles.length > 0 && !hasRole(roles, "admin") && !hasRole(roles, "operator"),
    }),
    [user, loading, oidcEnabled, basicEnabled, login, logout, reload, roles],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return ctx;
}
