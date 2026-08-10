"use client";

import { useCallback } from "react";
import { ContentDetail } from "@/components/content/ContentDetail";
import { ContentSidebar, type RelatedContentEntry } from "@/components/content/ContentSidebar";
import { useContentDetailOverlay } from "@/components/content/use-content-detail-overlay";
import { VersionHistory } from "@/components/content/VersionHistory";
import type { AttachmentData, ContentDetailData } from "@/lib/content";

interface ContentDetailOverlayHostProps {
  content: ContentDetailData & { attachments: AttachmentData[]; tags: string[] };
  zone: "original" | "fanwork";
  author?: { id?: number; username?: string };
  ip?: { id?: number; name?: string; slug?: string };
  sourceOriginal?: { id: number; title: string } | null;
}

/** 详情页宿主：详情主体 + 侧栏 + 共享内容详情浮层（关联内容入口打开浮窗，下钻不跳页）。 */
export function ContentDetailOverlayHost({
  content,
  zone,
  author,
  ip,
  sourceOriginal,
}: ContentDetailOverlayHostProps) {
  const { open: handleOpenRelated, overlayElement } = useContentDetailOverlay({
    source: "zone-page",
  });

  const openRelated = useCallback(
    (relatedEntry: RelatedContentEntry, trigger: HTMLElement) => {
      handleOpenRelated(
        { contentId: relatedEntry.id, zone: relatedEntry.zone },
        trigger,
      );
    },
    [handleOpenRelated],
  );

  return (
    <div className="mx-auto flex w-full max-w-[1280px] gap-6 px-6 py-6">
      <div className="min-w-0 flex-1">
        <ContentDetail
          data={{ ...content, attachments: content.attachments, tags: content.tags }}
        />
        {zone === "fanwork" && <VersionHistory contentId={content.id} />}
      </div>

      <ContentSidebar
        author={author}
        authorStats={undefined}
        zone={zone}
        ip={ip}
        sourceOriginal={sourceOriginal}
        onOpenRelated={openRelated}
      />

      {overlayElement}
    </div>
  );
}
