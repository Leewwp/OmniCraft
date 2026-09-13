"use client";

import { useCallback } from "react";
import { ContentDetail } from "@/components/content/ContentDetail";
import { ContentSidebar, type RelatedContentEntry } from "@/components/content/ContentSidebar";
import { FollowButton } from "@/components/social/FollowButton";
import { useContentDetailOverlay } from "@/components/content/use-content-detail-overlay";
import { VersionHistory } from "@/components/content/VersionHistory";
import type { SourceSummary } from "@/components/content/SourceAttribution";
import type { ContentCardData } from "@/components/content/ContentCard";
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
  /** SP-17/T4：透传详情 author 的头像与登录视角关注态（侧栏真按钮 + 悬浮卡触发器）。 */
  author?: { id?: number; username?: string; avatar_url?: string; is_following?: boolean };
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

  /* #90 相关内容块卡片：浮层栈内打开（source=zone-page）。 */
  const openRelatedDetail = useCallback(
    (data: ContentCardData, trigger: HTMLElement) => {
      handleOpenRelated(
        { contentId: data.id, zone: data.zone === "original" ? "original" : "fanwork" },
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
          onOpenRelatedDetail={openRelatedDetail}
        />
        {zone === "fanwork" && <VersionHistory contentId={content.id} />}
      </div>

      {/* SP-17/T4：侧栏创作者卡接真 FollowButton（原静态兜底），初始态来自
          详情 author.is_following；authorStats 死 prop 按 #493 取简裁决移除。 */}
      <ContentSidebar
        author={author}
        zone={zone}
        ip={ip}
        sourceOriginal={sourceOriginal}
        onOpenRelated={openRelated}
        followAction={
          author?.id ? (
            <FollowButton
              targetType="user"
              targetId={author.id}
              initialFollowing={author.is_following ?? false}
            />
          ) : undefined
        }
      />

      {overlayElement}
    </div>
  );
}
