"use client";

import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  useRef,
  ReactNode,
} from "react";
import { api, ApiRequestError, setAccessToken, getAccessToken, refreshSession } from "@/lib/api";
import {
  saveTokens,
  clearTokens,
  isTokenExpired,
} from "@/lib/auth";
import { silentError } from "@/lib/error-handler";
import { mergeLocalIpsIntoAccount } from "@/lib/ip-visit-history";

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
  email_verified_at: string | null;
  accept_collab_invites: boolean;
  created_at: string;
}

export interface UnreadCounts {
  total: number;
  reply: number;
  like: number;
  system: number;
  pr: number;
  follow: number;
  // 广播未读（FIX-31b）：后端 unread-count 已按 channel 聚合返回，缺键会让
  // 下拉的广播角标永不显示。
  broadcast: number;
}

export interface InteractionCapabilities {
  can_interact: boolean;
  interaction_denial_reason?: string;
}

const DENIAL_REASON_USER_BANNED = "USER_BANNED";
const DENIAL_REASON_EMAIL_NOT_VERIFIED = "EMAIL_NOT_VERIFIED";
const DENIAL_REASON_INSUFFICIENT_REPUTATION = "INSUFFICIENT_REPUTATION";
const DENIAL_REASON_CONFIG_ERROR = "CONFIG_ERROR";
const DENIAL_REASON_UNAVAILABLE = "AUTH_STATUS_UNAVAILABLE";

const FAIL_CLOSED_CAPABILITIES: InteractionCapabilities = {
  can_interact: false,
  interaction_denial_reason: DENIAL_REASON_UNAVAILABLE,
};

export function readCapabilities(
  data: { capabilities?: InteractionCapabilities },
): InteractionCapabilities {
  if (!data.capabilities || typeof data.capabilities.can_interact !== "boolean") {
    return FAIL_CLOSED_CAPABILITIES;
  }
  return data.capabilities;
}

export function interactionDenialKey(reason?: string): string {
  switch (reason) {
    case DENIAL_REASON_USER_BANNED:
      return "capabilities.deniedBanned";
    case DENIAL_REASON_EMAIL_NOT_VERIFIED:
      return "capabilities.deniedEmailNotVerified";
    case DENIAL_REASON_INSUFFICIENT_REPUTATION:
      return "capabilities.deniedInsufficientReputation";
    case DENIAL_REASON_CONFIG_ERROR:
    case DENIAL_REASON_UNAVAILABLE:
      return "capabilities.deniedUnavailable";
    default:
      return "capabilities.deniedUnknown";
  }
}

interface AuthContextValue {
  user: User | null;
  isLoading: boolean;
  unreadCounts: UnreadCounts;
  capabilities: InteractionCapabilities;
  ipHistoryVersion: number;
  login: (email: string, password: string, rememberMe?: boolean) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<boolean>;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [capabilities, setCapabilities] = useState<InteractionCapabilities>(FAIL_CLOSED_CAPABILITIES);
  const [unreadCounts, setUnreadCounts] = useState<UnreadCounts>({ total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0, broadcast: 0 });
  const [ipHistoryVersion, setIPHistoryVersion] = useState(0);
  const previousUserRef = useRef<User | null>(null);
  // 会话代际：login/logout 都会推进。迟到的 refresh 失败回调只有在代际未变
  // （期间没有发生登录/登出）时才允许清空用户态，否则会把刚建立的会话抹掉。
  const authEpochRef = useRef(0);

  // #381：统一走 api.ts 的应用级单飞（refreshSession → doRefreshToken）。
  // 此前这里自建了一条 authRefreshInFlight 管线，与 api.ts 的 refreshPromise
  // 互不感知——同一标签页内两管线并发会对同一 refresh cookie 双发轮换请求，
  // 打进服务端轮换竞态窗口。失败时仅复位本地状态，跳转交给 (protected) 守卫。
  const refresh = useCallback(async (): Promise<boolean> => {
    const epoch = authEpochRef.current;
    const ok = await refreshSession();
    if (authEpochRef.current !== epoch) {
      // 期间发生了 login/logout：无论成败都不得回写，否则迟到的失败会抹掉
      // 刚登录的会话、迟到的成功会在登出后复活 token。
      return ok;
    }
    if (!ok) {
      silentError("refresh failed", { component: "AuthContext", action: "refresh" });
      clearTokens();
      setAccessToken(null);
      setUser(null);
      setCapabilities(FAIL_CLOSED_CAPABILITIES);
    } else {
      const token = getAccessToken();
      if (token) {
        saveTokens(token);
      }
    }
    return ok;
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
      const data = await api.get<{ user: User; capabilities?: InteractionCapabilities }>("/api/v1/auth/me");
      setUser(data.user);
      setCapabilities(readCapabilities(data));
    } catch (e) {
      silentError(e, { component: "AuthContext", action: "fetchMe" });
      clearTokens();
      setAccessToken(null);
      setUser(null);
      setCapabilities(FAIL_CLOSED_CAPABILITIES);
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
      setUnreadCounts({ total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0, broadcast: 0 });
      return;
    }
    let cancelled = false;
    async function pollUnread() {
      try {
        const data = await api.get<{ unread_counts: UnreadCounts }>("/api/v1/notifications/unread-count");
        if (cancelled) return;
        setUnreadCounts(data.unread_counts || { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0, broadcast: 0 });
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

  // Idempotent login merge: the first time the session turns authenticated
  // (including the initial page load of an already signed-in user), local
  // anonymous IP visits are merged into the account history. A failed merge
  // keeps the local records and only retries on a later auth transition,
  // so a 401 can never become a retry loop. The version bump re-reads any
  // recent-IP surface once the merge attempt settles.
  useEffect(() => {
    if (user && !previousUserRef.current) {
      mergeLocalIpsIntoAccount().finally(() => setIPHistoryVersion((v) => v + 1));
    }
    previousUserRef.current = user;
  }, [user]);

  const login = useCallback(async (email: string, password: string, _rememberMe?: boolean) => {
    const data = await api.post<{
      user: User;
      tokens: { access_token: string };
      capabilities?: InteractionCapabilities;
    }>("/api/v1/auth/login", { email, password });
    saveTokens(data.tokens.access_token);
    setAccessToken(data.tokens.access_token);
    authEpochRef.current += 1;
    setUser(data.user);
    setCapabilities(readCapabilities(data));
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.post("/api/v1/auth/logout", {});
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 401) {
        // already logged out
      }
    } finally {
      authEpochRef.current += 1;
      clearTokens();
      setAccessToken(null);
      setUser(null);
      setCapabilities(FAIL_CLOSED_CAPABILITIES);
    }
  }, []);

  const refreshUser = useCallback(async () => {
    try {
      const data = await api.get<{ user: User; capabilities?: InteractionCapabilities }>("/api/v1/auth/me");
      setUser(data.user);
      setCapabilities(readCapabilities(data));
    } catch (e) {
      silentError(e, { component: "AuthContext", action: "refreshUser" });
    }
  }, []);

  return (
    <AuthContext.Provider value={{ user, isLoading, unreadCounts, capabilities, ipHistoryVersion, login, logout, refresh, refreshUser }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
