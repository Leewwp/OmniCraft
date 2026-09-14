"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { ContentCardData } from "@/components/content/ContentCard";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { Button } from "@/components/ui/button";
import { FollowButton } from "@/components/social/FollowButton";
import { MessageComposeButton } from "@/components/social/MessageComposeButton";
import { normalizeContentList } from "@/lib/content";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { useTranslations } from 'next-intl';
import { silentError } from "@/lib/error-handler";
import { UserCollectionsPanel } from "./UserCollectionsPanel";
import type { ProfileTab } from "./profile-tab";

interface UserProfileClientProps {
  userId: number;
  displayName: string;
  initialTab?: ProfileTab;
}

interface DiscussionCardRecord {
  id?: number;
  title?: string;
  author_id?: number;
  author?: { id?: number; username?: string };
  view_count?: number;
}

function toDiscussionCardData(value: unknown[]): ContentCardData[] {
  return value
    .map((item): ContentCardData | null => {
      const raw = item as DiscussionCardRecord;
      if (typeof raw?.id !== "number" || typeof raw.title !== "string") return null;
      return {
        id: raw.id,
        title: raw.title,
        author_id: raw.author_id,
        author: raw.author,
        view_count: raw.view_count,
      };
    })
    .filter((item): item is ContentCardData => item !== null);
}

export function UserProfileClient({ userId, displayName, initialTab = "contents" }: UserProfileClientProps) {
  const t = useTranslations();
  const router = useRouter();
  const { user } = useAuth();
  const [tab, setTab] = useState<ProfileTab>(initialTab);
  const [items, setItems] = useState<ContentCardData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const isOwnProfile = user?.id === userId;

  useEffect(() => {
    if (tab === "collections") return;
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
        case "discussions":
          url = `/api/v1/users/${userId}/discussions?page=1&page_size=24`;
          break;
      }
      interface ProfileResponse { contents?: unknown[]; discussions?: unknown[] }
      const data = await api.get<ProfileResponse>(url);
      const raw = data.contents ?? data.discussions ?? [];
      setItems(tab === "discussions" ? toDiscussionCardData(raw) : normalizeContentList(raw));
    } catch (e) {
      silentError(e, { component: 'UserProfileClient', action: 'loadTab' });
      setError(t(getUserFacingErrorKey(e, "common.loadFailed")));
    } finally {
      setLoading(false);
    }
  }

  /* 页内切换三个 tab（#508 收藏集不再外跳）：原生 replaceState 静默同步 ?tab=
     供分享/深链（收藏集旧路由 301 到此），不触发服务端往返。 */
  function switchTab(next: ProfileTab) {
    setTab(next);
    window.history.replaceState(null, "", `/user/${userId}?tab=${next}`);
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        {!isOwnProfile && (
          <>
            <FollowButton targetType="user" targetId={userId} />
            {/* FIX-30①/T35：任意用户可发起私信（纯前端入口，后端 guard 不动）。 */}
            <MessageComposeButton userId={userId} displayName={displayName} />
          </>
        )}
        {isOwnProfile && (
          <Button size="sm" variant="outline" onClick={() => router.push("/settings")}>
            {t('user.editProfile')}
          </Button>
        )}
      </div>

      <div className="flex gap-1 border-b border-border" role="tablist">
        {(["contents", "collections", "discussions"] as const).map((tKey) => (
          <button
            key={tKey}
            type="button"
            role="tab"
            aria-selected={tab === tKey}
            onClick={() => switchTab(tKey)}
            className={`border-b-2 px-4 py-2 text-sm transition-colors ${
              tab === tKey
                ? "border-foreground text-foreground font-medium"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            {tKey === "contents"
              ? t('user.tabPublish')
              : tKey === "collections"
                ? t('user.tabCollections')
                : t('user.tabDiscussions')}
          </button>
        ))}
      </div>

      {tab === "collections" ? (
        <UserCollectionsPanel ownerId={userId} />
      ) : (
        <>
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
        </>
      )}
    </div>
  );
}
