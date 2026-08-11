"use client";

import { useCallback } from "react";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import type { ContentCardData } from "@/components/content/ContentCard";
import { useContentDetailOverlay } from "@/components/content/use-content-detail-overlay";

interface OverlayMasonryGridProps {
  items: ContentCardData[];
  emptyText?: string;
  className?: string;
  /** 浮窗返回文案语义：分区页（二创/原创）用 zone-page，IP 详情面用 ip-page。 */
  source: "zone-page" | "ip-page";
}

/**
 * 分区页/IP 页共用的内容卡片瀑布流：卡片主点击区改为打开共享
 * ContentDetailOverlay（复用推荐页接线模式），关闭后恢复触发卡片焦点与
 * 页面滚动位置；直接 URL 访问详情页的深链不受影响。
 */
export function OverlayMasonryGrid({ items, emptyText, className, source }: OverlayMasonryGridProps) {
  const { open: handleOpenDetail, overlayElement } = useContentDetailOverlay({ source });

  const openDetail = useCallback(
    (data: ContentCardData, trigger: HTMLElement) => {
      const index = items.findIndex((item) => item.id === data.id);
      handleOpenDetail(
        {
          contentId: data.id,
          zone: data.zone === "original" ? "original" : "fanwork",
          /* #89 连续浏览：把触发上下文列表 + 当前索引传给浮层（移动端上滑切篇）。 */
          contextList: items.map((item) => ({
            id: item.id,
            zone: item.zone === "original" ? "original" : "fanwork",
          })),
          contextIndex: index >= 0 ? index : undefined,
        },
        trigger,
      );
    },
    [handleOpenDetail, items],
  );

  return (
    <>
      <MasonryGrid items={items} emptyText={emptyText} className={className} onOpenDetail={openDetail} />
      {overlayElement}
    </>
  );
}
