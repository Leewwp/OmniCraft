"use client";

import { useEffect, useState } from "react";
import { selectMediaItems, type MediaGalleryItem } from "@/components/content/MediaGallery";
import { getCoverPlaceholder } from "@/lib/coverPlaceholder";
import { coverRenderSrc, prefetchCoverVariant } from "@/lib/overlay-motion";
import type { NormalizedContentDetailResponse } from "@/lib/content";

/**
 * 详情浮层统一版式的全类型媒体几何库（D2 #859 起）：媒体链构建、图片/视频
 * 类型分流尺寸探测与共享有界就绪预算。
 * 设计输入：docs/superpowers/specs/2026-10-09-omnicraft-detail-overlay-unification-design.md §9.3；
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
 * 迟到元数据由统一版式冻结窗口（OverlayVariantLayout freezeGeometry 锁比例）
 * 挡在入场动画之外，落定后随切位/回填平滑调整。 */
export const MEDIA_READY_BUDGET_MS = 1500;
/** 自动文字封面名义几何（getCoverPlaceholder 3:4 渐变字牌）。 */
const PLACEHOLDER_WIDTH = 300;
const PLACEHOLDER_HEIGHT = 400;

export function itemAspectRatio(item: MediaGalleryItem | undefined): number {
  if (item?.width && item.height && item.width > 0 && item.height > 0) {
    return item.width / item.height;
  }
  return OVERLAY_DEFAULT_RATIO;
}

export function isLongImageItem(item: MediaGalleryItem | undefined): boolean {
  if (item?.type === "video") return false;
  if (!item?.width || !item?.height || item.width <= 0 || item.height <= 0) return false;
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

/**
 * 全类型媒体源链（胜者记录 §7）：真实媒体集 → 内容封面 → 自动文字封面
 * （getCoverPlaceholder 3:4 渐变字牌）。cover_width/cover_height 全库为 0，
 * 封面项由 useOverlayMedia 预加载实测 intrinsic 尺寸后再判朝向（横封面 ≥16:9
 * 维持现设计）。
 */
export function buildOverlayMedia(detail: NormalizedContentDetailResponse): MediaGalleryItem[] {
  const content = detail.content;
  if (!content) return [];
  const contentType = content.content_type;
  const { media } = selectMediaItems(
    detail.attachments ?? [],
    contentType,
    contentType === "video" ? content.cover_image_url : undefined,
  );
  if (media.length > 0) return media;
  if (content.cover_image_url) {
    return [
      {
        id: -(content.id * 100),
        url: content.cover_image_url,
        type: "image",
        width: content.cover_width,
        height: content.cover_height,
      },
    ];
  }
  return [
    {
      id: -(content.id * 100 + 1),
      url: getCoverPlaceholder(contentType ?? "other", content.title),
      type: "image",
      width: PLACEHOLDER_WIDTH,
      height: PLACEHOLDER_HEIGHT,
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
