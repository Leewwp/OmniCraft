"use client";

import { useEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { useTranslations } from "next-intl";
import { AudioLines, Box, ChevronLeft, ChevronRight, FileText, Maximize2, Music, Package, RefreshCw } from "lucide-react";
import { cn } from "@/lib/utils";
import { coverRenderSrc } from "@/lib/overlay-motion";
import { HoldCrossfadeImage } from "@/components/content/HoldCrossfadeImage";
import { MediaViewer } from "@/components/content/MediaViewer";
import { useAuthGate } from "@/components/auth/AuthGateProvider";
import { DownloadButton } from "@/components/content/DownloadButton";
import { SlideViewer, asViewerSlide } from "@/components/content/SlideViewer";
import { MIDIPlayer, OSMDRenderer } from "@/components/content/SheetMusicViewer";
import type { MediaGalleryItem, MediaSlideType } from "@/components/content/MediaGallery";
import { isEnlargeableSlide, isLongImageItem } from "@/lib/overlay-media";
import {
  OVERLAY_GEOMETRY_TRANSITION,
  OVERLAY_RIGHT_COL_WIDTH,
  computeOverlayGeometry,
  contentFrozenRatio,
  slideTierOf,
  type OverlayGeometrySpec,
} from "@/lib/overlay-geometry";
import { OVERLAY_MOTION } from "@/lib/overlay-motion";
import { attemptPlay, clampIndex, toggleVideoSlideClick } from "@/lib/media-interaction";
import { getCoverPlaceholder } from "@/lib/coverPlaceholder";

/**
 * 内容详情浮窗「统一版式」（D1 #858，桌面 ≥960 全类型唯一版式）：左媒体列
 * 贴边满幅、右栏固定宽独立滚动（data-slot="layer-scroller"，滚动条贴面板
 * 右缘全高贯通）；控件 = 悬浮半透明箭头 + 底部指示点 + 右上角标，图片主体
 * 点击进查看器。
 *
 * 媒体列直贴面板（宿主容器桌面零内边距，不再用负 margin 抵消 overlay-scroller
 * 的 px/pt/pb）；loading/错误态媒体链为空时媒体列保持稳定黑底占位（双栏外壳
 * 不塌缩、不回退全宽单列）。
 *
 * E1 #953 几何引擎（lib/overlay-geometry 纯函数 + 本组件接线）：
 * - 壳宽内容推导：壳宽 = 媒体列（= 壳高 × 内容冻结比例）+ 固定右列 440，
 *   上限 min(1320, 视口宽 − 2rem)；几何规格经 onGeometrySpec 上浮到浮层壳层
 *   驱动 dialog 宽（同程 240ms 过渡）；
 * - 按内容冻结：冻结比例 = 媒体链首 slide intrinsic（E3 封面恒首屏后自然
 *   变为封面），内容内翻页零几何变化（跨比例图在同一媒体列 letterbox）；
 * - 阅读宽舞台档：文档族 slide（document/text_pdf/乐谱族/model3d；pane
 *   data-tier=reading）取满可用宽；影像+文档混合内容翻页时一次 240ms 档位
 *   切换，同档内翻页宽度恒定；
 * - 首帧宽度即终值（px，无百分比兜底→实测的跳变舞蹈）；测量落地帧关闭
 *   过渡，其后真实值变化才走 240ms 宽度过渡。
 *
 * D2 #859 媒体列几何与视频行为（spec §3.1/§3.3/§9.3）：
 * - 视频全套小红书行为：活跃可见项 muted+playsInline 自动播放；主体点击单次
 *   切换播放/暂停（原生 controls 条 44px 内点击不切播、不翻页、不进查看器）；
 *   play() 被浏览器策略拒绝时保留 controls 手动播放入口；视频不再进 MediaViewer
 *   （全屏用控制条原生按钮）。
 * - 两段式根除：共享有界就绪预算（lib MEDIA_READY_BUDGET_MS）先于入场保险
 *   结清 chainReady；contain 完整呈现允许黑边。首 slide 迟到元数据回填在
 *   入场落定后按新冻结比例一次 240ms 平滑修正（宽度仅依赖首 slide，后续位
 *   元数据不参与几何）。
 * - 挂起矩阵：非顶层（压层）、上层查看器、登录浮窗打开时停止隐藏媒体；
 *   恢复仅当前项；任何时刻至多一段声音（同刻只挂载当前 slide）。
 * - 三件套可用性：仅多项显示；边界 clamp 不循环；hover 与 focus-within 均可见；
 *   触控目标 44px；键盘 ←/→ 只作用于顶层活动媒体区，不劫持输入框/评论编辑器/
 *   播放器控件/上层查看器/登录浮窗；切位按钮点击不触发放大。
 *
 * D5 #862 可预览媒体链与逐类型轮播位（spec §9.6/§3.2）+ E3 #955 封面恒为
 * 首屏与全类型托底：
 * - 媒体链（lib/overlay-media buildOverlayMedia）：有封面 → 封面 slide 0 +
 *   附件序列；无封面 → 附件序列 → 自动文字封面兜底。附件序列 = 逐附件
 *   slide（可预览族 audio/model3d/document/text-pdf/sheet_music 与
 *   image/video 混排；不可预览族 mscz/txt/mod 等出 fallback_download
 *   托底位）——任何 default 态内容舞台非空。
 * - 逐类型位：audio = 占位图为底 + 底部 AudioPlayer 条；model3d/document =
 *   对应查看器（懒分块 + 预算/消毒语义不变）；pdf = 原生 embed；乐谱 =
 *   OSMD 横滚 / midi 回放条（箭头悬浮层不拦截谱内滚动）；fallback_download
 *   = 封面/占位图为底 + 文件名/大小 + 下载钮属性卡。audio/midi 播放条
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
 * 媒体几何比例源 = 媒体链首 slide intrinsic（内容冻结，lib/overlay-geometry
 * contentFrozenRatio）。
 */

/* 视频 controls 条高度 / 索引钳制 / 自动播放重试 / 视频点击切播：共享原语
   收口于 lib/media-interaction（C1 #920）。 */

/** 根区实测（ResizeObserver）：高度 = 壳内容高（媒体列高的几何基准；宽度
    不再参与推导——壳宽由内容反推，根区宽随之确定，不存在反向依赖）。 */
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

/** 壳高 CSS 基准（dialog min-[960px]:h-[min(92dvh,900px)]）的 JS 镜像：
    根区实测未就绪时的首帧估计（桌面无 URL 栏，dvh ≈ vh，估计与实测同值
    量级，仅亚像素差）；测量落地后以实测高为准。 */
function estimateShellHeight(): number {
  if (typeof window === "undefined") return 0;
  return Math.min(window.innerHeight * 0.92, 900);
}

/** 视口宽订阅（壳宽上限 = min(1320, 视口宽 − 2rem) 的基准；resize 时重算）。 */
function useViewportWidth(): number {
  const [width, setWidth] = useState(() => (typeof window === "undefined" ? 0 : window.innerWidth));
  useEffect(() => {
    const handleResize = () => setWidth(window.innerWidth);
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);
  return width;
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

/** D5 #862 audio 位：audio 专属占位图为底 + 底部悬浮紧凑 AudioPlayer 条
 * （封面合成留宿主；查看器装配经 SlideViewer 单一来源）。起播权在用户；
 * 切位卸载停播释放、挂起暂停；右下角预留刷新钮命中区。 */
function AudioSlide({ item, contentId, allowCopy, suspended }: SlideViewerContext) {
  const slide = asViewerSlide(item);
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
        {slide && (
          <SlideViewer item={slide} contentId={contentId} allowCopy={allowCopy} suspended={suspended} />
        )}
      </div>
    </div>
  );
}

/** D5 #862 model3d 位：ModelViewer 画布（拖转/滚轮缩放/四机位已有；画布
 * 点击不绑定——轮播切页依显式箭头）。装配（懒加载/预算映射）经 SlideViewer。 */
function Model3dSlide({ item, contentId, allowCopy }: SlideViewerContext) {
  const slide = asViewerSlide(item);
  return (
    <div className="h-full w-full overflow-y-auto p-3" data-slot="slide-model3d">
      {slide && <SlideViewer item={slide} contentId={contentId} allowCopy={allowCopy} />}
    </div>
  );
}

/** D5 #862 document 位：DocumentViewer 全高容器（docx 消毒 HTML / xlsx·csv
 * 网格 / 超预算降级卡由查看器内部维持；装配经 SlideViewer）。 */
function DocumentSlide({ item, contentId, allowCopy }: SlideViewerContext) {
  const slide = asViewerSlide(item);
  return (
    <div className="h-full w-full overflow-y-auto p-3" data-slot="slide-document">
      {slide && <SlideViewer item={slide} contentId={contentId} allowCopy={allowCopy} />}
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

/** E3 #955 托底位（fallback_download）：不可预览附件的属性卡——底图 = 有
 *  内容封面用封面图（压暗衬底）/ 无封面按附件族占位图；前景 = 族图标 +
 *  文件名/大小 + 预览不可用提示 + 下载钮（下载许可 = allowCopy 与右栏下载
 *  行同源语义，动作复用 DownloadButton 按需现签路径）。任何内容打开舞台
 *  非空的兜底形态。 */
function FallbackSlide({ item, contentId, allowCopy }: SlideViewerContext) {
  const t = useTranslations();
  const family = item.fallbackFamily ?? "";
  const backdrop = item.url
    ? coverRenderSrc(item.url) || item.url
    : getCoverPlaceholder(fallbackPlaceholderType(family));
  const Icon = fallbackFamilyIcon(family);
  const size =
    item.fileSize == null ? null : item.fileSize >= 1024 * 1024
      ? `${(item.fileSize / 1024 / 1024).toFixed(2)} MB`
      : `${(item.fileSize / 1024).toFixed(1)} KB`;
  return (
    <div className="relative h-full w-full bg-black" data-slot="slide-fallback">
      {/* eslint-disable-next-line @next/next/no-img-element -- 装饰底图（SVG/data: 占位直通，不走图片优化器） */}
      <img
        src={backdrop}
        alt=""
        aria-hidden="true"
        className="absolute inset-0 h-full w-full object-cover opacity-40"
      />
      <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 p-6 text-center">
        <span className="flex h-12 w-12 items-center justify-center rounded-full bg-black/45 text-white backdrop-blur">
          <Icon className="h-6 w-6" aria-hidden="true" />
        </span>
        <div className="max-w-full rounded-lg bg-black/45 px-4 py-3 backdrop-blur">
          <p className="truncate text-sm font-medium text-white" title={item.fileName ?? undefined}>
            {item.fileName || t("content.attachmentUnknownType")}
          </p>
          {size && <p className="mt-0.5 text-xs tabular-nums text-white/70">{size}</p>}
          <p className="mt-1.5 text-xs text-white/70">{t("content.attachmentPreview.fallbackHint")}</p>
          {allowCopy && contentId != null && (
            <div className="mt-3 flex justify-center">
              <DownloadButton
                contentId={contentId}
                attachmentId={item.id}
                contentType={family}
                size="sm"
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

/** 托底位附件族 → 类型占位图（无封面底图兜底；getCoverPlaceholder 键位）。 */
function fallbackPlaceholderType(family: string): string {
  if (family.startsWith("sheet_music")) return "sheet_music";
  return family || "other";
}

/** 托底位附件族 → 前景图标。 */
function fallbackFamilyIcon(family: string): typeof FileText {
  if (family.startsWith("sheet_music")) return Music;
  if (family === "mod") return Package;
  if (family === "model3d") return Box;
  if (family === "audio") return AudioLines;
  return FileText;
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
    case "fallback_download":
      return "fallback";
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
     不变）；image 走既有图片分支（video 已于上方早返回）；E3 #955 不可
     预览附件走托底位（FallbackSlide 属性卡）。default = 编译期穷尽检查：
     新增 MediaSlideType 而漏配渲染位时，never 赋值在此编译红。 */
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
    case "fallback_download":
      return <FallbackSlide {...viewerContext} />;
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
      data-slot="media-arrow"
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
  /** E1 #953 几何规格上浮：桌面媒体链就绪且根区实测后，把推导出的档位/媒体
      列宽/壳宽报给浮层壳层（驱动 dialog 宽）；值变化才触发（翻页同档零上报）。 */
  onGeometrySpec?: (spec: OverlayGeometrySpec) => void;
  /** #409 F1 几何稳定门：根区完成首次实测（ResizeObserver 回报）后回调一次，
      浮层据此放行入场转场（测量在几何稳定后进行）。 */
  onPaneMeasured?: () => void;
  /** E2 #954 关窗回源令牌：递增时舞台瞬时回源（index→0 + 档位/宽度无过渡，
      供关窗 morphBack 在稳定几何上测量；已回源则幂等）。 */
  exitResetToken?: number;
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
  onGeometrySpec,
  onPaneMeasured,
  exitResetToken = 0,
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
  const viewportWidth = useViewportWidth();
  const firstSettledRef = useRef(false);
  const measuredOnceRef = useRef(false);
  const viewerTriggerRef = useRef<HTMLElement | null>(null);
  const videoRef = useRef<HTMLVideoElement | null>(null);
  /* E2 #954 淡出层独立 video ref：与当前位 videoRef 隔离，切位停旧播新不受
     共享 ref 覆盖。 */
  const exitVideoRef = useRef<HTMLVideoElement | null>(null);
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
  /* 托底位（fallback_download）无签名 URL 可刷新（下载按需现签）——不提供
     「刷新预览」钮。 */
  const isViewerSlide = current
    ? current.type !== "image" && current.type !== "video" && current.type !== "fallback_download"
    : false;
  /* D5 #862：当前位为播放条形态（audio/midi）——底部圆点/类型角标上移让位。 */
  const playerBarSlide = current?.type === "audio" || current?.type === "sheet_music_midi";

  /* E1 #953 几何推导：冻结比例 = 首 slide intrinsic（空链回退防御比例，双栏
     外壳几何稳定）；档位 = 当前 slide 类型（同档内翻页宽度恒定，档位切换一次
     240ms）；壳高取根区实测（未就绪用 CSS 同式估计，首帧宽度即终值口径）。 */
  const frozenRatio = contentFrozenRatio(media);
  const tier = current ? slideTierOf(current) : "visual";
  const measured = area != null && area.h > 0;
  const { paneWidth, shellWidth } = computeOverlayGeometry({
    tier,
    frozenRatio,
    shellHeight: measured ? area.h : estimateShellHeight(),
    viewportWidth,
  });
  /* 测量落地帧关闭过渡（估计→实测的亚像素差不产生入场动画），其后真实值
     变化（档位切换/跨内容）才走 240ms 宽度过渡。 */
  const [geometrySettled, setGeometrySettled] = useState(false);
  useEffect(() => {
    if (!measured || geometrySettled) return;
    const handle = requestAnimationFrame(() => setGeometrySettled(true));
    return () => cancelAnimationFrame(handle);
  }, [measured, geometrySettled]);
  const tall = isLongImageItem(current);

  /* 几何规格上浮（仅顶层活动层 + 媒体链就绪 + 实测有效）：驱动浮层壳层
     dialog 宽。依赖为原始值——同档翻页/重复渲染零上报，档位切换/跨内容/
     首项迟到回填才上报一次。 */
  const geometryMeasured = measured && media.length > 0;
  useEffect(() => {
    if (!active || !geometryMeasured) return;
    onGeometrySpec?.({ tier, paneWidth, shellWidth });
  }, [active, geometryMeasured, tier, paneWidth, shellWidth, onGeometrySpec]);

  /* ── E2 #954 舞台 slide 双层交叉淡出（规范第 1/2 条） ─────────────────────
     翻页时旧 slide 保留为淡出层（180ms 真交叉淡出，新层淡入同时旧层淡出），
     240ms 后移除；快速连按时已在淡出的层被最新切换立即接管替换（发号牌制：
     仅最新切换持有旧层清理权，任意时刻舞台至多 2 层 = 1 新 + 1 淡出中）。 */
  const [exitSlide, setExitSlide] = useState<{ item: MediaGalleryItem; index: number } | null>(null);
  const exitOwnerRef = useRef(0);
  const prevAtRef = useRef(clampIndex(0, media.length));
  useEffect(() => {
    const prevAt = prevAtRef.current;
    prevAtRef.current = at;
    if (prevAt === at) return;
    const prevItem = media[prevAt];
    if (!prevItem) {
      setExitSlide(null);
      return;
    }
    setExitSlide({ item: prevItem, index: prevAt });
    const myToken = ++exitOwnerRef.current;
    const timer = window.setTimeout(() => {
      /* 发号牌：已有更新的切换接管清理权时本定时器作废，不误杀新淡出层。 */
      if (exitOwnerRef.current !== myToken) return;
      setExitSlide(null);
    }, OVERLAY_MOTION.slideExitRemoveMs);
    return () => window.clearTimeout(timer);
  }, [at, media]);

  /* ── E2 #954 关窗回源令牌 ──────────────────────────────────────────────────
     关窗开始时递增：舞台瞬时回源（index→0、交叉淡回封面态、档位/宽度无过渡
     ——过渡中的盒尺寸会让 morphBack 锚点/缩放测量漂移、落点对不齐卡片），
     并按 slide 0 档位立即重报几何规格（关窗期 active=false，常规上报门已关）。 */
  const [exitInstantWidth, setExitInstantWidth] = useState(false);
  const exitTokenRef = useRef(0);
  useEffect(() => {
    if (exitResetToken === 0 || exitTokenRef.current === exitResetToken) return;
    exitTokenRef.current = exitResetToken;
    setExitInstantWidth(true);
    setIndex(0);
    const firstTier = media.length > 0 ? slideTierOf(media[0]) : "visual";
    onGeometrySpec?.(
      computeOverlayGeometry({
        tier: firstTier,
        frozenRatio,
        shellHeight: measured ? (area?.h ?? estimateShellHeight()) : estimateShellHeight(),
        viewportWidth,
      }),
    );
    const handle = requestAnimationFrame(() =>
      requestAnimationFrame(() => setExitInstantWidth(false)),
    );
    return () => cancelAnimationFrame(handle);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [exitResetToken]);

  /* D2 挂起矩阵：非顶层（压入下一层/被压）、上层查看器、登录浮窗任一成立即
     停止隐藏媒体；恢复时仅当前 slide（唯一挂载者）续播（用户主动暂停不覆盖）。 */
  const suspended = !active || viewerIndex !== null || isGateOpen;

  const settleFirstMedia = (state: "ready" | "error") => {
    if (firstSettledRef.current || !onFirstMediaSettled) return;
    firstSettledRef.current = true;
    onFirstMediaSettled(state);
  };

  /* 点击媒体：图片进入 MediaViewer 看大图（关闭后焦点还给触发媒体区，AC4）；
     视频主体 = 单次切换播放/暂停——共享处理函数收口于 lib/media-interaction
     （C1 #920：controls 条豁免/原生全屏豁免/preventDefault 防双触发/记账顺序
     与画廊同源）。用户主动暂停的意图被记录，恢复激活时不自动续播。 */
  function handleCoverClick(event: React.MouseEvent<HTMLDivElement>) {
    if (!current) return;
    if (isVideo) {
      toggleVideoSlideClick({
        event,
        getVideo: () => videoRef.current,
        onUserPausedChange: (paused) => {
          userPausedRef.current = paused;
        },
      });
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
      {/* 媒体列（E1 #953）：宽 = 壳高 × 内容冻结比例（visual 档）/ 取满可用宽
          （reading 档 data-tier），黑底满幅贴边；同档翻页宽度恒定，档位切换一次
          240ms 过渡。媒体链为空（loading/错误态）保持稳定黑底占位——双栏外壳
          不塌缩。首帧宽度即终值（px），测量落地帧无过渡；E2 #954 关窗回源帧
          同样无过渡（exitInstantWidth，防 morphBack 测量漂移）。 */}
      <div
        data-slot="variant-media-pane"
        data-tier={tier}
        className="group/media relative h-full shrink-0 overflow-hidden bg-black"
        style={{
          width: `${paneWidth}px`,
          transition:
            geometrySettled && !exitInstantWidth ? OVERLAY_GEOMETRY_TRANSITION : "none",
        }}
      >
        {current && (
          <>
            {/* 当前层（R3 锚点契约）：不含翻页控件；#753 取消内部竖向滚动（长图
                随列宽缩窄居中完整显示）。E2 #954：key 按 slide id——翻页时新层
                重挂载触发 180ms 淡入（与旧层淡出构成真交叉淡出）；absolute 舞台
                分层不改变几何（宽度在 pane 上，E1 冻结）。data-ultra-tall：长图
                当前项标记——浮层转场据此退化居中缩淡（C2，卡片两档裁切 vs 面板
                contain 取景无法统一）。空链（错误态）不渲染锚点：浮层转场维持
                居中缩淡降级。 */}
            <div
              key={current.id}
              data-slot="detail-cover"
              data-ultra-tall={tall ? "true" : undefined}
              className="absolute inset-0 overflow-hidden"
              onClick={handleCoverClick}
              style={
                exitSlide
                  ? {
                      animation: `overlay-slide-fade-in ${OVERLAY_MOTION.slideCrossfadeMs}ms ease both`,
                    }
                  : undefined
              }
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
            {/* E2 #954 淡出层（规范第 1/2 条）：旧 slide 覆盖其上 180ms 淡出，
                240ms 后由发号牌定时器移除；挂起态（suspended 硬真）停播防双声。 */}
            {exitSlide && (
              <div
                data-slot="slide-fade-out"
                aria-hidden="true"
                className="pointer-events-none absolute inset-0 overflow-hidden"
                style={{
                  animation: `overlay-slide-fade-out ${OVERLAY_MOTION.slideCrossfadeMs}ms ease both`,
                }}
              >
                <MediaSlide
                  item={exitSlide.item}
                  index={exitSlide.index}
                  total={media.length}
                  onSettle={settleFirstMedia}
                  videoRef={exitVideoRef}
                  suspended
                  userPausedRef={userPausedRef}
                  contentId={contentId}
                  allowCopy={allowCopy}
                />
              </div>
            )}
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

      {/* 右文字列（E1 #953 固定宽 440，小红书实测互动列宽）：浮层内唯一滚动
          容器（滚动条贴面板右缘全高贯通）；× 悬浮面板右上覆盖内容，不留整行
          避让空带（首块右上局部避让由 ContentDetail creatorFirst 头部承担）。 */}
      <div
        data-slot="layer-scroller"
        className="min-h-0 shrink-0 overflow-y-auto overscroll-contain bg-card px-5 pb-4 pt-4"
        style={{ width: OVERLAY_RIGHT_COL_WIDTH }}
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
