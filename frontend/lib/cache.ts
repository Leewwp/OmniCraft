import useSWR from "swr";
import { api } from "./api";

// ── Fetcher ──────────────────────────────────────────────

const jsonFetcher = async <T>(url: string): Promise<T> => api.get<T>(url);

// ── Auth ─────────────────────────────────────────────────

interface UserInfo {
  user: {
    id: number;
    email: string;
    username: string;
    avatar_url: string;
    bio: string;
    reputation: number;
    role: string;
    is_banned: boolean;
    created_at: string;
  };
}

export function useCurrentUser() {
  return useSWR<UserInfo>("/api/v1/auth/me", jsonFetcher, {
    revalidateOnFocus: false,
    dedupingInterval: 60000,
  });
}

// ── Notifications ────────────────────────────────────────

interface UnreadCounts {
  unread_counts: {
    total: number;
    reply: number;
    like: number;
    system: number;
    pr: number;
    follow: number;
  };
}

export function useUnreadCount() {
  return useSWR<UnreadCounts>("/api/v1/notifications/unread-count", jsonFetcher, {
    refreshInterval: 30000,
    dedupingInterval: 10000,
  });
}

// ── Stats ─────────────────────────────────────────────────

interface StatsSummary {
  summary: {
    users: number;
    ips: number;
    contents: number;
  };
}

export function useStatsSummary() {
  return useSWR<StatsSummary>("/api/v1/stats/summary", jsonFetcher, {
    revalidateOnFocus: false,
    dedupingInterval: 300000,
  });
}

// ── Hot Contents ──────────────────────────────────────────

interface ContentItem {
  id: number;
  title: string;
  cover_image_url?: string;
  zone?: string;
  content_type?: string;
  view_count?: number;
  like_count?: number;
  created_at?: string;
  author?: { id: number; username: string; avatar_url?: string };
}

export function useHotContents(zone = "fanwork", limit = 24) {
  return useSWR<{ contents: ContentItem[] }>(
    `/api/v1/contents?zone=${zone}&sort=hot&page_size=${limit}`,
    jsonFetcher,
    {
      revalidateOnFocus: false,
      dedupingInterval: 120000,
    }
  );
}

// ── IP detail ─────────────────────────────────────────────

export function useIPDetail(ipId: number | null) {
  return useSWR(
    ipId ? `/api/v1/ips/${ipId}` : null,
    jsonFetcher,
    { revalidateOnFocus: false, dedupingInterval: 60000 }
  );
}

// ── Cache helpers ─────────────────────────────────────────

export { useSWR };
