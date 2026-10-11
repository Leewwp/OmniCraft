"use client";

import { lazy, Suspense, useEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { useTranslations } from "next-intl";
import { ChevronLeft, ChevronRight, Maximize2, RefreshCw } from "lucide-react";
import { cn } from "@/lib/utils";
import { coverRenderSrc } from "@/lib/overlay-motion";
import { HoldCrossfadeImage } from "@/components/content/HoldCrossfadeImage";
import { MediaViewer } from "@/components/content/MediaViewer";
import { useAuthGate } from "@/components/auth/AuthGateProvider";
import { AudioPlayer } from "@/components/content/AudioPlayer";
import { MIDIPlayer, OSMDRenderer } from "@/components/content/SheetMusicViewer";
import type { MediaGalleryItem, MediaSlideType } from "@/components/content/MediaGallery";
import { OVERLAY_DEFAULT_RATIO, isEnlargeableSlide, isLongImageItem, itemAspectRatio } from "@/lib/overlay-media";
import { getCoverPlaceholder } from "@/lib/coverPlaceholder";
import { usePublicConfig } from "@/lib/use-public-config";

/** D5 #862 查看器族懒分块：重依赖（mammoth/exceljs/three.js）不进主包，
 * 当前位才挂载（React.lazy + Suspense；轮播位仅在客户端浮层打开后渲染，
 * 无 SSR 直出路径）。 */
const DocumentViewer = lazy(() =>
  import("@/components/content/DocumentViewer").then((m) => ({ default: m.DocumentViewer })),
);
const ModelViewer = lazy(() =>
  import("@/components/content/ModelViewer").then((m) => ({ default: m.ModelViewer })),
);

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
 * D5 #862 可预览媒体链与逐类型轮播位（spec §9.6/§3.2）：
 * - 媒体链 = 逐附件 slide 序列（overlay-media selectOverlaySlides）：audio/
 *   model3d/document/text-pdf/sheet_music（musicxml/midi/pdf）逐附件进链，
 *   image/video 既有契约不变；无可预览项回退封面→文字封面。
 * - 逐类型位：audio = 占位图为底 + 底部 AudioPlayer 条；model3d/document =
 *   对应查看器（懒分块 + 预算/消毒语义不变）；pdf = 原生 embed；乐谱 =
 *   OSMD 横滚 / midi 回放条（箭头悬浮层不拦截谱内滚动）。audio/midi 播放条
 *   位挂起暂停（恢复不自动续播）、切位卸载停播；当前位为播放条形态时底部
 *   圆点/类型角标上移让位。当前位才挂载重型查看器，首屏不抓全部附件正文；
 *   查看器族名义几何 300×400（3:4），首项挂载即 settle。
 * - MediaViewer 收可放大序列（D6 #863：image/model3d/text_pdf，
 *   isEnlargeableSlide 谓词）——放大态内独立计数、边界 clamp；入口 =
 *   image 主体点击或右上放大钮、model3d/text_pdf 仅右上放大钮、其余位无
 *   入口；关闭返回最后所看项并恢复触发焦点。混排链显示逐 slide 类型角标；
 *   >9 项隐藏圆点只留 N/M 徽标 + 箭头；查看器位提供「刷新预览」有限重试
 *   （重取详情刷新签名 URL，按附件 id 锚定保持当前位）。
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

/** 圆点指示器渲染上限（D5 #862）：>9 项隐藏圆点，只留 N/M 徽标 + 箭头
 * （非媒体类型 slide 无数量上限）。 */
const MAX_DOT_ITEMS = 9;

/** D5 #862 播放条形态位（audio/midi）的底部控件让位高度：播放条贴底占位
 * （AudioPlayer/MIDIPlayer 条 + 12px 贴底内边距 ≈ 110px），圆点/类型角标
 * 上移到其上方。 */
const PLAYER_BAR_YIELD_CLASS = "bottom-32";

interface SlideViewerContext {
  item: MediaGalleryItem;
  contentId?: number;
  allowCopy?: boolean;
  suspended: boolean;
}

/** D5 #862 audio 位：audio 专属占位图为底 + 底部悬浮紧凑 AudioPlayer 条。
 * 起播权在用户；切位卸载停播释放、挂起暂停；右下角预留刷新钮命中区。 */
function AudioSlide({ item, contentId, allowCopy, suspended }: SlideViewerContext) {
  return (
    <div className="relative h-full w-full" data-slot="slide-audio">
      {/* eslint-disable-next-line @next/next/no-img-element -- data: SVG 占位图直通（不走图片优化器） */}
      <img
        src={getCoverPlaceholder("audio", "")}
        alt=""
        aria-hidden="true"
        className="absolute inset-0 h-full w-full object-contain"
      />
      <div className="absolute inset-x-0 bottom-0 pb-3 pl-3 pr-14">
        <AudioPlayer
          url={item.url}
          fileName={item.fileName ?? ""}
          fileSize={item.fileSize}
          contentId={contentId ?? 0}
          attachmentId={item.id}
          allowCopy={Boolean(allowCopy)}
          suspended={suspended}
        />
      </div>
    </div>
  );
}

/** D5 #862 model3d 位：ModelViewer 画布（拖转/滚轮缩放/四机位已有；画布
 * 点击不绑定——轮播切页依显式箭头）。 */
function Model3dSlide({ item, contentId, allowCopy }: SlideViewerContext) {
  const config = usePublicConfig();
  return (
    <div className="h-full w-full overflow-y-auto p-3" data-slot="slide-model3d">
      <Suspense fallback={null}>
        <ModelViewer
          url={item.url}
          fileName={item.fileName ?? ""}
          fileSize={item.fileSize}
          maxPreviewMB={config?.upload?.model3d_max_preview_mb}
          maxTriangles={config?.upload?.model3d_max_triangles}
          gcodeMaxLines={config?.upload?.gcode_max_lines}
          contentId={contentId ?? 0}
          attachmentId={item.id}
          allowCopy={Boolean(allowCopy)}
        />
      </Suspense>
    </div>
  );
}

/** D5 #862 document 位：DocumentViewer 全高容器（docx 消毒 HTML / xlsx·csv
 * 网格 / 超预算降级卡由查看器内部维持）。 */
function DocumentSlide({ item, contentId, allowCopy }: SlideViewerContext) {
  const config = usePublicConfig();
  return (
    <div className="h-full w-full overflow-y-auto p-3" data-slot="slide-document">
      <Suspense fallback={null}>
        <DocumentViewer
          url={item.url}
          fileName={item.fileName ?? ""}
          fileSize={item.fileSize}
          maxPreviewMB={config?.upload?.document_preview_max_mb}
          contentId={contentId ?? 0}
          attachmentId={item.id}
          allowCopy={Boolean(allowCopy)}
        />
      </Suspense>
    </div>
  );
}

/** D5 #862 乐谱位（musicxml）：OSMD 横滚容器——轮播箭头为悬浮小钮，
 * 不拦截谱内横向滚动。 */
function SheetMusicXmlSlide({ item }: SlideViewerContext) {
  return (
    <div className="h-full w-full overflow-y-auto bg-canvas-default p-3" data-slot="slide-sheet-music">
      <OSMDRenderer ossUrl={item.url} />
    </div>
  );
}

/** D5 #862 乐谱位（midi）：乐谱占位图为底 + 底部回放条贴底（挂起暂停、切位
 * 卸载即停，AudioContext 随组件清理关闭）。 */
function SheetMusicMidiSlide({ item, suspended }: SlideViewerContext) {
  return (
    <div className="relative h-full w-full" data-slot="slide-sheet-music">
      {/* eslint-disable-next-line @next/next/no-img-element -- data: SVG 占位图直通 */}
      <img
        src={getCoverPlaceholder("sheet_music", "")}
        alt=""
        aria-hidden="true"
        className="absolute inset-0 h-full w-full object-contain"
      />
      <div className="absolute inset-x-0 bottom-0 pb-3 pl-3 pr-14">
        <MIDIPlayer ossUrl={item.url} suspended={suspended} />
      </div>
    </div>
  );
}

/** D5 #862 pdf 位（text 族 pdf / sheet_music 族 pdf）：原生 embed 全高容器
 * （放大语义归 D6，本票不绑定 embed 内容点击）。 */
function PdfSlide({ item }: SlideViewerContext) {
  return (
    <div className="h-full w-full bg-canvas-default" data-slot="slide-pdf">
      <embed src={item.url} type="application/pdf" title={item.fileName ?? ""} className="h-full w-full" />
    </div>
  );
}

/** 混排链逐 slide 类型角标的 family 键（content.attachmentPreview.familyLabel.*）。
    default = 编译期穷尽检查：新增 MediaSlideType 而漏配 family 时，never 赋值
    在此编译红，不再静默回落。 */
function slideFamilyKey(type: MediaSlideType): string {
  switch (type) {
    case "image":
      return "image";
    case "video":
      return "video";
    case "audio":
      return "audio";
    case "model3d":
      return "model3d";
    case "document":
      return "document";
    case "text_pdf":
      return "pdf";
    case "sheet_music_xml":
    case "sheet_music_midi":
    case "sheet_music_pdf":
      return "sheetMusic";
    default: {
      const _exhaustive: never = type;
      return _exhaustive;
    }
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
  /** D5 #862：查看器族 slide 的内容上下文（查看器下载/错误卡所需）。 */
  contentId?: number;
  /** D5 #862：查看器族 slide 的下载许可（与右栏下载行同源语义）。 */
  allowCopy?: boolean;
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
  contentId,
  allowCopy,
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

  /* D5 #862 查看器族位无加载探针：挂载即视为落定（首项驱动正文 reveal，
     不等浮层 2s 入场保险；onSettle 在布局侧有一次性守卫，重复触发无害）。 */
  useEffect(() => {
    if (item.type === "image" || item.type === "video") return;
    if (index === 0) onSettle("ready");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [item.type, item.id]);

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

  /* D5 #862 逐类型位分发：查看器族复用既有组件（预算/消毒/动态加载语义
     不变）；image 走既有图片分支（video 已于上方早返回）。default = 编译期
     穷尽检查：新增 MediaSlideType 而漏配渲染位时，never 赋值在此编译红。 */
  const viewerContext: SlideViewerContext = { item, contentId, allowCopy, suspended };
  switch (item.type) {
    case "image":
      return (
        <div className="relative h-full">
          <HoldCrossfadeImage
            canonicalSrc={coverRenderSrc(item.url) || item.url}
            holdSrc={index === 0 ? holdSrc : null}
            alt={alt}
            /* cursor-zoom-in 只给真实图片（主体点击开查看器）；合成自动文字封面
               被 isEnlargeableSlide 排除、主体点击无副作用，保持默认光标。 */
            imgClassName={item.autoCover ? "object-contain" : "cursor-zoom-in object-contain"}
            onSettle={settleIfFirst}
          />
        </div>
      );
    case "audio":
      return <AudioSlide {...viewerContext} />;
    case "model3d":
      return <Model3dSlide {...viewerContext} />;
    case "document":
      return <DocumentSlide {...viewerContext} />;
    case "sheet_music_xml":
      return <SheetMusicXmlSlide {...viewerContext} />;
    case "sheet_music_midi":
      return <SheetMusicMidiSlide {...viewerContext} />;
    case "sheet_music_pdf":
    case "text_pdf":
      return <PdfSlide {...viewerContext} />;
    default: {
      const _exhaustive: never = item.type;
      return _exhaustive;
    }
  }
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

/** 底部半透明指示点（D2：默认唯一常显件 = 触屏可发现入口）。D5 #862：当前位为
    播放条形态（audio/midi）时上移让位（PLAYER_BAR_YIELD_CLASS），不遮播放条。 */
function BottomDots({ media, index, raised }: { media: MediaGalleryItem[]; index: number; raised?: boolean }) {
  return (
    <div
      data-slot="media-paging-dots"
      className={cn(
        "absolute left-1/2 z-10 flex -translate-x-1/2 items-center gap-1.5 rounded-full bg-black/25 px-2 py-1 backdrop-blur transition-[bottom] duration-150",
        raised ? PLAYER_BAR_YIELD_CLASS : "bottom-3",
      )}
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

/** D5 #862 混排链逐 slide 类型小角标（i18n familyLabel；仅混排链显示）。
    当前位为播放条形态（audio/midi）时同样上移让位。 */
function TypeBadge({ labelKey, raised }: { labelKey: string; raised?: boolean }) {
  const t = useTranslations();
  return (
    <span
      data-slot="media-type-badge"
      className={cn(
        "absolute left-3 z-10 rounded-full bg-black/45 px-2.5 py-0.5 text-xs text-white backdrop-blur transition-[bottom] duration-150",
        raised ? PLAYER_BAR_YIELD_CLASS : "bottom-3",
      )}
    >
      {t(labelKey)}
    </span>
  );
}

/** D5 #862 查看器位「刷新预览」：签名 URL（300s）过期等加载失败的有限用户
 * 重试——重取详情刷新签名（上层回调），不做定时轮询。 */
function RefreshPreviewButton({ onClick }: { onClick: () => void }) {
  const t = useTranslations();
  return (
    <button
      type="button"
      data-slot="media-refresh-preview"
      aria-label={t("media.gallery.refreshPreview")}
      title={t("media.gallery.refreshPreview")}
      onClick={(event) => {
        event.stopPropagation();
        onClick();
      }}
      className="absolute bottom-3 right-3 z-10 flex h-9 w-9 items-center justify-center rounded-full bg-black/45 text-white backdrop-blur transition-colors hover:bg-black/60 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
    >
      <RefreshCw className="h-4 w-4" aria-hidden="true" />
    </button>
  );
}

/** D6 #863 右上放大钮（可放大位入口）：image = 主体点击或本钮；model3d/
 * text_pdf = 仅本钮（画布/embed 点击交原生交互）。与三件套同一显隐语义
 * （hover 与 focus-within 显现、触屏恒显保证可发现入口、44px 触控目标）；
 * 多项链中位于 N/M 徽标正下方（单项链无徽标，直接贴角）。
 * 点击 stopPropagation = 防御性兜底：按钮渲染在 detail-cover 的兄弟位，
 * 点击本就到不了主体点击语义（该路径当前不可达）；与 RefreshPreviewButton
 * 同款惯例。 */
function EnlargeButton({ belowBadge, onClick }: { belowBadge: boolean; onClick: (event: React.MouseEvent<HTMLButtonElement>) => void }) {
  const t = useTranslations();
  return (
    <button
      type="button"
      data-slot="media-enlarge-button"
      aria-label={t("media.gallery.enlarge")}
      title={t("media.gallery.enlarge")}
      onClick={onClick}
      className={cn(
        "absolute right-3 z-10 flex h-11 w-11 min-h-11 min-w-11 items-center justify-center rounded-full bg-black/35 text-white opacity-0 backdrop-blur transition-opacity duration-150",
        "group-hover/media:opacity-100 group-focus-within/media:opacity-100 [@media(hover:none)]:opacity-100",
        "hover:bg-black/50 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
        belowBadge ? "top-14" : "top-3",
      )}
    >
      <Maximize2 className="h-5 w-5" aria-hidden="true" />
    </button>
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
  /** D5 #862：查看器族 slide 的内容上下文（查看器下载/错误卡所需）。 */
  contentId?: number;
  /** D5 #862：查看器族 slide 的下载许可（与右栏下载行同源语义）。 */
  allowCopy?: boolean;
  /** D5 #862：签名过期等加载失败的有限重试（重取详情刷新签名 URL；重取后
   * 媒体链按附件 id 锚定保持当前位）。 */
  onMediaRefresh?: () => void;
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
  contentId,
  allowCopy,
  onMediaRefresh,
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

  /* D5 #862 重取详情（签名刷新）后媒体链重建：按附件 id 锚定保持当前位
     （顺序由后端稳定排序保证，锚定为防御性双保险）。恢复位同步写回
     anchorIndexRef：下方记录 effect 在同一提交内仍以旧 index 运行，锚定值
     不依赖 effect 执行顺序/状态落地时序。 */
  const anchorIdRef = useRef<number | null>(null);
  const anchorIndexRef = useRef(0);
  const prevMediaRef = useRef(media);
  useEffect(() => {
    if (prevMediaRef.current === media) return;
    prevMediaRef.current = media;
    const anchorId = anchorIdRef.current;
    if (anchorId != null) {
      const found = media.findIndex((item) => item.id === anchorId);
      const next = found >= 0 ? found : clampIndex(anchorIndexRef.current, media.length);
      anchorIndexRef.current = next;
      setIndex(next);
    }
  }, [media]);
  useEffect(() => {
    /* 记录当前位锚点：id 与 index 成对写同一 clamped 位（渲染侧 at 同源），
       避免链缩短时把越界裸 index 写进锚定兜底。 */
    const at = clampIndex(index, media.length);
    const item = media[at];
    if (item) {
      anchorIdRef.current = item.id;
      anchorIndexRef.current = at;
    }
  }, [media, index]);

  const at = clampIndex(index, media.length);
  const current = media[at];
  const showControls = media.length > 1;
  const isVideo = current?.type === "video";
  /* D6 #863：可放大序列 = image / model3d / text_pdf（overlay-media
     isEnlargeableSlide 谓词，spec §3.2 放大态行）；video/audio/document/
     乐谱族（含乐谱族 pdf）不进。稳定附件 id 映射左列序列，放大态内独立
     计数、边界 clamp（MediaViewer 只在传入序列内翻页）。混排链显示逐
     slide 类型角标；>9 项隐藏圆点。 */
  const viewerItems = media.filter(isEnlargeableSlide);
  const mixedFamilies = new Set(media.map((item) => slideFamilyKey(item.type))).size > 1;
  const showDots = media.length > 1 && media.length <= MAX_DOT_ITEMS;
  const isViewerSlide = current ? current.type !== "image" && current.type !== "video" : false;
  /* D5 #862：当前位为播放条形态（audio/midi）——底部圆点/类型角标上移让位。 */
  const playerBarSlide = current?.type === "audio" || current?.type === "sheet_music_midi";
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
    /* D6 #863：主体点击仅可放大的 image 位开查看器（真图/真封面双入口之一；
       合成自动文字封面经 autoCover 标记被谓词排除——主体点击无副作用不开
       查看器；查看器族位点击不绑定——3D 拖转、PDF/文档原生输入不被抢，
       3D/text-pdf 走右上放大钮）；查看器序 = 可放大序列内的位置（稳定附件
       id 映射）。 */
    if (!isEnlargeableSlide(current)) return;
    openViewerAt(current, event.currentTarget);
  }

  /* D6 #863：打开放大态——按当前 slide 的稳定附件 id 定位可放大序列内的
     位置（在 3D/pdf 位打开 → 当前项 = 该 3D/pdf，不是首图）；找不到 id
     （被排除项/链已变）显式 no-op，绝无兜底打开首项路径。 */
  function openViewerAt(item: MediaGalleryItem, trigger: HTMLElement | null) {
    const viewerAt = viewerItems.findIndex((candidate) => candidate.id === item.id);
    if (viewerAt < 0) return;
    viewerTriggerRef.current = trigger;
    setViewerIndex(viewerAt);
  }

  /* D6 #863 右上放大钮入口：触发焦点落在按钮自身（关闭恢复焦点用）；
     stopPropagation = 防御性兜底（按钮当前为 detail-cover 兄弟位，点击
     不会冒泡进主体点击语义，该路径当前不可达；与 RefreshPreviewButton
     同款惯例）。 */
  function handleEnlargeClick(event: React.MouseEvent<HTMLButtonElement>) {
    if (!current) return;
    event.stopPropagation();
    openViewerAt(current, event.currentTarget);
  }

  function handleViewerOpenChange(open: boolean) {
    if (open) return;
    /* D6：关闭返回最后所看项——左列轮播定位到关闭前查看器内的当前项
       （稳定附件 id 反查媒体链；换片是轮播语义，复位用户暂停意图）。 */
    const lastViewed = viewerIndex != null ? viewerItems[clampIndex(viewerIndex, viewerItems.length)] : undefined;
    setViewerIndex(null);
    if (lastViewed) {
      const mediaAt = media.findIndex((item) => item.id === lastViewed.id);
      if (mediaAt >= 0 && mediaAt !== clampIndex(index, media.length)) {
        userPausedRef.current = false;
        setIndex(mediaAt);
      }
    }
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
                contentId={contentId}
                allowCopy={allowCopy}
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
                {showDots && <BottomDots media={media} index={at} raised={playerBarSlide} />}
              </>
            )}
            {current && mixedFamilies && (
              <TypeBadge
                labelKey={`content.attachmentPreview.familyLabel.${slideFamilyKey(current.type)}`}
                raised={playerBarSlide}
              />
            )}
            {current && isViewerSlide && onMediaRefresh && (
              <RefreshPreviewButton onClick={onMediaRefresh} />
            )}
            {/* D6 #863：可放大位右上放大钮（image/model3d/text_pdf）；
                多项链中位于 N/M 徽标正下方。 */}
            {current && isEnlargeableSlide(current) && (
              <EnlargeButton belowBadge={showControls} onClick={handleEnlargeClick} />
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

      {viewerItems.length > 0 && (
        <MediaViewer
          items={viewerItems}
          index={viewerIndex ?? 0}
          open={viewerIndex !== null}
          onOpenChange={handleViewerOpenChange}
          onIndexChange={setViewerIndex}
          contentId={contentId}
          allowCopy={allowCopy}
        />
      )}
    </div>
  );
}
