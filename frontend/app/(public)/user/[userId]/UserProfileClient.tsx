"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { ContentCardData } from "@/components/content/ContentCard";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { Button } from "@/components/ui/button";
import { FollowButton } from "@/components/social/FollowButton";
import { normalizeContentList } from "@/lib/content";
import { useTranslations } from 'next-intl';

interface UserProfileClientProps {
  userId: number;
  displayName: string;
}

export function UserProfileClient({ userId, displayName }: UserProfileClientProps) {
  const t = useTranslations();
  const router = useRouter();
  const { user } = useAuth();
  const [tab, setTab] = useState<"contents" | "favorites" | "discussions">("contents");
  const [items, setItems] = useState<ContentCardData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const isOwnProfile = user?.id === userId;

  useEffect(() => {
    void loadTab();
  }, [userId, tab]);

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
          // GET /users/:id/discussions not yet implemented; fallback to user's content
          url = `/api/v1/users/${userId}/contents?page=1&page_size=24`;
          break;
      }
      interface ProfileResponse { contents?: unknown[]; favorites?: unknown[]; discussions?: unknown[] }
      const data = await api.get<ProfileResponse>(url);
      const list = data.contents ?? data.favorites ?? data.discussions ?? [];
      setItems(normalizeContentList(list));
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t('common.loadFailed'));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        {!isOwnProfile && (
          <FollowButton targetType="user" targetId={userId} />
        )}
        {isOwnProfile && (
          <Button size="sm" variant="outline" onClick={() => router.push("/settings")}>
            {t('user.editProfile')}
          </Button>
        )}
      </div>

      <div className="flex gap-1 border-b border-border">
        {(["contents", "favorites", "discussions"] as const).map((tKey) => (
          <button
            key={tKey}
            onClick={() => setTab(tKey)}
            className={`px-4 py-2 text-sm border-b-2 transition-colors ${
              tab === tKey
                ? "border-foreground text-foreground font-medium"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            {tKey === "contents" ? t('user.tabPublish') : tKey === "favorites" ? t('user.tabFavorites') : t('user.tabDiscussions')}
          </button>
        ))}
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading ? (
        <div className="rounded-md border border-border bg-card p-8 text-center text-sm text-muted-foreground">{t('common.loading')}</div>
      ) : items.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-8 text-center text-sm text-muted-foreground">
          {tab === "contents" ? (isOwnProfile ? t('user.noContentOwn') : t('user.noContent', { name: displayName })) : t('user.noContentGeneric')}
        </div>
      ) : (
        <MasonryGrid items={items} />
      )}
    </div>
  );
}
