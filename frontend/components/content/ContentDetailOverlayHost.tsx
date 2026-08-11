"use client";

import { useCallback } from "react";
import { ContentDetail } from "@/components/content/ContentDetail";
import { ContentSidebar, type RelatedContentEntry } from "@/components/content/ContentSidebar";
import { useContentDetailOverlay } from "@/components/content/use-content-detail-overlay";
import { VersionHistory } from "@/components/content/VersionHistory";
import type { SourceSummary } from "@/components/content/SourceAttribution";
import type { AttachmentData, ContentDetailData } from "@/lib/content";

interface RelatedFanworksSlot {
  sourceContentId: number;
  sourceZone: "original" | "fanwork";
  titleKey: string;
  createHref?: string;
  viewAllHref?: string;
}

interface ContentDetailOverlayHostProps {
  content: ContentDetailData & { attachments: AttachmentData[]; tags: string[] };
  zone: "original" | "fanwork";
  author?: { id?: number; username?: string };
  ip?: { id?: number; name?: string; slug?: string };
  sourceOriginal?: { id: number; title: string } | null;
  sourceFanwork?: SourceSummary | null;
  relatedFanworks?: RelatedFanworksSlot;
}

/** 详情页宿主：详情主体 + 侧栏 + 共享内容详情浮层（关联内容入口打开浮窗，下钻不跳页）。 */
export function ContentDetailOverlayHost({
  content,
  zone,
  author,
  ip,
  sourceOriginal,
  sourceFanwork,
  relatedFanworks,
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
          sourceOriginal={
            sourceOriginal && sourceOriginal.title
              ? { id: sourceOriginal.id, title: sourceOriginal.title, zone: "original" }
              : undefined
          }
          sourceFanwork={sourceFanwork ?? undefined}
          relatedFanworks={relatedFanworks}
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
