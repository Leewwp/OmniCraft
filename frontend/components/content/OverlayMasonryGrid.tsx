"use client";

import { useCallback, useRef, useState } from "react";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import type { ContentCardData } from "@/components/content/ContentCard";
import { ContentDetailOverlay } from "@/components/content/ContentDetailOverlay";

interface OverlayEntry {
  contentId: number;
  zone: "original" | "fanwork";
}

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
  const [overlayEntry, setOverlayEntry] = useState<OverlayEntry | null>(null);
  const overlayTriggerRef = useRef<HTMLElement | null>(null);

  const handleOpenDetail = useCallback((data: ContentCardData, trigger: HTMLElement) => {
    overlayTriggerRef.current = trigger;
    setOverlayEntry({
      contentId: data.id,
      zone: data.zone === "original" ? "original" : "fanwork",
    });
  }, []);

  return (
    <>
      <MasonryGrid items={items} emptyText={emptyText} className={className} onOpenDetail={handleOpenDetail} />

      {overlayEntry && (
        <ContentDetailOverlay
          key={`${overlayEntry.zone}:${overlayEntry.contentId}`}
          contentId={overlayEntry.contentId}
          zone={overlayEntry.zone}
          source={source}
          open
          onOpenChange={(open) => {
            if (!open) setOverlayEntry(null);
          }}
          returnFocusRef={overlayTriggerRef}
        />
      )}
    </>
  );
}
