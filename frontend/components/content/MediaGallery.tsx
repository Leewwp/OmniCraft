"use client";

import { useCallback, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { ChevronLeft, ChevronRight, ImageOff } from "lucide-react";
import { cn } from "@/lib/utils";
import type { AttachmentData } from "@/lib/content";

export interface MediaGalleryItem {
  id: number;
  url: string;
  type: "image" | "video";
  width?: number;
  height?: number;
  posterUrl?: string;
}

interface MediaGalleryProps {
  className?: string;
  items: MediaGalleryItem[];
  initialIndex?: number;
  /** 点击媒体进入 MediaViewer（#86 接入前不传，媒体区不可点）。 */
  onOpenViewer?: (index: number) => void;
  /** 移动端连续浏览：#89 媒体集最后一项继续上滑时触发。 */
  onReachEnd?: () => void;
  /** 浮层封面同步（#64 决策 11）：首项媒体加载落定/失败时回调一次，驱动主体 reveal。 */
  onFirstMediaSettled?: (state: "ready" | "error") => void;
}

/** 防御性默认比例（AC4）：宽高缺失/历史数据时使用 3:4，不报错不隐藏。 */
const DEFAULT_ASPECT_RATIO = 3 / 4;
/** 超高图阈值：height / width > 2 时限高 + 内部滚动。 */
const ULTRA_TALL_RATIO = 2;
/** 超高图容器高度上限（视口可用高的一部分，不撑破详情布局）。 */
const ULTRA_TALL_MAX_HEIGHT = "70vh";
/** 滑动翻页/连续浏览位移阈值（px）。 */
const SWIPE_THRESHOLD = 40;
/** 视频 controls 条近似高度：点击该区域内不进入查看器。 */
const VIDEO_CONTROLS_STRIP = 44;

function clampIndex(value: number, length: number): number {
  if (!Number.isFinite(value) || length <= 0) return 0;
  return Math.min(Math.max(Math.floor(value), 0), length - 1);
}

function itemAspectRatio(item: MediaGalleryItem): number {
  if (item.width && item.height && item.width > 0 && item.height > 0) {
    return item.width / item.height;
  }
  return DEFAULT_ASPECT_RATIO;
}

function isUltraTall(item: MediaGalleryItem): boolean {
  if (!item.width || !item.height || item.width <= 0 || item.height <= 0) return false;
  return item.height / item.width > ULTRA_TALL_RATIO;
}

/**
 * 媒体集 vs 附件语义拆分（AC3）：image/video 内容中 file_type 为 image/video
 * 且可解析 URL 的附件进入媒体集（按后端稳定顺序，不重排），其余维持下载列表；
 * 其他内容类型（article/sheet_music/mod/audio/template/prompt）全部维持下载列表。
 */
export function selectMediaItems(
  attachments: AttachmentData[],
  contentType: string | undefined,
  videoPosterUrl?: string,
): { media: MediaGalleryItem[]; downloads: AttachmentData[] } {
  if (contentType !== "image" && contentType !== "video") {
    return { media: [], downloads: attachments };
  }
  const media: MediaGalleryItem[] = [];
  const downloads: AttachmentData[] = [];
  for (const attachment of attachments) {
    const type =
      attachment.file_type === "image"
        ? "image"
        : attachment.file_type === "video"
          ? "video"
          : null;
    const url = attachment.oss_url || attachment.oss_key;
    if (type && url) {
      media.push({
        id: attachment.id,
        url,
        type,
        width: attachment.width,
        height: attachment.height,
        posterUrl: type === "video" ? videoPosterUrl : undefined,
      });
    } else {
      downloads.push(attachment);
    }
  }
  return { media, downloads };
}

/** 详情/浮层媒体区（#85）：contain 不裁切、几何由首项决定且会话内稳定、指示点 + 滑动/按钮翻页、超高图限高内部滚动。 */
export function MediaGallery({
  className,
  items,
  initialIndex,
  onOpenViewer,
  onReachEnd,
  onFirstMediaSettled,
}: MediaGalleryProps) {
  const t = useTranslations();
  const [index, setIndex] = useState(() => clampIndex(initialIndex ?? 0, items.length));
  const [failed, setFailed] = useState<Record<number, boolean>>({});
  const [loaded, setLoaded] = useState<Record<number, boolean>>({});
  const touchStartRef = useRef<{ x: number; y: number } | null>(null);
  const firstSettledRef = useRef(false);

  const goPrevious = useCallback(() => {
    setIndex((current) => Math.max(0, current - 1));
  }, []);

  const goNext = useCallback(() => {
    setIndex((current) => Math.min(items.length - 1, current + 1));
  }, [items.length]);

  if (items.length === 0) return null;

  const current = items[clampIndex(index, items.length)];
  const first = items[0];
  const ultraTallContainer = isUltraTall(first);
  const showControls = items.length > 1;
  const positionLabel = t("media.gallery.position", {
    current: clampIndex(index, items.length) + 1,
    total: items.length,
  });

  function handleMediaClick(event: React.MouseEvent<HTMLDivElement>) {
    if (!onOpenViewer) return;
    if (current.type === "video") {
      const rect = event.currentTarget.getBoundingClientRect();
      if (rect.height > 0 && event.clientY > rect.bottom - VIDEO_CONTROLS_STRIP) return;
    }
    onOpenViewer(clampIndex(index, items.length));
  }

  function handleTouchStart(event: React.TouchEvent) {
    const touch = event.touches[0];
    if (!touch) return;
    touchStartRef.current = { x: touch.clientX, y: touch.clientY };
  }

  function handleTouchEnd(event: React.TouchEvent) {
    const start = touchStartRef.current;
    touchStartRef.current = null;
    const touch = event.changedTouches[0];
    if (!start || !touch) return;
    const dx = touch.clientX - start.x;
    const dy = touch.clientY - start.y;
    // 水平滑动翻页：仅当水平位移明显大于垂直位移，避免与页面滚动冲突。
    if (Math.abs(dx) > SWIPE_THRESHOLD && Math.abs(dx) > Math.abs(dy)) {
      if (dx < 0) goNext();
      else goPrevious();
      return;
    }
    // 移动端连续浏览（#89）：媒体集最后一项继续上滑时触发，桌面端不触发。
    if (
      onReachEnd &&
      clampIndex(index, items.length) === items.length - 1 &&
      dy < -SWIPE_THRESHOLD &&
      Math.abs(dy) > Math.abs(dx)
    ) {
      onReachEnd();
    }
  }

  const currentFailed = Boolean(failed[current.id]);
  const currentLoaded = Boolean(loaded[current.id]);

  function settleFirstMedia(state: "ready" | "error") {
    if (firstSettledRef.current || !onFirstMediaSettled) return;
    firstSettledRef.current = true;
    onFirstMediaSettled(state);
  }

  return (
    <section
      data-slot="detail-cover"
      className={cn(
        "relative overflow-hidden rounded-lg border border-border-default bg-canvas-default",
        className,
      )}
    >
      <div
        onTouchStart={handleTouchStart}
        onTouchEnd={handleTouchEnd}
        className="relative w-full overflow-y-auto"
        style={{
          aspectRatio: String(itemAspectRatio(first)),
          maxHeight: ultraTallContainer ? ULTRA_TALL_MAX_HEIGHT : undefined,
        }}
      >
        {items.map((item, itemIndex) => {
          const active = itemIndex === clampIndex(index, items.length);
          const tall = isUltraTall(item);
          const itemFailed = Boolean(failed[item.id]);
          const itemLoaded = Boolean(loaded[item.id]);
          return (
            <div
              key={item.id}
              aria-current={active ? "true" : undefined}
              className={cn("relative", active ? cn("block", !tall && "h-full") : "hidden")}
              onClick={active ? handleMediaClick : undefined}
            >
              {item.type === "image" ? (
                <>
                  {!itemFailed && !itemLoaded && (
                    <div
                      aria-hidden="true"
                      className={cn(
                        "absolute inset-0 animate-pulse bg-muted",
                        active ? undefined : "hidden",
                      )}
                    />
                  )}
                  {itemFailed ? (
                    <div className="flex h-full min-h-40 w-full flex-col items-center justify-center gap-2 text-muted-foreground">
                      <ImageOff className="h-8 w-8" aria-hidden="true" />
                      <span className="text-xs">{t("media.gallery.error.loadFailed")}</span>
                    </div>
                  ) : (
                    <img
                      src={item.url}
                      alt={t("media.gallery.imageAlt", {
                        current: itemIndex + 1,
                        total: items.length,
                      })}
                      className={cn(
                        "w-full object-contain",
                        tall ? "h-auto" : "h-full",
                      )}
                      onLoad={() => {
                        setLoaded((prev) => ({ ...prev, [item.id]: true }));
                        if (itemIndex === 0) settleFirstMedia("ready");
                      }}
                      onError={() => {
                        setFailed((prev) => ({ ...prev, [item.id]: true }));
                        if (itemIndex === 0) settleFirstMedia("error");
                      }}
                    />
                  )}
                </>
              ) : (
                <video
                  src={item.url}
                  controls
                  preload="metadata"
                  poster={item.posterUrl}
                  className={cn(
                    "w-full object-contain",
                    tall ? "h-auto" : "h-full",
                  )}
                  onLoadedMetadata={() => {
                    if (itemIndex === 0) settleFirstMedia("ready");
                  }}
                  onError={() => {
                    setFailed((prev) => ({ ...prev, [item.id]: true }));
                    if (itemIndex === 0) settleFirstMedia("error");
                  }}
                />
              )}
            </div>
          );
        })}
      </div>

      {showControls && (
        <div className="flex items-center justify-between gap-2 border-t border-border-default px-3 py-2">
          <button
            type="button"
            onClick={goPrevious}
            disabled={clampIndex(index, items.length) === 0}
            aria-label={t("media.gallery.previous")}
            className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring disabled:pointer-events-none disabled:opacity-40"
          >
            <ChevronLeft className="h-4 w-4" aria-hidden="true" />
          </button>
          <div role="group" aria-label={positionLabel} className="flex items-center gap-2">
            {items.map((item, itemIndex) => (
              <span
                key={item.id}
                aria-hidden="true"
                className={cn(
                  "h-2 w-2 rounded-full transition-colors",
                  itemIndex === clampIndex(index, items.length)
                    ? "bg-foreground"
                    : "bg-muted-foreground/30",
                )}
              />
            ))}
          </div>
          <button
            type="button"
            onClick={goNext}
            disabled={clampIndex(index, items.length) === items.length - 1}
            aria-label={t("media.gallery.next")}
            className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring disabled:pointer-events-none disabled:opacity-40"
          >
            <ChevronRight className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>
      )}
    </section>
  );
}
