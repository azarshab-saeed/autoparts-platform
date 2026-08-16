"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { getMe, MOCK_MODE } from "@/lib/api";
import { getKeycloak, initKeycloak } from "@/lib/keycloak";
import type { UserSession } from "@/lib/types";

type AuthContextValue = {
  ready: boolean;
  authenticated: boolean;
  session: UserSession | null;
  login: (redirectPath?: string) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

const storeMockSession: UserSession = {
  token: "mock-token",
  displayName: "رضا رضایی",
  email: "owner@example.com",
  role: "owner",
  roles: ["owner"],
  storeName: "یدکی رضایی",
  storeId: "22222222-2222-2222-2222-222222222222",
  warehouseId: "33333333-3333-3333-3333-333333333333",
};
const mechanicMockSession: UserSession = {
  token: "mock-mechanic-token",
  displayName: "مهدی مکانیک",
  email: "mechanic@example.com",
  role: "mechanic",
  roles: ["mechanic"],
  storeName: "",
  storeId: "",
  warehouseId: "",
};

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [ready, setReady] = useState(false);
  const [session, setSession] = useState<UserSession | null>(null);

  const syncFromKeycloak = useCallback(async () => {
    const kc = getKeycloak();
    if (!kc.authenticated) {
      setSession(null);
      return;
    }
    await kc.updateToken(30);
    if (!kc.token) throw new Error("Keycloak access token is missing");
    const me = await getMe(kc.token);
    setSession({
      token: kc.token,
      displayName: me.display_name || me.email || "کاربر",
      email: me.email || "",
      role: me.role,
      roles: me.roles || [me.role],
      storeName: me.store_name || "",
      storeId: me.store_id || "",
      warehouseId: me.default_warehouse_id || "",
    });
  }, []);

  useEffect(() => {
    let active = true;
    let timer: ReturnType<typeof setInterval> | undefined;

    async function start() {
      try {
        if (MOCK_MODE) {
          if (active) setSession(window.location.pathname.startsWith("/mechanic") ? mechanicMockSession : storeMockSession);
          return;
        }
        const authenticated = await initKeycloak();
        if (!active) return;
        if (authenticated) await syncFromKeycloak();

        const kc = getKeycloak();
        kc.onAuthLogout = () => setSession(null);
        kc.onTokenExpired = () => {
          void syncFromKeycloak().catch(() => setSession(null));
        };
        timer = setInterval(() => {
          if (kc.authenticated) {
            void syncFromKeycloak().catch(() => setSession(null));
          }
        }, 20_000);
      } finally {
        if (active) setReady(true);
      }
    }

    void start();
    return () => {
      active = false;
      if (timer) clearInterval(timer);
    };
  }, [syncFromKeycloak]);

  const login = useCallback(async (redirectPath = "/store") => {
    if (MOCK_MODE) {
      const next = redirectPath.startsWith("/mechanic") ? mechanicMockSession : storeMockSession;
      setSession(next);
      window.location.href = redirectPath;
      return;
    }
    const kc = getKeycloak();
    await kc.login({ redirectUri: `${window.location.origin}${redirectPath}` });
  }, []);

  const logout = useCallback(async () => {
    if (MOCK_MODE) {
      setSession(null);
      window.location.href = "/login";
      return;
    }
    const kc = getKeycloak();
    setSession(null);
    await kc.logout({ redirectUri: `${window.location.origin}/login` });
  }, []);

  const value = useMemo<AuthContextValue>(() => ({
    ready,
    authenticated: Boolean(session),
    session,
    login,
    logout,
  }), [ready, session, login, logout]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used inside AuthProvider");
  return value;
}
