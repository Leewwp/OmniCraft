"use client";

import { useEffect, useState } from "react";
import { type MediaGalleryItem, type MediaSlideType } from "@/components/content/MediaGallery";
import { isPreviewBlockedByScan } from "@/lib/attachment-policy";
import { getCoverPlaceholder } from "@/lib/coverPlaceholder";
import { coverRenderSrc, prefetchCoverVariant } from "@/lib/overlay-motion";
import type { AttachmentData, NormalizedContentDetailResponse } from "@/lib/content";

/**
 * 详情浮层统一版式的全类型媒体几何库（D2 #859 起；D5 #862 扩为可预览媒体链；
 * E3 #955 封面恒为首屏 + 不可预览附件托底位）：媒体链构建、逐附件 slide
 * 分发、图片/视频类型分流尺寸探测与共享有界就绪预算。
 * 设计输入：docs/superpowers/specs/2026-10-09-omnicraft-detail-overlay-unification-design.md §9.3/§9.6；
 * docs/superpowers/specs/2026-11-07-omnicraft-overlay-xhs-parity-design.md（四项裁决 3）；
 * 历史：docs/working/2026-09-06-overlay-rework-prototype-handoff.md §2/§7（#397 R2 胜者记录）。
 */

/** 横竖朝向判定边界（用户 2026-09-06 二次修订）：w/h ≥ 16/9 = 横图（恰 16:9 归横图）。 */
export const PORTRAIT_BOUNDARY_RATIO = 16 / 9;
/** #753 长图阈值（单一共享常量，含 9:16 边界等号）：h/w ≥ 16/9 判定长图。
 * 移动端 = 初始顶部 3:4 折叠 + 就地展开；PC = 按真实比例缩窄居中完整显示
 * （旧 h/w > 2 + 70vh 内滚退役）。 */
export const LONG_IMAGE_RATIO = 16 / 9;
/** 媒体几何缺失时的防御性默认比例（与 MediaGallery DEFAULT_ASPECT_RATIO 一致）。 */
export const OVERLAY_DEFAULT_RATIO = 3 / 4;
/** 共享有界就绪预算（D2 #859）：未知尺寸探测的上限。必须短于浮层入场保险
 * （ContentDetailOverlay ENTRANCE_SAFETY_MS=2000）——保证 chainReady 在入场
 * 保险之前落定（超时按稳定 fallback 完成当前入场）；预算窗口之后才到达的
 * 迟到元数据不参与入场（几何按内容冻结，E1 #953 lib/overlay-geometry
 * contentFrozenRatio 消费首 slide），落定后按 240ms 平滑修正。 */
export const MEDIA_READY_BUDGET_MS = 1500;
/** 自动文字封面名义几何（getCoverPlaceholder 3:4 渐变字牌）。 */
const PLACEHOLDER_WIDTH = 300;
const PLACEHOLDER_HEIGHT = 400;
/** D5 #862 查看器族 slide 名义几何（无 intrinsic 宽高）：与占位图一致（3:4）。 */
export const SLIDE_NOMINAL_WIDTH = 300;
export const SLIDE_NOMINAL_HEIGHT = 400;

export function itemAspectRatio(item: MediaGalleryItem | undefined): number {
  if (item?.width && item.height && item.width > 0 && item.height > 0) {
    return item.width / item.height;
  }
  return OVERLAY_DEFAULT_RATIO;
}

export function isLongImageItem(item: MediaGalleryItem | undefined): boolean {
  if (item?.type !== "image") return false;
  if (!item?.width || !item.height || item.width <= 0 || item.height <= 0) return false;
  return item.height / item.width >= LONG_IMAGE_RATIO;
}

/** 旧名兼容（语义已并入长图判定）：供转场降级标记等既有调用点。 */
export const isUltraTallItem = isLongImageItem;

/** 朝向判定（handoff §2.1）：任一素材 w/h < 16/9 → 整集走新版布局（混合集一律新版）；
    全部缺几何的历史数据不判竖（防御，维持现设计）。 */
export function isPortraitMediaSet(items: MediaGalleryItem[]): boolean {
  const dimmed = items.filter((item) => item.width && item.height && item.width > 0 && item.height > 0);
  if (dimmed.length === 0) return false;
  return dimmed.some((item) => item.width! / item.height! < PORTRAIT_BOUNDARY_RATIO);
}

/** sheet_music 族逐附件分派（D5 #862）：file_type 优先、持久化 MIME 兜底；
 * mscz/mscx 与未知格式不产出可预览 slide（E3 #955 起由
 * overlayFallbackSlideFromAttachment 产出托底位）。 */
function sheetMusicSlideType(attachment: AttachmentData): MediaSlideType | null {
  const fileType = attachment.file_type ?? "";
  const mime = attachment.mime_type ?? "";
  if (fileType === "sheet_music_xml" || mime.includes("musicxml")) return "sheet_music_xml";
  if (fileType === "sheet_music_midi" || mime === "audio/midi") return "sheet_music_midi";
  if (fileType === "sheet_music_pdf" || mime === "application/pdf") return "sheet_music_pdf";
  return null;
}

/** text 族 pdf 判定与后端签名白名单（display_url.go）同口径：仅持久化 MIME
 * 精确为 application/pdf（正规化 media type，文件名不算数）；txt 留右栏下载行。 */
function persistedPdfMime(mimeType: string | undefined): boolean {
  return (mimeType ?? "").split(";")[0].trim().toLowerCase() === "application/pdf";
}

/**
 * 可预览媒体链逐附件分发（D5 #862，spec §9.6）——overlay 侧扩展点，
 * 不触碰 selectMediaItems 对独立详情页的既有契约：
 * - image/video 族沿用既有规则（oss_key 兜底、无前端扫描门），但不再限定
 *   内容类型（template/3d_print 等附件族的图/视附件同样进链参与混排）；
 * - audio/model3d/document 族：必须有签名 oss_url 且过扫描门（clean/
 *   not_required，沿用 isPreviewBlockedByScan 语义）；.mtl 不进；
 * - text 族仅持久化 MIME=application/pdf；mod 永不进（固定包裹占位）。
 */
export function overlaySlideFromAttachment(
  attachment: AttachmentData,
  videoPosterUrl?: string,
): MediaGalleryItem | null {
  const fileType = attachment.file_type ?? "";
  if (fileType === "image" || fileType === "video") {
    const url = attachment.oss_url || attachment.oss_key;
    if (!url) return null;
    return {
      id: attachment.id,
      url,
      type: fileType,
      width: attachment.width,
      height: attachment.height,
      posterUrl: fileType === "video" ? videoPosterUrl : undefined,
    };
  }
  if (!attachment.oss_url || isPreviewBlockedByScan(attachment.scan_status)) return null;
  const fileName = attachment.original_file_name || attachment.oss_key || "";
  const base = {
    id: attachment.id,
    url: attachment.oss_url,
    fileName,
    fileSize: attachment.file_size,
    width: SLIDE_NOMINAL_WIDTH,
    height: SLIDE_NOMINAL_HEIGHT,
  };
  switch (fileType) {
    case "audio":
      return { ...base, type: "audio" };
    case "model3d":
      if (fileName.toLowerCase().endsWith(".mtl")) return null;
      return { ...base, type: "model3d" };
    case "document":
      return { ...base, type: "document" };
    case "text":
      if (!persistedPdfMime(attachment.mime_type)) return null;
      return { ...base, type: "text_pdf" };
    default: {
      const sheetType = sheetMusicSlideType(attachment);
      return sheetType ? { ...base, type: sheetType } : null;
    }
  }
}

/**
 * 可放大 slide 谓词（D6 #863，spec §3.2 放大态行 / §9.6 末两段）：可放大
 * 序列仅三类 = 真实图片（image，含真内容封面 slide；不含 autoCover 标记的
 * 合成文字封面）、model3d、text 族 pdf（text_pdf）。video/audio/document/
 * 乐谱族（含 sheet_music_pdf——乐谱族 pdf 仍属乐谱位）与托底位
 * （fallback_download，属性卡非媒体）一律无放大入口；自动文字封面/音频
 * 背景封面/video poster 不算独立图片附件——后两者不是 slide 链成员（由
 * slide 分发层保证）；自动文字封面确会作为无可预览项时的唯一 fallback
 * image slide 进链（buildOverlayMedia 置 autoCover 标记），谓词层按标记排除。
 */
export function isEnlargeableSlide(item: MediaGalleryItem): boolean {
  if (item.type === "image" && item.autoCover) return false;
  return item.type === "image" || item.type === "model3d" || item.type === "text_pdf";
}

/**
 * 可预览媒体链（slide 序列）：按附件稳定顺序（后端 sort_order ASC NULLS LAST,
 * id ASC，0 有效）逐附件分派；前端不重排。顺序即轮播顺序。
 */
export function selectOverlaySlides(
  attachments: AttachmentData[],
  videoPosterUrl?: string,
): MediaGalleryItem[] {
  const slides: MediaGalleryItem[] = [];
  for (const attachment of attachments) {
    const slide = overlaySlideFromAttachment(attachment, videoPosterUrl);
    if (slide) slides.push(slide);
  }
  return slides;
}

/**
 * 不可预览附件的托底 slide（E3 #955）：mscz/txt/mod/压缩包等无浏览器渲染器
 * 的附件逐附件进链（翻页可逐个查看属性），任何内容打开舞台非空。资格 =
 * 过扫描门（clean/not_required，与可预览链同门；被拦截行留右栏扫描卡）
 * 且非 .mtl（纹理伴生文件不是独立内容页）。下载不依赖签名 oss_url——走
 * DownloadButton 既有 `/contents/{id}/download?attachment_id` 路径按需现签。
 * url = 舞台底图（有内容封面 = 封面图，无封面 = 空串 → 渲染层按 family
 * 占位图兜底）；几何 = 查看器族名义 3:4。
 */
export function overlayFallbackSlideFromAttachment(
  attachment: AttachmentData,
  coverUrl?: string,
): MediaGalleryItem | null {
  if (isPreviewBlockedByScan(attachment.scan_status)) return null;
  const fileName = attachment.original_file_name || attachment.oss_key || "";
  if (fileName.toLowerCase().endsWith(".mtl")) return null;
  return {
    id: attachment.id,
    url: coverUrl ?? "",
    type: "fallback_download",
    fileName,
    fileSize: attachment.file_size,
    width: SLIDE_NOMINAL_WIDTH,
    height: SLIDE_NOMINAL_HEIGHT,
    fallbackFamily: attachment.file_type ?? "",
  };
}

/** URL 归一化（剥 query/hash）：封面与图片附件「同一文件」判定——展示 URL
    逐响应签名轮换（Expires/Signature 不同），同文件比对只认路径。 */
function normalizedMediaUrl(url: string): string {
  return url.split(/[?#]/, 1)[0];
}

/**
 * 全类型媒体源链（E3 #955 封面恒为首屏 + 全类型舞台托底）：有封面 →
 * 封面 slide 0 + 附件序列（可预览优先、不可预览托底，按附件稳定顺序混排）；
 * 无封面 → 附件序列（维持可预览附件优先现状）→ 自动文字封面兜底
 * （getCoverPlaceholder 3:4 渐变字牌）。附件序列为空时任何路径必有 slide，
 * 绝不空舞台。封面与图片附件为同一文件（图片内容封面常 = 首图，签名参数
    轮换后路径相同）时去重——保留封面位，避免翻页第 2 张与第 1 张相同。
 * cover_width/cover_height 全库为 0：封面项由 useOverlayMedia 预加载实测
 * intrinsic 尺寸（缺尺寸图片探针管道）后参与几何推导（E1 contentFrozenRatio
 * 消费首 slide，封面比例即冻结比例）。
 */
export function buildOverlayMedia(detail: NormalizedContentDetailResponse): MediaGalleryItem[] {
  const content = detail.content;
  if (!content) return [];
  const contentType = content.content_type;
  const coverUrl = content.cover_image_url ?? "";
  const attachments = detail.attachments ?? [];
  const slides: MediaGalleryItem[] = [];
  for (const attachment of attachments) {
    const slide =
      overlaySlideFromAttachment(
        attachment,
        contentType === "video" ? coverUrl : undefined,
      ) ?? overlayFallbackSlideFromAttachment(attachment, coverUrl);
    if (slide) slides.push(slide);
  }
  if (coverUrl) {
    const coverSlide: MediaGalleryItem = {
      id: -(content.id * 100),
      url: coverUrl,
      type: "image",
      width: content.cover_width,
      height: content.cover_height,
    };
    const coverKey = normalizedMediaUrl(coverUrl);
    const deduped = slides.filter(
      (slide) => !(slide.type === "image" && normalizedMediaUrl(slide.url) === coverKey),
    );
    return [coverSlide, ...deduped];
  }
  if (slides.length > 0) return slides;
  return [
    {
      id: -(content.id * 100 + 1),
      url: getCoverPlaceholder(contentType ?? "other", content.title),
      type: "image",
      width: PLACEHOLDER_WIDTH,
      height: PLACEHOLDER_HEIGHT,
      /* D6 #863：合成文字封面不算独立图片附件（spec §3.2 article 行 = — /
         §9.6），autoCover 标记供 isEnlargeableSlide 排除放大入口。 */
      autoCover: true,
    },
  ];
}

/**
 * 媒体尺寸探针结果：ok=false 统一表示「拿不到可信几何」（加载失败/超时/非正
 * 尺寸），失败策略由调用方按类型决定——图片回退占位图（既有契约），视频保留
 * 真实 URL 走防御比例（D2 #859）。
 */
export interface MediaSizeProbe {
  ok: boolean;
  width?: number;
  height?: number;
}

/** 图片 intrinsic 实测（Image 探针；仅图片项使用，走规范变体预热同源）。 */
export function probeImageNaturalSize(url: string): Promise<MediaSizeProbe> {
  return new Promise((resolve) => {
    const probe = new window.Image();
    probe.onload = () => {
      resolve(
        probe.naturalWidth > 0 && probe.naturalHeight > 0
          ? { ok: true, width: probe.naturalWidth, height: probe.naturalHeight }
          : { ok: false },
      );
    };
    probe.onerror = () => resolve({ ok: false });
    probe.src = url;
  });
}

/** 视频尺寸实测（D2 #859）：loadedmetadata + videoWidth/videoHeight；失败/
 * 超时/非正尺寸一律 ok:false，绝不 reject、绝不产出坏几何。探针只加载元数据
 * （preload=metadata），使用原始视频 URL——视频地址不得交给图片优化器或
 * Image 探针。 */
export function probeVideoMetadataSize(
  url: string,
  budgetMs: number = MEDIA_READY_BUDGET_MS,
): Promise<MediaSizeProbe> {
  return new Promise((resolve) => {
    let settled = false;
    let guard: number | null = null;
    const probe = document.createElement("video");
    const finish = (result: MediaSizeProbe) => {
      if (settled) return;
      settled = true;
      probe.onloadedmetadata = null;
      probe.onerror = null;
      if (guard !== null) window.clearTimeout(guard);
      resolve(result);
    };
    probe.preload = "metadata";
    probe.onloadedmetadata = () => {
      finish(
        probe.videoWidth > 0 && probe.videoHeight > 0
          ? { ok: true, width: probe.videoWidth, height: probe.videoHeight }
          : { ok: false },
      );
    };
    probe.onerror = () => finish({ ok: false });
    guard = window.setTimeout(() => finish({ ok: false }), budgetMs);
    probe.src = url;
  });
}

/**
 * 媒体链 + 尺寸实测（D2 #859 类型分流）：缺几何的图片项走 Image 规范变体探针
 * （失败回退自动文字封面，既有契约）；缺几何的视频项走 video loadedmetadata
 * 实测（失败/超时保留真实视频 URL，不换占位图/SVG，防御比例下仍可播放可重试）；
 * 视频首项不做 prefetchCoverVariant 预热（图片优化器只认图片）。实测在共享有界
 * 预算（MEDIA_READY_BUDGET_MS）内结清：完成前 ready=false（布局判定与入场转场
 * 等几何，避免横竖误判），超时按稳定 fallback 放行。detail 为 null（加载中）
 * 时不产出媒体。
 */
export function useOverlayMedia(detail: NormalizedContentDetailResponse | null): {
  media: MediaGalleryItem[];
  ready: boolean;
} {
  const [state, setState] = useState<{ media: MediaGalleryItem[]; ready: boolean }>({
    media: [],
    ready: false,
  });

  useEffect(() => {
    if (!detail?.content) {
      setState({ media: [], ready: false });
      return;
    }
    const base = buildOverlayMedia(detail);
    /* #409 F1 同源预热（D2 收紧为仅图片首项）：媒体链首项为图片时在数据落定
       瞬间预取规范变体并解码——入场落定后 holdSrc→真实媒体的切换不撞冷缓存。
       视频首项绝不能进图片优化器（poster 由 <video poster> 自行加载）。 */
    if (base.length > 0 && base[0].type === "image") {
      prefetchCoverVariant(base[0].url);
    }
    const hasDimensions = (item: MediaGalleryItem) =>
      Boolean(item.width && item.height && item.width > 0 && item.height > 0);
    const pendingImages = base.filter(
      (item) =>
        item.type === "image" && !hasDimensions(item) && item.url && !item.url.startsWith("data:"),
    );
    const pendingVideos = base.filter(
      (item) => item.type === "video" && !hasDimensions(item) && item.url,
    );
    if (pendingImages.length === 0 && pendingVideos.length === 0) {
      setState({ media: base, ready: true });
      return;
    }
    setState({ media: base, ready: false });
    let cancelled = false;
    const patched = [...base];
    let remaining = pendingImages.length + pendingVideos.length;
    const settle = () => {
      remaining -= 1;
      if (remaining <= 0 && !cancelled) setState({ media: [...patched], ready: true });
    };
    const patch = (id: number, data: Partial<MediaGalleryItem>) => {
      const idx = patched.findIndex((item) => item.id === id);
      if (idx >= 0) patched[idx] = { ...patched[idx], ...data };
    };
    /* 共享有界就绪预算：预算内未结清的探针统一放行（超时按稳定 fallback 完成
       当前入场，迟到补丁由冻结窗口挡在动画外）。 */
    const guard = window.setTimeout(() => {
      if (!cancelled) setState({ media: [...patched], ready: true });
    }, MEDIA_READY_BUDGET_MS);
    for (const item of pendingImages) {
      /* #409 F1：探针走规范变体（coverRenderSrc）——实测 intrinsic 与变体无关，
         但这一请求恰好预热浮窗首帧将渲染的同一 URL（卡片端已预取则命中缓存）。 */
      void probeImageNaturalSize(coverRenderSrc(item.url) ?? item.url).then((result) => {
        if (cancelled) return;
        if (result.ok) {
          patch(item.id, { width: result.width, height: result.height });
        } else {
          patch(item.id, {
            url: getCoverPlaceholder(detail.content?.content_type ?? "other", detail.content?.title),
            width: PLACEHOLDER_WIDTH,
            height: PLACEHOLDER_HEIGHT,
          });
        }
        settle();
      });
    }
    for (const item of pendingVideos) {
      void probeVideoMetadataSize(item.url).then((result) => {
        if (cancelled) return;
        /* 失败/超时：保留真实视频 URL 与无尺寸态（防御比例），不改占位图。 */
        if (result.ok) patch(item.id, { width: result.width, height: result.height });
        settle();
      });
    }
    return () => {
      cancelled = true;
      window.clearTimeout(guard);
    };
  }, [detail]);

  return state;
}
