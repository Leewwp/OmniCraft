"use client";

import { useEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { useTranslations } from "next-intl";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";
import { coverRenderSrc } from "@/lib/overlay-motion";
import { HoldCrossfadeImage } from "@/components/content/HoldCrossfadeImage";
import { MediaViewer } from "@/components/content/MediaViewer";
import { useAuthGate } from "@/components/auth/AuthGateProvider";
import type { MediaGalleryItem } from "@/components/content/MediaGallery";
import { OVERLAY_DEFAULT_RATIO, isLongImageItem, itemAspectRatio } from "@/lib/overlay-media";

/**
 * 内容详情浮窗「统一版式」（D1 #858，桌面 ≥960 全类型唯一版式；几何承袭 #397
 * R2 竖屏集新版布局）：左媒体列贴边满幅、列宽 = 可用高 × 当前项真实比例
 * （240ms 过渡；长图缩窄居中完整显示、无内部滚动）；控件 = 悬浮半透明箭头 +
 * 底部指示点 + 右上角标，图片主体点击进查看器；右栏独立滚动
 * （data-slot="layer-scroller"，滚动条贴面板右缘全高贯通）。
 *
 * 媒体列直贴面板（宿主容器桌面零内边距，不再用负 margin 抵消 overlay-scroller
 * 的 px/pt/pb）；loading/错误态媒体链为空时媒体列保持稳定黑底占位（双栏外壳
 * 不塌缩、不回退全宽单列）。
 *
 * D2 #859 媒体列几何与视频行为（spec §3.1/§3.3/§9.3）：
 * - 视频全套小红书行为：活跃可见项 muted+playsInline 自动播放；主体点击单次
 *   切换播放/暂停（原生 controls 条 44px 内点击不切播、不翻页、不进查看器）；
 *   play() 被浏览器策略拒绝时保留 controls 手动播放入口；视频不再进 MediaViewer
 *   （全屏用控制条原生按钮）。
 * - 两段式根除：共享有界就绪预算（lib MEDIA_READY_BUDGET_MS）先于入场保险
 *   结清 chainReady；冻结窗口（freezeGeometry = 入场转场起跑→落定）内锁存
 *   起跑比例——迟到元数据不得在同次入场动画中改列宽，落定后随切位/回填平滑
 *   调整；contain 完整呈现允许黑边。
 * - 挂起矩阵：非顶层（压层）、上层查看器、登录浮窗打开时停止隐藏媒体；
 *   恢复仅当前项；任何时刻至多一段声音（同刻只挂载当前 slide）。
 * - 三件套可用性：仅多项显示；边界 clamp 不循环；hover 与 focus-within 均可见；
 *   触控目标 44px；键盘 ←/→ 只作用于顶层活动媒体区，不劫持输入框/评论编辑器/
 *   播放器控件/上层查看器/登录浮窗；切位按钮点击不触发放大。
 *
 * 布局不变量（R3 动效契约挂点）：data-slot="detail-cover" 锚点盒不含翻页控件，
 * 且仅在媒体链非空时渲染（空链不产出转场锚点，错误态维持居中缩淡降级）；
 * 媒体几何单一比例源 = 当前项 intrinsic（itemAspectRatio）。
 */

/** 右栏最小宽度：媒体列宽上限 = 根区宽 − 该值（保证文字列可读）。 */
const RIGHT_COL_MIN = 380;
/** 媒体列最小宽度。 */
const PANE_MIN_WIDTH = 280;
/** 逐张自适应几何过渡（SP-12 缓动口径）。 */
const PANE_TRANSITION = "width 240ms cubic-bezier(0.22,0.61,0.36,1)";
/** 视频 controls 条近似高度：点击该区域内属于原生控件语义（与 MediaGallery 一致）。 */
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

/** 当前项列宽（#753）：可用高 × 当前项真实比例（长图缩窄居中完整显示，不再按
    3:4 名义宽拉大 + 内滚），上限 = 根区宽 − 右栏最小宽（横图自然收窄 letterbox）。
    D1 #858：全类型统一（文字封面链 3:4 / 媒体集真实比例同一公式）；媒体链为空
    （loading/错误态）按防御比例 3:4 占位。 */
function paneWidth(area: { w: number; h: number } | null, ratio: number): string {
  if (!area) return `${Math.round(PANE_FALLBACK_RATIO * 100)}%`;
  const byHeight = area.h * ratio;
  const cap = Math.max(area.w - RIGHT_COL_MIN, PANE_MIN_WIDTH);
  return `${Math.round(Math.max(PANE_MIN_WIDTH, Math.min(byHeight, cap)))}px`;
}

/** 浏览器策略可能拒绝 play()（jsdom 返回 undefined 同样防御）：失败静默，
    controls 手动播放入口保留。 */
function attemptPlay(video: HTMLVideoElement): void {
  const attempt = video.play() as unknown as Promise<void> | undefined;
  if (attempt && typeof attempt.catch === "function") {
    attempt.catch(() => {
      /* 自动播放被拒：用户可用 controls 手动播放。 */
    });
  }
}

interface MediaSlideProps {
  item: MediaGalleryItem;
  index: number;
  total: number;
  /** 首项媒体加载落定/失败回调一次（驱动正文 reveal，同 split 路径 coverReady）。 */
  onSettle: (state: "ready" | "error") => void;
  /** #409 F1 首帧保持：入场未落定期首项图片渲染卡片封面 src（防换图）。 */
  holdSrc?: string | null;
  /** D2：当前 slide 的 <video> 引用（主体点击切播与挂起矩阵使用）。 */
  videoRef: RefObject<HTMLVideoElement | null>;
  /** D2：挂起信号（非顶层/上层查看器/登录浮窗）——true 停播，false 恢复当前项。 */
  suspended: boolean;
  /** D2：用户主动暂停标记（ref 语义）——恢复激活时不覆盖用户意图自动续播。 */
  userPausedRef: RefObject<boolean>;
  /** D2：挂载视频元数据回填（探针失败/超时后的迟到修正）。 */
  onVideoMetadata?: (id: number, width: number, height: number) => void;
}

/** 媒体项渲染（#753）：contain 不裁切、填满锚点盒（允许黑边）；长图随列宽
    等比缩窄居中完整显示（列宽按真实比例求取，无内部竖向滚动）。
    #409 F1 同源图 + #430 两变体渐进：规范层 w=1080（coverRenderSrc），
    首项入场保持层 = 卡片快变体（holdSrc），落定且规范层就绪后 180ms 交叉淡入；
    next/image 的响应式 sizes 无法钉死变体，故用受控 <img>。 */
function MediaSlide({
  item,
  index,
  total,
  onSettle,
  holdSrc,
  videoRef,
  suspended,
  userPausedRef,
  onVideoMetadata,
}: MediaSlideProps) {
  const t = useTranslations();
  const settleIfFirst = (state: "ready" | "error") => {
    if (index === 0) onSettle(state);
  };
  const alt = t("media.gallery.imageAlt", { current: index + 1, total });

  /* D2 视频生命周期：活跃且可见时自动静音播放；挂起（压层/上层弹窗）停播；
     恢复仅当前项续播且不覆盖用户主动暂停；切位（item 身份变化）= 显式停旧播新
     （React 对同位 <video> 只 patch src，不重挂载，必须显式接管）；卸载清理
     兜底停播。 */
  useEffect(() => {
    if (item.type !== "video") return;
    const video = videoRef.current;
    if (!video) return;
    if (suspended) {
      video.pause();
      return;
    }
    if (!userPausedRef.current) attemptPlay(video);
    return () => {
      video.pause();
    };
  }, [item.type, item.id, item.url, suspended, userPausedRef, videoRef]);

  if (item.type === "video") {
    return (
      <div className="relative h-full">
        <video
          ref={videoRef}
          src={item.url}
          controls
          autoPlay
          muted
          playsInline
          preload="metadata"
          poster={item.posterUrl}
          className="h-full w-full object-contain"
          onLoadedMetadata={() => {
            const video = videoRef.current;
            if (video && video.videoWidth > 0 && video.videoHeight > 0) {
              onVideoMetadata?.(item.id, video.videoWidth, video.videoHeight);
            }
            settleIfFirst("ready");
          }}
          onError={() => settleIfFirst("error")}
        />
      </div>
    );
  }

  return (
    <div className="relative h-full">
      <HoldCrossfadeImage
        canonicalSrc={coverRenderSrc(item.url) || item.url}
        holdSrc={index === 0 ? holdSrc : null}
        alt={alt}
        imgClassName="cursor-zoom-in object-contain"
        onSettle={settleIfFirst}
      />
    </div>
  );
}

/** 悬浮半透明圆形翻页箭头（D2：hover 与 focus-within 显现；触屏（hover:none）
    恒显保证可发现入口；44px 触控目标）。 */
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
        "absolute top-1/2 z-10 flex h-11 w-11 min-h-11 min-w-11 -translate-y-1/2 items-center justify-center rounded-full bg-black/35 text-white opacity-0 backdrop-blur transition-opacity duration-150",
        "group-hover/media:opacity-100 group-focus-within/media:opacity-100 [@media(hover:none)]:opacity-100",
        "hover:bg-black/50 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-0",
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

/** 右上角页码角标（小红书式「1 / 5」；D2：随三件套 hover/focus-within 显现，
    触屏（hover:none）恒显）。 */
function CountBadge({ current, total }: { current: number; total: number }) {
  const t = useTranslations();
  return (
    <span
      data-slot="media-count-badge"
      aria-label={t("media.gallery.position", { current, total })}
      className="absolute right-3 top-3 z-10 rounded-full bg-black/45 px-2.5 py-0.5 text-xs tabular-nums text-white opacity-0 backdrop-blur transition-opacity duration-150 group-hover/media:opacity-100 group-focus-within/media:opacity-100 [@media(hover:none)]:opacity-100"
    >
      {current} / {total}
    </span>
  );
}

/** 底部半透明指示点（D2：默认唯一常显件 = 触屏可发现入口）。 */
function BottomDots({ media, index }: { media: MediaGalleryItem[]; index: number }) {
  return (
    <div
      data-slot="media-paging-dots"
      className="absolute bottom-3 left-1/2 z-10 flex -translate-x-1/2 items-center gap-1.5 rounded-full bg-black/25 px-2 py-1 backdrop-blur"
    >
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
  /** #409 F1 / D2 几何时序：true = 入场转场起跑→落定的冻结窗口——窗口内锁存
      起跑比例（迟到元数据不得在同次入场动画中改列宽）；窗口外跟随真实比例。 */
  freezeGeometry?: boolean;
  /** #409 F1 几何稳定门：根区完成首次实测（ResizeObserver 回报）后回调一次，
      浮层据此放行入场转场（测量在几何稳定后进行）。 */
  onPaneMeasured?: () => void;
  /** D2：本层是否为顶层活动层（键盘翻页/媒体挂起矩阵只作用于顶层；
      压入下一层、被上层弹窗覆盖时 false = 挂起）。 */
  active?: boolean;
  /** D2：挂载视频元数据回填（探针失败/超时后的迟到修正，落定后平滑应用）。 */
  onMediaDimensions?: (id: number, width: number, height: number) => void;
  /** 右栏内容（创作者行/标题/正文/关联块/评论区/推荐区 = 统一钉死序）。 */
  children: ReactNode;
}

export function OverlayVariantLayout({
  media,
  onFirstMediaSettled,
  holdSrc,
  freezeGeometry,
  onPaneMeasured,
  active = true,
  onMediaDimensions,
  children,
}: OverlayVariantLayoutProps) {
  const { isGateOpen } = useAuthGate();
  const [index, setIndex] = useState(0);
  const [viewerIndex, setViewerIndex] = useState<number | null>(null);
  const { ref: rootRef, area } = useMediaArea();
  const firstSettledRef = useRef(false);
  const measuredOnceRef = useRef(false);
  const viewerTriggerRef = useRef<HTMLElement | null>(null);
  const videoRef = useRef<HTMLVideoElement | null>(null);
  /* D2：用户主动暂停意图（切位换片时复位——换片是轮播语义，不是暂停意图）。 */
  const userPausedRef = useRef(false);

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
  /* 媒体链为空（loading/错误态）按防御比例 3:4 占位，双栏外壳几何稳定。 */
  const liveRatio = current ? itemAspectRatio(current) : OVERLAY_DEFAULT_RATIO;

  /* D2 两段式根除：冻结窗口（入场转场起跑→落定）锁存起跑时的比例——起跑前
     壳层 opacity 0，比例修正即时生效不可见；窗口内迟到元数据不改列宽（零跳宽）；
     落定后清除锁存，真实比例随 240ms 过渡平滑应用（切位/回填正常调整）。 */
  const [frozenRatio, setFrozenRatio] = useState<number | null>(null);
  const prevFreezeRef = useRef(false);
  useEffect(() => {
    const frozen = Boolean(freezeGeometry);
    if (frozen && !prevFreezeRef.current) {
      setFrozenRatio(liveRatio);
    } else if (!frozen && frozenRatio !== null) {
      setFrozenRatio(null);
    }
    prevFreezeRef.current = frozen;
  }, [freezeGeometry, liveRatio, frozenRatio]);
  const ratio = frozenRatio ?? liveRatio;
  const tall = isLongImageItem(current);

  /* D2 挂起矩阵：非顶层（压入下一层/被压）、上层查看器、登录浮窗任一成立即
     停止隐藏媒体；恢复时仅当前 slide（唯一挂载者）续播（用户主动暂停不覆盖）。 */
  const suspended = !active || viewerIndex !== null || isGateOpen;

  const settleFirstMedia = (state: "ready" | "error") => {
    if (firstSettledRef.current || !onFirstMediaSettled) return;
    firstSettledRef.current = true;
    onFirstMediaSettled(state);
  };

  /* 点击媒体：图片进入 MediaViewer 看大图（关闭后焦点还给触发媒体区，AC4）；
     视频主体 = 单次切换播放/暂停（controls 条 44px 内点击属原生控件语义，
     不切播、不翻页、不进查看器——全屏用控制条原生按钮，D2 #859）。用户主动
     暂停的意图被记录，恢复激活时不自动续播。 */
  function handleCoverClick(event: React.MouseEvent<HTMLDivElement>) {
    if (!current) return;
    if (isVideo) {
      const rect = event.currentTarget.getBoundingClientRect();
      if (rect.height > 0 && event.clientY > rect.bottom - VIDEO_CONTROLS_STRIP) return;
      /* 原生全屏内点击交还原生控件（Blink 全屏路径不理会 defaultPrevented）；
         其余主体点击必须 preventDefault——WebKit 显示 controls 时对主体 click
         有默认 togglePlayState，不抑制会与 JS 切播双触发互相抵消（审查 N1：
         Safari 主体切播整体失效、Chrome 原生全屏内失效）。 */
      if (document.fullscreenElement) return;
      event.preventDefault();
      const video = videoRef.current;
      if (!video) return;
      if (video.paused) {
        userPausedRef.current = false;
        attemptPlay(video);
      } else {
        userPausedRef.current = true;
        video.pause();
      }
      return;
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

  /* D2 键盘 ←/→：只作用于顶层活动媒体区；clamp 不循环；不劫持输入框/评论
     编辑器（contenteditable）/select、播放器控件（video/audio 原生快进快退）、
     上层查看器（viewerIndex 打开）与登录浮窗（isGateOpen）。切位复位用户暂停
     意图（换片是轮播语义，不是暂停意图）。 */
  const changeSlide = (update: (value: number) => number) => {
    userPausedRef.current = false;
    setIndex(update);
  };

  useEffect(() => {
    if (!active || media.length < 2) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      /* Radix menu/listbox 等已消费的箭头事件不再叠加切位（审查 N2）。 */
      if (event.defaultPrevented) return;
      if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
      if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
      if (viewerIndex !== null || isGateOpen) return;
      const target = event.target;
      if (target instanceof HTMLElement) {
        if (
          target.closest("input, textarea, select, video, audio, [contenteditable]") ||
          target.isContentEditable
        ) {
          return;
        }
      }
      event.preventDefault();
      if (event.key === "ArrowLeft") {
        changeSlide((value) => Math.max(0, clampIndex(value, media.length) - 1));
      } else {
        changeSlide((value) => Math.min(media.length - 1, clampIndex(value, media.length) + 1));
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, media.length, viewerIndex, isGateOpen]);

  return (
    /* 贴边根容器：宿主（overlay-scroller）桌面零内边距，媒体列直贴面板——旧的
       负 margin（负 mx/mb/mt）抵消结构退役（D1 #858）。 */
    <div
      ref={rootRef}
      data-slot="variant-root"
      className="relative flex h-full min-h-0 w-full"
    >
      {/* 媒体列：宽 = 可用高 × 当前项比例（逐张自适应过渡），黑底满幅贴边。
          媒体链为空（loading/错误态）保持稳定黑底占位——双栏外壳不塌缩。
          #409 F1：入场未落定（freezeGeometry）时 width 过渡关闭；D2：冻结窗口
          内比例锁存（迟到元数据零跳宽）。 */}
      <div
        data-slot="variant-media-pane"
        className="group/media relative h-full shrink-0 overflow-hidden bg-black"
        style={{
          width: paneWidth(area, ratio),
          transition: freezeGeometry ? "none" : PANE_TRANSITION,
        }}
      >
        {current && (
          <>
            {/* 锚点盒（R3 契约）：不含翻页控件；#753 取消内部竖向滚动（长图随列宽
                缩窄居中完整显示）。data-ultra-tall：长图当前项标记——浮层转场据此
                退化居中缩淡（C2，卡片两档裁切 vs 面板 contain 取景无法统一）。
                空链（错误态）不渲染锚点：浮层转场维持居中缩淡降级。 */}
            <div
              data-slot="detail-cover"
              data-ultra-tall={tall ? "true" : undefined}
              className="h-full w-full overflow-hidden"
              onClick={handleCoverClick}
            >
              <MediaSlide
                item={current}
                index={at}
                total={media.length}
                onSettle={settleFirstMedia}
                holdSrc={holdSrc}
                videoRef={videoRef}
                suspended={suspended}
                userPausedRef={userPausedRef}
                onVideoMetadata={onMediaDimensions}
              />
            </div>
            {showControls && (
              <>
                {/* #753：翻页由悬停/键盘聚焦可见的显式箭头承担；旧左右 1/3 隐形
                    点击热区与「点击放大进查看器」冲突，已移除。D2：hover 与
                    focus-within 显现、触屏恒显可发现、44px 触控目标、边界 clamp、
                    键盘 ←/→、切位不触发放大。 */}
                <HoverArrow side="left" disabled={at === 0} onClick={() => changeSlide((value) => Math.max(0, clampIndex(value, media.length) - 1))} />
                <HoverArrow
                  side="right"
                  disabled={at === media.length - 1}
                  onClick={() => changeSlide((value) => Math.min(media.length - 1, clampIndex(value, media.length) + 1))}
                />
                <CountBadge current={at + 1} total={media.length} />
                <BottomDots media={media} index={at} />
              </>
            )}
          </>
        )}
      </div>

      {/* 右文字列：浮层内唯一滚动容器（滚动条贴面板右缘全高贯通）；× 悬浮面板
          右上覆盖内容，不留整行避让空带（首块右上局部避让由 ContentDetail
          creatorFirst 头部承担）。 */}
      <div
        data-slot="layer-scroller"
        className="min-h-0 min-w-0 flex-1 overflow-y-auto overscroll-contain bg-card px-5 pb-4 pt-4"
      >
        {children}
      </div>

      {media.length > 0 && (
        <MediaViewer
          items={media}
          index={viewerIndex ?? 0}
          open={viewerIndex !== null}
          onOpenChange={handleViewerOpenChange}
          onIndexChange={setViewerIndex}
        />
      )}
    </div>
  );
}
