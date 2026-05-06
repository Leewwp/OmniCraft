"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { ContentCardData } from "@/components/content/ContentCard";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { Button } from "@/components/ui/button";
import { normalizeContentList } from "@/lib/content";

interface UserProfileClientProps {
  userId: number;
  displayName: string;
}

export function UserProfileClient({ userId, displayName }: UserProfileClientProps) {
  const { user } = useAuth();
  const [tab, setTab] = useState<"contents" | "favorites" | "discussions">("contents");
  const [items, setItems] = useState<ContentCardData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [following, setFollowing] = useState(false);
  const [followBusy, setFollowBusy] = useState(false);

  const isOwnProfile = user?.id === userId;

  useEffect(() => {
    void loadTab();
  }, [userId, tab]);

  useEffect(() => {
    if (!user || isOwnProfile) return;
    void checkFollow();
  }, [user, userId, isOwnProfile]);

  async function checkFollow() {
    try {
      const data = await api.get<{ following?: { target_id: number }[] }>(`/api/v1/users/${user!.id}/following`);
      const list = data.following || [];
      setFollowing(list.some((f) => f.target_id === userId));
    } catch { /* ignore */ }
  }

  async function toggleFollow() {
    if (!user) return;
    setFollowBusy(true);
    try {
      if (following) {
        await api.delete(`/api/v1/users/${userId}/follow`);
        setFollowing(false);
      } else {
        await api.post(`/api/v1/users/${userId}/follow`, {});
        setFollowing(true);
      }
    } catch { /* ignore */ }
    finally { setFollowBusy(false); }
  }

  async function loadTab() {
    setError("");
    setLoading(true);
    try {
      let url = "";
      switch (tab) {
        case "contents":
          url = `/api/v1/users/${userId}/contents?page=1&page_size=24`;
          break;
        case "favorites":
          url = `/api/v1/users/${userId}/favorites`;
          break;
        case "discussions":
          url = `/api/v1/ips/1/discussions`; // fallback
          break;
      }
      const data = await api.get<{ contents?: unknown[]; favorites?: unknown[] }>(url);
      const list = (data as any).contents || (data as any).favorites || [];
      setItems(normalizeContentList(list));
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        {!isOwnProfile && (
          <Button size="sm" variant={following ? "default" : "outline"} disabled={followBusy} onClick={() => void toggleFollow()}>
            {following ? "已关注" : "关注"}
          </Button>
        )}
        {isOwnProfile && (
          <Button size="sm" variant="outline" onClick={() => window.location.href = "/settings"}>
            编辑资料
          </Button>
        )}
      </div>

      <div className="flex gap-1 border-b border-border">
        {(["contents", "favorites", "discussions"] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm border-b-2 transition-colors ${
              tab === t
                ? "border-foreground text-foreground font-medium"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            {t === "contents" ? "发布" : t === "favorites" ? "收藏" : "讨论"}
          </button>
        ))}
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading ? (
        <div className="rounded-md border border-border bg-card p-8 text-center text-sm text-muted-foreground">加载中...</div>
      ) : items.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-8 text-center text-sm text-muted-foreground">
          {tab === "contents" ? `${isOwnProfile ? "你" : displayName}还没有发布内容` : "暂无内容"}
        </div>
      ) : (
        <MasonryGrid items={items} />
      )}
    </div>
  );
}
