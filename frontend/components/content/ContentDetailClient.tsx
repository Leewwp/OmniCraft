"use client";

import { useTranslations } from "next-intl";
import { VersionHistory } from "@/components/content/VersionHistory";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { useEffect, useState } from "react";

interface ContentDetailClientProps {
  contentId: number;
  authorId?: number;
}

export function ContentDetailClient({ contentId, authorId }: ContentDetailClientProps) {
  const t = useTranslations();
  const { user } = useAuth();
  const [favorited, setFavorited] = useState(false);
  const [favBusy, setFavBusy] = useState(false);
  const isAuthor = user?.id === authorId;

  useEffect(() => {
    if (!user) return;
    void checkFavorite();
    recordBrowseHistory();
  }, [user, contentId]);

  function recordBrowseHistory() {
    const key = `browse_history_${contentId}`;
    const lastRecorded = localStorage.getItem(key);
    const now = Date.now();
    if (lastRecorded && now - parseInt(lastRecorded, 10) < 5 * 60 * 1000) return;
    localStorage.setItem(key, String(now));
    api.post(`/api/v1/users/me/history`, { content_item_id: contentId }).catch(() => {});
  }

  async function checkFavorite() {
    try {
      const data = await api.get<{ favorites?: { content_item_id: number }[] }>(
        `/api/v1/users/${user!.id}/favorites`
      );
      const favs = data.favorites || [];
      setFavorited(favs.some((f) => f.content_item_id === contentId));
    } catch { /* ignore */ }
  }

  async function toggleFavorite() {
    if (!user) return;
    setFavBusy(true);
    try {
      if (favorited) {
        await api.delete(`/api/v1/favorites/${contentId}`);
        setFavorited(false);
      } else {
        await api.post("/api/v1/favorites", { content_item_id: contentId });
        setFavorited(true);
      }
    } catch { /* ignore */ }
    finally { setFavBusy(false); }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Button size="sm" variant={favorited ? "default" : "outline"} disabled={favBusy} onClick={() => void toggleFavorite()}>
          {favorited ? t('content.favorited') : t('content.favorite')}
        </Button>
      </div>
      <VersionHistory contentId={contentId} isAuthor={isAuthor} />
    </div>
  );
}
