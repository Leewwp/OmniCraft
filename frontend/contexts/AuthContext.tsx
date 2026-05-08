"use client";

import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  ReactNode,
} from "react";
import { api, ApiRequestError } from "@/lib/api";
import {
  saveTokens,
  clearTokens,
  getAccessToken,
  getRefreshToken,
  isTokenExpired,
} from "@/lib/auth";

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

interface AuthContextValue {
  user: User | null;
  isLoading: boolean;
  login: (email: string, password: string, rememberMe?: boolean) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<boolean>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const refresh = useCallback(async (): Promise<boolean> => {
    const refreshToken = getRefreshToken();
    if (!refreshToken) return false;
    try {
      const data = await api.post<{ tokens: { access_token: string; refresh_token: string } }>(
        "/api/v1/auth/refresh",
        { refresh_token: refreshToken }
      );
      saveTokens(data.tokens.access_token, data.tokens.refresh_token);
      return true;
    } catch {
      clearTokens();
      setUser(null);
      return false;
    }
  }, []);

  const fetchMe = useCallback(async () => {
    try {
      const accessToken = getAccessToken();
      if (!accessToken) {
        setIsLoading(false);
        return;
      }
      if (isTokenExpired(accessToken)) {
        const ok = await refresh();
        if (!ok) {
          setIsLoading(false);
          return;
        }
      }
      const data = await api.get<{ user: User }>("/api/v1/auth/me");
      setUser(data.user);
    } catch {
      clearTokens();
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  }, [refresh]);

  useEffect(() => {
    fetchMe();
  }, [fetchMe]);

  const login = useCallback(async (email: string, password: string, rememberMe?: boolean) => {
    const data = await api.post<{
      user: User;
      tokens: { access_token: string; refresh_token: string };
    }>("/api/v1/auth/login", { email, password });
    saveTokens(data.tokens.access_token, data.tokens.refresh_token, rememberMe);
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
      setUser(null);
    }
  }, []);

  return (
    <AuthContext.Provider value={{ user, isLoading, login, logout, refresh }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
