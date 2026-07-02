"use client";

import { useTranslations } from "next-intl";
import { VersionHistory } from "@/components/content/VersionHistory";
import { CollectionPicker } from "@/components/content/CollectionPicker";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { useEffect, useState } from "react";

interface ContentDetailClientProps {
  contentId: number;
  contentTitle?: string;
  zone?: "original" | "fanwork";
  authorId?: number;
}

export function ContentDetailClient({ contentId, contentTitle, zone = "original", authorId }: ContentDetailClientProps) {
  const t = useTranslations();
  const { user } = useAuth();
  const [collectionPickerOpen, setCollectionPickerOpen] = useState(false);
  const isAuthor = user?.id === authorId;

  useEffect(() => {
    if (!user) return;
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

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Button size="sm" variant="outline" disabled={!user} onClick={() => setCollectionPickerOpen(true)}>
          {t("collections.picker.actions.open")}
        </Button>
        <CollectionPicker
          contentId={contentId}
          contentTitle={contentTitle ?? t("common.unknown")}
          zone={zone}
          open={collectionPickerOpen}
          onOpenChange={setCollectionPickerOpen}
        />
      </div>
      <VersionHistory contentId={contentId} isAuthor={isAuthor} />
    </div>
  );
}
