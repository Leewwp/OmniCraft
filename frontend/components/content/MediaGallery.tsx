"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { ChevronLeft, ChevronRight, ImageOff } from "lucide-react";
import { cn } from "@/lib/utils";
import { coverRenderSrc } from "@/lib/overlay-motion";
import { HoldCrossfadeImage } from "@/components/content/HoldCrossfadeImage";
import { MediaViewer } from "@/components/content/MediaViewer";
import type { AttachmentData } from "@/lib/content";
import { LONG_IMAGE_RATIO } from "@/lib/overlay-media";

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
  /** 移动端连续浏览：#89 媒体集最后一项继续上滑时触发。
      #753：仅在最近可滚祖先滚到实际内容末端后才放行（纵读长图不误跳篇）。 */
  onReachEnd?: () => void;
  /** 浮层封面同步（#64 决策 11）：首项媒体加载落定/失败时回调一次，驱动主体 reveal。 */
  onFirstMediaSettled?: (state: "ready" | "error") => void;
  /** #409 F1 首帧保持：入场转场未落定期间，首项图片以卡片封面 src 渲染
      （媒体链与卡片封面不同文件时防动效期间换图）；落定后由上层清除。 */
  firstItemHoldSrc?: string | null;
}

/** 防御性默认比例（AC4）：宽高缺失/历史数据时使用 3:4，不报错不隐藏。 */
const DEFAULT_ASPECT_RATIO = 3 / 4;
/* #753 长图阈值 = overlay-media LONG_IMAGE_RATIO（单一共享常量，含等号）。 */
/** #753 最低容器占位：0.75×媒体可用宽（宽高 4:3 的最低占位；无侧边距时
    等价 75vw）。横图不足该高度时留空居中，不放大不裁切。 */
const MIN_CONTAINER_HEIGHT = "75cqw";
/** #753 折叠长图高度：顶部 3:4 区域 = 4W/3（显式 cqw 高度 + overflow-hidden，
    流式图片从顶渲染被裁出顶部 3:4 取景；展开后该高度约束移除）。 */
const FOLDED_LONG_IMAGE_HEIGHT = "133.333cqw";
/** #753 图片布局分端断点：<960 移动语义（通栏自然比例 + 0.75W 最低占位 +
    长图折叠/就地展开）；≥960 PC 语义（当前项 contain、无内部滚动、无折叠，
    长图随容器等比缩窄居中）。与浮层图片布局门同值。 */
const MOBILE_MEDIA_QUERY = "(max-width: 959px)";
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

/** #753：当前项决定几何与长图判定（首项锁高退役）；含边界等号。 */
function isLongImage(item: MediaGalleryItem): boolean {
  if (item.type !== "image") return false;
  if (!item.width || !item.height || item.width <= 0 || item.height <= 0) return false;
  return item.height / item.width >= LONG_IMAGE_RATIO;
}

/** #753 连续浏览放行条件：最近可滚祖先（浮层滚动层或页面）已到实际内容
    末端；无内部滚动容器时以 window 兜底。纵读折叠长图时内容未到末端，
    上滑不切篇；展开读到末尾后继续上滑才触发。 */
function nearestScrollerAtEnd(start: HTMLElement | null): boolean {
  let node = start?.parentElement ?? null;
  while (node) {
    const style = window.getComputedStyle(node);
    const scrollsY =
      (style.overflowY === "auto" || style.overflowY === "scroll") &&
      node.scrollHeight > node.clientHeight + 2;
    if (scrollsY) {
      return node.scrollTop + node.clientHeight >= node.scrollHeight - 2;
    }
    node = node.parentElement;
  }
  const doc = document.documentElement;
  return window.innerHeight + window.scrollY >= doc.scrollHeight - 2;
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

/**
 * 详情/浮层媒体区（#85 起；#753 重做）：通栏自然比例（当前项决定高度），
 * 最低占位 0.75×W；长图（h/w ≥ 16/9）初始顶部 3:4 折叠 + 「查看长图」就地
 * 展开（交页面/浮层主体滚动，不保留 70vh 内部滚动）；指示点 + 滑动/按钮翻页；
 * 展开状态按内容/图片身份隔离（换内容复位，同集内切回同图保持）。
 */
export function MediaGallery({
  className,
  items,
  initialIndex,
  onOpenViewer,
  onReachEnd,
  onFirstMediaSettled,
  firstItemHoldSrc,
}: MediaGalleryProps) {
  const t = useTranslations();
  const [index, setIndex] = useState(() => clampIndex(initialIndex ?? 0, items.length));
  const [failed, setFailed] = useState<Record<number, boolean>>({});
  const [loaded, setLoaded] = useState<Record<number, boolean>>({});
  const [viewerIndex, setViewerIndex] = useState<number | null>(null);
  /* #753 长图就地展开状态：键 = 媒体项 id（内容内稳定）；换内容（items
     身份变化）复位，同一查看过程切回同图保持展开。 */
  const [expandedIds, setExpandedIds] = useState<Record<number, boolean>>({});
  const touchStartRef = useRef<{ x: number; y: number } | null>(null);
  const firstSettledRef = useRef(false);
  const viewerTriggerRef = useRef<HTMLElement | null>(null);
  const sectionRef = useRef<HTMLElement | null>(null);
  /* #753 分端：matchMedia 缺失（测试环境）按移动语义渲染。 */
  const [mobileLayout, setMobileLayout] = useState(true);
  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const mq = window.matchMedia(MOBILE_MEDIA_QUERY);
    const update = () => setMobileLayout(mq.matches);
    update();
    mq.addEventListener("change", update);
    return () => mq.removeEventListener("change", update);
  }, []);

  /* 换内容复位：展开状态与页码回到初始（同一媒体集内翻页不复位）。 */
  const itemsIdentity = items.map((item) => item.id).join(",");
  useEffect(() => {
    setExpandedIds({});
    setIndex(clampIndex(initialIndex ?? 0, items.length));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [itemsIdentity]);

  const goPrevious = useCallback(() => {
    setIndex((current) => Math.max(0, current - 1));
  }, []);

  const goNext = useCallback(() => {
    setIndex((current) => Math.min(items.length - 1, current + 1));
  }, [items.length]);

  if (items.length === 0) return null;

  const current = items[clampIndex(index, items.length)];
  const first = items[0];
  const showControls = items.length > 1;
  const positionLabel = t("media.gallery.position", {
    current: clampIndex(index, items.length) + 1,
    total: items.length,
  });

  /* #753 几何：当前项决定。图片 = 自然比例 contain + 0.75W 最低占位；
     长图折叠态 = 顶部 3:4（高 4W/3）裁切 + 渐隐 + 展开按钮；展开态 = 高度
     auto 交主体滚动。视频维持 w-full 自然比例（折叠规则不作用于视频）。 */
  const currentIsLongImage = isLongImage(current);
  const currentExpanded = Boolean(expandedIds[current.id]);
  const longImageCollapsed = mobileLayout && currentIsLongImage && !currentExpanded;

  function expandCurrent() {
    setExpandedIds((prev) => ({ ...prev, [current.id]: true }));
  }

  function handleMediaClick(event: React.MouseEvent<HTMLElement>) {
    if (current.type === "video") {
      const rect = event.currentTarget.getBoundingClientRect();
      if (rect.height > 0 && event.clientY > rect.bottom - VIDEO_CONTROLS_STRIP) return;
    }
    const targetIndex = clampIndex(index, items.length);
    viewerTriggerRef.current = event.currentTarget;
    // onOpenViewer 已由上层消费时交给上层（#88/#89 预留给双栏/连续浏览的入口），
    // 未消费则内部自持状态渲染 MediaViewer（当前唯一消费方）。
    if (onOpenViewer) {
      onOpenViewer(targetIndex);
      return;
    }
    setViewerIndex(targetIndex);
  }

  /** 查看器关闭后把焦点还给触发媒体区（AC4：恢复触发点焦点）。 */
  const handleViewerOpenChange = useCallback((open: boolean) => {
    if (open) return;
    setViewerIndex(null);
    const trigger = viewerTriggerRef.current;
    viewerTriggerRef.current = null;
    if (trigger) {
      requestAnimationFrame(() => trigger.focus({ preventScroll: true }));
    }
  }, []);

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
    // 移动端连续浏览（#89，#753 收紧）：最后一项上滑，且最近可滚祖先已到
    // 实际内容末端（折叠长图未展开/未读到底不切篇）。
    if (
      onReachEnd &&
      clampIndex(index, items.length) === items.length - 1 &&
      dy < -SWIPE_THRESHOLD &&
      Math.abs(dy) > Math.abs(dx) &&
      nearestScrollerAtEnd(sectionRef.current)
    ) {
      onReachEnd();
    }
  }

  const currentFailed = Boolean(failed[current.id]);
  void currentFailed;
  const currentLoaded = Boolean(loaded[current.id]);
  void currentLoaded;

  function settleFirstMedia(state: "ready" | "error") {
    if (firstSettledRef.current || !onFirstMediaSettled) return;
    firstSettledRef.current = true;
    onFirstMediaSettled(state);
  }

  /* 容器高度决策（当前项）：
     - 折叠长图：aspect-ratio 3/4（高 = 4W/3），内容 overflow-hidden 顶部对齐；
     - 普通图片：aspect-ratio = 当前项比例 + min-height 75cqw（container-type
       在外层容器上，W = 媒体区自身宽）；
     - 视频与展开长图：高度自然（无 aspect-ratio / min-height）。
     首项几何标记（data-ultra-tall → 转场居中缩淡降级）沿用首项长图判定。 */
  const containerStyle: React.CSSProperties = longImageCollapsed
    ? { height: FOLDED_LONG_IMAGE_HEIGHT }
    : mobileLayout && current.type === "image" && !currentExpanded
      ? { aspectRatio: String(itemAspectRatio(current)), minHeight: MIN_CONTAINER_HEIGHT }
      : { aspectRatio: current.type === "image" ? String(itemAspectRatio(current)) : undefined, maxHeight: "100%" };
  const firstIsLongImage = isLongImage(first);

  return (
    <section
      ref={sectionRef}
      data-slot="detail-cover"
      /* #398 C2：首项长图标记——浮层转场据此退化为居中缩淡（卡片两档裁切与
         详情自然比例取景无法统一）。 */
      data-ultra-tall={firstIsLongImage ? "true" : undefined}
      className={cn("relative w-full bg-card [container-type:inline-size]", className)}
    >
      <div
        onTouchStart={handleTouchStart}
        onTouchEnd={handleTouchEnd}
        className="relative w-full overflow-hidden"
        style={containerStyle}
      >
        {items.map((item, itemIndex) => {
          const active = itemIndex === clampIndex(index, items.length);
          const itemFailed = Boolean(failed[item.id]);
          const itemLoaded = Boolean(loaded[item.id]);
          const itemCollapsedLong = mobileLayout && active && isLongImage(item) && !expandedIds[item.id];
          const itemExpandedLong = mobileLayout && active && isLongImage(item) && expandedIds[item.id];
          return (
            <div
              key={item.id}
              aria-current={active ? "true" : undefined}
              tabIndex={active ? -1 : undefined}
              className={cn(
                "relative",
                /* PC 填充形态：wrapper 占满容器高（容器 aspect 驱动高度）；
                   移动折叠/展开形态：流式自然高度。 */
                active ? cn("block", !itemCollapsedLong && !itemExpandedLong && "h-full") : "hidden",
              )}
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
                    /* #753：普通图 = 容器内 object-contain 居中（横图留空不裁）；
                       折叠长图 = 自然流式从顶渲染，容器 overflow-hidden 裁出顶部
                       3:4 区域；展开长图 = 流式全高（主体滚动接管）。
                       #409 F1 同源图 + #430 两变体渐进保持。 */
                    itemCollapsedLong || itemExpandedLong ? (
                      <HoldCrossfadeImage
                        canonicalSrc={coverRenderSrc(item.url) || item.url}
                        holdSrc={itemIndex === 0 ? firstItemHoldSrc : null}
                        alt={t("media.gallery.imageAlt", {
                          current: itemIndex + 1,
                          total: items.length,
                        })}
                        imgClassName="w-full h-auto object-contain object-top"
                        flowLayout
                        onSettle={(state) => {
                          if (state === "ready") {
                            setLoaded((prev) => ({ ...prev, [item.id]: true }));
                            if (itemIndex === 0) settleFirstMedia("ready");
                          } else {
                            setFailed((prev) => ({ ...prev, [item.id]: true }));
                            if (itemIndex === 0) settleFirstMedia("error");
                          }
                        }}
                      />
                    ) : (
                      <HoldCrossfadeImage
                        canonicalSrc={coverRenderSrc(item.url) || item.url}
                        holdSrc={itemIndex === 0 ? firstItemHoldSrc : null}
                        alt={t("media.gallery.imageAlt", {
                          current: itemIndex + 1,
                          total: items.length,
                        })}
                        imgClassName="absolute inset-0 h-full w-full object-contain"
                        onSettle={(state) => {
                          if (state === "ready") {
                            setLoaded((prev) => ({ ...prev, [item.id]: true }));
                            if (itemIndex === 0) settleFirstMedia("ready");
                          } else {
                            setFailed((prev) => ({ ...prev, [item.id]: true }));
                            if (itemIndex === 0) settleFirstMedia("error");
                          }
                        }}
                      />
                    )
                  )}
                  {/* 折叠长图底部渐隐 + 「查看长图」就地展开（不冒泡打开查看器） */}
                  {itemCollapsedLong && (
                    <>
                      <div
                        aria-hidden="true"
                        className="pointer-events-none absolute inset-x-0 bottom-0 h-28 bg-gradient-to-t from-card via-card/80 to-transparent"
                      />
                      <button
                        type="button"
                        onClick={(event) => {
                          event.stopPropagation();
                          expandCurrent();
                        }}
                        className="absolute bottom-4 left-1/2 z-10 -translate-x-1/2 rounded-full bg-background/90 px-4 py-2 text-xs font-medium text-foreground shadow-[var(--elevation-2)] backdrop-blur transition-colors hover:bg-background focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring min-h-11"
                      >
                        {t("media.gallery.viewFullImage")}
                      </button>
                    </>
                  )}
                </>
              ) : (
                <video
                  src={item.url}
                  controls
                  preload="metadata"
                  poster={item.posterUrl}
                  className={cn("w-full object-contain", mobileLayout ? "h-auto" : "h-full")}
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
            className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:outline-none focus-visible:ring-1 focus:ring-ring disabled:pointer-events-none disabled:opacity-40"
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
            className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:outline-none focus-visible:ring-1 focus:ring-ring disabled:pointer-events-none disabled:opacity-40"
          >
            <ChevronRight className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>
      )}

      <MediaViewer
        items={items}
        index={viewerIndex ?? 0}
        open={viewerIndex !== null}
        onOpenChange={handleViewerOpenChange}
        onIndexChange={setViewerIndex}
      />
    </section>
  )
}
