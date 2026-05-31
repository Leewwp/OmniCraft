"use client";

import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  ReactNode,
} from "react";
import { api, ApiRequestError, setAccessToken, getAccessToken } from "@/lib/api";
import {
  saveTokens,
  clearTokens,
  isTokenExpired,
} from "@/lib/auth";
import { silentError } from "@/lib/error-handler";

export interface User {
  id: number;
  email: string;
  username: string;
  avatar_url: string;
  bio: string;
  reputation: number;
  preferred_locale: string;
  role: string;
  is_banned: boolean;
  created_at: string;
}

export interface UnreadCounts {
  total: number;
  reply: number;
  like: number;
  system: number;
  pr: number;
  follow: number;
}

interface AuthContextValue {
  user: User | null;
  isLoading: boolean;
  unreadCounts: UnreadCounts;
  login: (email: string, password: string, rememberMe?: boolean) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<boolean>;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [unreadCounts, setUnreadCounts] = useState<UnreadCounts>({ total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 });

  const refresh = useCallback(async (): Promise<boolean> => {
    try {
      const data = await api.post<{ tokens: { access_token: string } }>(
        "/api/v1/auth/refresh",
        {}
      );
      saveTokens(data.tokens.access_token);
      setAccessToken(data.tokens.access_token);
      return true;
    } catch (e) {
      silentError(e, { component: "AuthContext", action: "refresh" });
      clearTokens();
      setAccessToken(null);
      setUser(null);
      return false;
    }
  }, []);

  const fetchMe = useCallback(async () => {
    try {
      const accessToken = getAccessToken();
      if (!accessToken) {
        const ok = await refresh();
        if (!ok) {
          setIsLoading(false);
          return;
        }
      } else if (isTokenExpired(accessToken)) {
        const ok = await refresh();
        if (!ok) {
          setIsLoading(false);
          return;
        }
      }
      const data = await api.get<{ user: User }>("/api/v1/auth/me");
      setUser(data.user);
    } catch (e) {
      silentError(e, { component: "AuthContext", action: "fetchMe" });
      clearTokens();
      setAccessToken(null);
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  }, [refresh]);

  useEffect(() => {
    fetchMe();
    const interval = setInterval(() => {
      fetchMe();
    }, 5 * 60 * 1000);
    return () => clearInterval(interval);
  }, [fetchMe]);

  useEffect(() => {
    if (!user) {
      setUnreadCounts({ total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 });
      return;
    }
    let cancelled = false;
    async function pollUnread() {
      try {
        const data = await api.get<{ unread_counts: UnreadCounts }>("/api/v1/notifications/unread-count");
        if (cancelled) return;
        setUnreadCounts(data.unread_counts || { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 });
      } catch (e) {
        silentError(e, { component: "AuthContext", action: "pollUnread" });
      }
    }
    pollUnread();
    const interval = setInterval(pollUnread, 30000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [user]);

  const login = useCallback(async (email: string, password: string, _rememberMe?: boolean) => {
    const data = await api.post<{
      user: User;
      tokens: { access_token: string };
    }>("/api/v1/auth/login", { email, password });
    saveTokens(data.tokens.access_token);
    setAccessToken(data.tokens.access_token);
    setUser(data.user);
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.post("/api/v1/auth/logout", {});
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 401) {
        // already logged out
      }
    } finally {
      clearTokens();
      setAccessToken(null);
      setUser(null);
    }
  }, []);

  const refreshUser = useCallback(async () => {
    try {
      const data = await api.get<{ user: User }>("/api/v1/auth/me");
      setUser(data.user);
    } catch (e) {
      silentError(e, { component: "AuthContext", action: "refreshUser" });
    }
  }, []);

  return (
    <AuthContext.Provider value={{ user, isLoading, unreadCounts, login, logout, refresh, refreshUser }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
