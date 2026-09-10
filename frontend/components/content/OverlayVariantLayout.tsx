"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslations } from "next-intl";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";
import { coverRenderSrc } from "@/lib/overlay-motion";
import { HoldCrossfadeImage } from "@/components/content/HoldCrossfadeImage";
import { MediaViewer } from "@/components/content/MediaViewer";
import type { MediaGalleryItem } from "@/components/content/MediaGallery";
import { isUltraTallItem, itemAspectRatio } from "@/lib/overlay-media";

/**
 * 内容详情浮窗「竖屏集新版布局」（#397 R2，胜者记录 §7 = A 基底 + C 逐张自适应几何
 * + 方案二 float 壳层）。生产重写（原型分支 prototype/overlay-rework 仅作参照）：
 * 左媒体列贴边满幅、列宽 = 可用高 × 当前图比例（240ms 过渡）、超高图按 3:4 名义宽
 * 取列宽 + 内部滚动；控件 = 悬浮半透明箭头 + 底部指示点 + 右上角标 + 图片左右 1/3
 * 点击翻页、中间 1/3 看大图；右栏独立滚动（data-slot="layer-scroller"）。
 *
 * 布局不变量（R3 动效契约挂点）：data-slot="detail-cover" 锚点盒不含翻页控件；
 * 媒体几何单一比例源 = 当前项 intrinsic（itemAspectRatio）。
 */

/** 右栏最小宽度：媒体列宽上限 = 根区宽 − 该值（保证文字列可读）。 */
const RIGHT_COL_MIN = 380;
/** 媒体列最小宽度。 */
const PANE_MIN_WIDTH = 280;
/** 逐张自适应几何过渡（SP-12 缓动口径）。 */
const PANE_TRANSITION = "width 240ms cubic-bezier(0.22,0.61,0.36,1)";
/** 视频 controls 条近似高度：点击该区域内不进入查看器（与 MediaGallery 一致）。 */
const VIDEO_CONTROLS_STRIP = 44;
/** 测量未就绪时的媒体列初始宽度比例。 */
const PANE_FALLBACK_RATIO = 0.58;

function clampIndex(value: number, length: number): number {
  if (!Number.isFinite(value) || length <= 0) return 0;
  return Math.min(Math.max(Math.floor(value), 0), length - 1);
}

/** 根区可用宽高测量（ResizeObserver）：媒体列宽 = 可用高 × 当前图比例的基准。 */
function useMediaArea() {
  const ref = useRef<HTMLDivElement>(null);
  const [area, setArea] = useState<{ w: number; h: number } | null>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver((entries) => {
      const rect = entries[0]?.contentRect;
      if (rect) setArea({ w: rect.width, h: rect.height });
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  return { ref, area };
}

/** 当前图列宽：可用高 × 比例，上限 = 根区宽 − 右栏最小宽（横图自然收窄，黑底 letterbox）。
    超高图（h/w > 2）按 3:4 名义比例取列宽——图以该宽渲染必然超出列高，锚点盒内部
    竖向滚动（若按真实比例取宽则图恰好装满、永不滚动，两次原型踩坑勿再踩）。 */
function paneWidth(area: { w: number; h: number } | null, ratio: number, tall: boolean): string {
  if (!area) return `${Math.round(PANE_FALLBACK_RATIO * 100)}%`;
  const nominal = tall ? 3 / 4 : ratio;
  const byHeight = area.h * nominal;
  const cap = Math.max(area.w - RIGHT_COL_MIN, PANE_MIN_WIDTH);
  return `${Math.round(Math.max(PANE_MIN_WIDTH, Math.min(byHeight, cap)))}px`;
}

interface MediaSlideProps {
  item: MediaGalleryItem;
  index: number;
  total: number;
  /** 首项媒体加载落定/失败回调一次（驱动正文 reveal，同 split 路径 coverReady）。 */
  onSettle: (state: "ready" | "error") => void;
  /** #409 F1 首帧保持：入场未落定期首项图片渲染卡片封面 src（防换图）。 */
  holdSrc?: string | null;
}

/** 媒体项渲染：contain 不裁切；超高图 h-auto 超出锚点盒高度 → 内部滚动。
    #409 F1 同源图 + #430 两变体渐进：规范层 w=1080（coverRenderSrc），
    首项入场保持层 = 卡片快变体（holdSrc），落定且规范层就绪后 180ms 交叉淡入；
    next/image 的响应式 sizes 无法钉死变体，故用受控 <img>。 */
function MediaSlide({ item, index, total, onSettle, holdSrc }: MediaSlideProps) {
  const t = useTranslations();
  const tall = isUltraTallItem(item);
  const settleIfFirst = (state: "ready" | "error") => {
    if (index === 0) onSettle(state);
  };
  const alt = t("media.gallery.imageAlt", { current: index + 1, total });

  if (item.type === "video") {
    return (
      <div className={cn("relative", tall ? "h-auto" : "h-full")}>
        <video
          src={item.url}
          controls
          preload="metadata"
          poster={item.posterUrl}
          className={cn("w-full object-contain", tall ? "h-auto" : "h-full")}
          onLoadedMetadata={() => settleIfFirst("ready")}
          onError={() => settleIfFirst("error")}
        />
      </div>
    );
  }

  return (
    <div className={cn("relative", tall ? "h-auto" : "h-full")}>
      <HoldCrossfadeImage
        canonicalSrc={coverRenderSrc(item.url) || item.url}
        holdSrc={index === 0 ? holdSrc : null}
        alt={alt}
        imgClassName="cursor-zoom-in object-contain"
        flowLayout={tall}
        onSettle={settleIfFirst}
      />
    </div>
  );
}

/** 悬浮半透明圆形翻页箭头（hover 显现，小红书式）。 */
function HoverArrow({
  side,
  disabled,
  onClick,
}: {
  side: "left" | "right";
  disabled?: boolean;
  onClick: () => void;
}) {
  const t = useTranslations();
  return (
    <button
      type="button"
      aria-label={side === "left" ? t("media.gallery.previous") : t("media.gallery.next")}
      onClick={onClick}
      disabled={disabled}
      className={cn(
        "absolute top-1/2 z-10 flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-full bg-black/35 text-white opacity-0 backdrop-blur transition-opacity duration-150 hover:bg-black/50 focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-0",
        side === "left" ? "left-3" : "right-3",
      )}
    >
      {side === "left" ? (
        <ChevronLeft className="h-5 w-5" aria-hidden="true" />
      ) : (
        <ChevronRight className="h-5 w-5" aria-hidden="true" />
      )}
    </button>
  );
}

/** 图片左右 1/3 隐形点击翻页区（仅图片项；视频项的 controls 区不被遮挡）。
    纯指针热区：不进无障碍树/焦点序（翻页的可聚焦控件是悬浮箭头），避免与箭头
    出现重复的 accessible name。 */
function SideZones({
  onPrev,
  onNext,
  hasPrev,
  hasNext,
}: {
  onPrev: () => void;
  onNext: () => void;
  hasPrev: boolean;
  hasNext: boolean;
}) {
  return (
    <>
      <button
        type="button"
        aria-hidden="true"
        tabIndex={-1}
        onClick={onPrev}
        disabled={!hasPrev}
        className="absolute inset-y-0 left-0 z-0 w-1/3 cursor-pointer disabled:cursor-default"
      />
      <button
        type="button"
        aria-hidden="true"
        tabIndex={-1}
        onClick={onNext}
        disabled={!hasNext}
        className="absolute inset-y-0 right-0 z-0 w-1/3 cursor-pointer disabled:cursor-default"
      />
    </>
  );
}

/** 右上角页码角标（小红书式「1 / 5」）。 */
function CountBadge({ current, total }: { current: number; total: number }) {
  const t = useTranslations();
  return (
    <span
      aria-label={t("media.gallery.position", { current, total })}
      className="absolute right-3 top-3 z-10 rounded-full bg-black/45 px-2.5 py-0.5 text-xs tabular-nums text-white backdrop-blur"
    >
      {current} / {total}
    </span>
  );
}

/** 底部半透明指示点。 */
function BottomDots({ media, index }: { media: MediaGalleryItem[]; index: number }) {
  return (
    <div className="absolute bottom-3 left-1/2 z-10 flex -translate-x-1/2 items-center gap-1.5 rounded-full bg-black/25 px-2 py-1 backdrop-blur">
      {media.map((item, itemIndex) => (
        <span
          key={item.id}
          aria-hidden="true"
          className={cn(
            "h-1.5 w-1.5 rounded-full transition-colors",
            itemIndex === index ? "bg-white" : "bg-white/40",
          )}
        />
      ))}
    </div>
  );
}

export interface OverlayVariantLayoutProps {
  media: MediaGalleryItem[];
  /** 首项媒体加载落定信号（驱动正文 reveal，同 split 路径 coverReady 契约）。 */
  onFirstMediaSettled?: (state: "ready" | "error") => void;
  /** #409 F1 首帧保持：入场未落定期首项图片渲染卡片封面 src（防换图）。 */
  holdSrc?: string | null;
  /** #409 F1 起跑前几何冻结：true 时媒体列 width 过渡关闭（挂载期从百分比到
      实测像素的过渡不得发生在入场转场窗口内）；落定后恢复逐张过渡。 */
  freezeGeometry?: boolean;
  /** #409 F1 几何稳定门：根区完成首次实测（ResizeObserver 回报）后回调一次，
      浮层据此放行入场转场（测量在几何稳定后进行）。 */
  onPaneMeasured?: () => void;
  /** 右栏内容（标题/作者/正文/关联内容块/评论区 = 末块）。 */
  children: ReactNode;
}

export function OverlayVariantLayout({
  media,
  onFirstMediaSettled,
  holdSrc,
  freezeGeometry,
  onPaneMeasured,
  children,
}: OverlayVariantLayoutProps) {
  const [index, setIndex] = useState(0);
  const [viewerIndex, setViewerIndex] = useState<number | null>(null);
  const { ref: rootRef, area } = useMediaArea();
  const firstSettledRef = useRef(false);
  const measuredOnceRef = useRef(false);
  const viewerTriggerRef = useRef<HTMLElement | null>(null);

  /* #409 F1：首次实测到达即回报一次（后续 Resize 不再触发）。 */
  useEffect(() => {
    if (!area || measuredOnceRef.current) return;
    measuredOnceRef.current = true;
    onPaneMeasured?.();
  }, [area, onPaneMeasured]);

  const at = clampIndex(index, media.length);
  const current = media[at];
  const showControls = media.length > 1;
  const isVideo = current?.type === "video";
  const ratio = itemAspectRatio(current);
  const tall = isUltraTallItem(current);

  const settleFirstMedia = (state: "ready" | "error") => {
    if (firstSettledRef.current || !onFirstMediaSettled) return;
    firstSettledRef.current = true;
    onFirstMediaSettled(state);
  };

  /* 点击媒体（视频 controls 条除外）进入 MediaViewer 看大图；查看器关闭后
     焦点还给触发媒体区（AC4 恢复触发点焦点）。 */
  function handleCoverClick(event: React.MouseEvent<HTMLDivElement>) {
    if (isVideo) {
      const rect = event.currentTarget.getBoundingClientRect();
      if (rect.height > 0 && event.clientY > rect.bottom - VIDEO_CONTROLS_STRIP) return;
    }
    viewerTriggerRef.current = event.currentTarget;
    setViewerIndex(at);
  }

  function handleViewerOpenChange(open: boolean) {
    if (open) return;
    setViewerIndex(null);
    const trigger = viewerTriggerRef.current;
    viewerTriggerRef.current = null;
    if (trigger) requestAnimationFrame(() => trigger.focus({ preventScroll: true }));
  }

  if (media.length === 0) return null;

  return (
    /* 贴边根容器：负 margin 抵消 overlay-scroller 的 px-6/pt-4/pb-6（≥1100px），
       满幅铺满浮窗（壳层 float：媒体列直贴窗顶）。 */
    <div
      ref={rootRef}
      data-slot="variant-root"
      className="relative flex h-full min-h-0 w-full min-[1100px]:-mx-6 min-[1100px]:-mb-6 min-[1100px]:-mt-4 min-[1100px]:h-[calc(100%+2.5rem)]"
    >
      {/* 媒体列：宽 = 可用高 × 当前图比例（逐张自适应过渡），黑底满幅贴边。
          #409 F1：入场未落定（freezeGeometry）时 width 过渡关闭——挂载期从
          百分比兜底到实测像素的过渡不得与共享元素转场同窗发生。 */}
      <div
        data-slot="variant-media-pane"
        className="group/media relative h-full shrink-0 overflow-hidden bg-black"
        style={{
          width: paneWidth(area, ratio, tall),
          transition: freezeGeometry ? "none" : PANE_TRANSITION,
        }}
      >
        {/* 锚点盒（R3 契约）：不含翻页控件；超高图内部竖向滚动。
            data-ultra-tall：超高图当前项标记——浮层转场据此退化居中缩淡（C2，
            卡片 400px contain 整图 vs 面板 3:4 名义宽顶部裁切，取景无法统一）。 */}
        <div
          data-slot="detail-cover"
          data-ultra-tall={tall ? "true" : undefined}
          className="h-full w-full overflow-y-auto overflow-x-hidden"
          onClick={handleCoverClick}
        >
          <MediaSlide
            item={current}
            index={at}
            total={media.length}
            onSettle={settleFirstMedia}
            holdSrc={holdSrc}
          />
        </div>
        {showControls && (
          <>
            {!isVideo && (
              <SideZones
                onPrev={() => setIndex((value) => Math.max(0, value - 1))}
                onNext={() => setIndex((value) => Math.min(media.length - 1, value + 1))}
                hasPrev={at > 0}
                hasNext={at < media.length - 1}
              />
            )}
            <HoverArrow side="left" disabled={at === 0} onClick={() => setIndex((value) => Math.max(0, value - 1))} />
            <HoverArrow
              side="right"
              disabled={at === media.length - 1}
              onClick={() => setIndex((value) => Math.min(media.length - 1, value + 1))}
            />
            <CountBadge current={at + 1} total={media.length} />
            <BottomDots media={media} index={at} />
          </>
        )}
      </div>

      {/* 右文字列：浮层内唯一滚动容器（≥1100px）；pt-14 避让悬浮关闭钮（方案二 float）。 */}
      <div
        data-slot="layer-scroller"
        className="min-h-0 min-w-0 flex-1 overflow-y-auto overscroll-contain bg-card px-5 pb-4 pt-4 min-[1100px]:pt-14"
      >
        {children}
      </div>

      <MediaViewer
        items={media}
        index={viewerIndex ?? 0}
        open={viewerIndex !== null}
        onOpenChange={handleViewerOpenChange}
        onIndexChange={setViewerIndex}
      />
    </div>
  );
}
