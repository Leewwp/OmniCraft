"use client";

import { useEffect, useState } from "react";
import { selectMediaItems, type MediaGalleryItem } from "@/components/content/MediaGallery";
import { getCoverPlaceholder } from "@/lib/coverPlaceholder";
import { coverRenderSrc, prefetchCoverVariant } from "@/lib/overlay-motion";
import type { NormalizedContentDetailResponse } from "@/lib/content";

/**
 * 内容详情浮窗「竖屏集新版布局」（#397 R2）的媒体几何库。
 * 设计输入：docs/working/2026-09-06-overlay-rework-prototype-handoff.md §2/§7（胜者记录）。
 */

/** 横竖朝向判定边界（用户 2026-09-06 二次修订）：w/h ≥ 16/9 = 横图（恰 16:9 归横图）。 */
export const PORTRAIT_BOUNDARY_RATIO = 16 / 9;
/** 超高图阈值：与 MediaGallery ULTRA_TALL_RATIO 一致（h/w > 2 限高 + 内部滚动）。 */
export const ULTRA_TALL_RATIO = 2;
/** 媒体几何缺失时的防御性默认比例（与 MediaGallery DEFAULT_ASPECT_RATIO 一致）。 */
export const OVERLAY_DEFAULT_RATIO = 3 / 4;
/** 自动文字封面名义几何（getCoverPlaceholder 3:4 渐变字牌）。 */
const PLACEHOLDER_WIDTH = 300;
const PLACEHOLDER_HEIGHT = 400;
/** 封面尺寸实测保险：网络悬挂时按当前几何放行，不卡死浮窗。 */
const PROBE_GUARD_MS = 5000;

export function itemAspectRatio(item: MediaGalleryItem | undefined): number {
  if (item?.width && item.height && item.width > 0 && item.height > 0) {
    return item.width / item.height;
  }
  return OVERLAY_DEFAULT_RATIO;
}

export function isUltraTallItem(item: MediaGalleryItem | undefined): boolean {
  if (!item?.width || !item?.height || item.width <= 0 || item.height <= 0) return false;
  return item.height / item.width > ULTRA_TALL_RATIO;
}

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
 * 媒体链 + 尺寸实测：缺几何项（真实封面 cover_width/height=0）用 Image 预加载测
 * intrinsic 尺寸；实测完成前 ready=false（布局判定与入场转场等几何，避免横竖误判）；
 * 加载失败回退自动文字封面；5s 保险放行。detail 为 null（加载中）时不产出媒体。
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
    /* #409 F1 同源预热：媒体链首项的规范变体在数据落定瞬间预取并解码——
       入场落定后 holdSrc→真实媒体的切换不再撞冷缓存（展示 URL 按响应逐次签名，
       卡片封面预取与媒体集常为不同文件/不同串，落定切换必然是全新请求），
       预取与几何实测/入场转场并行，落定时大概率已解码命中。 */
    if (base.length > 0) prefetchCoverVariant(base[0].url);
    const pending = base.filter(
      (item) => (!item.width || !item.height) && item.url && !item.url.startsWith("data:"),
    );
    if (pending.length === 0) {
      setState({ media: base, ready: true });
      return;
    }
    setState({ media: base, ready: false });
    let cancelled = false;
    const patched = [...base];
    let remaining = pending.length;
    const settle = () => {
      remaining -= 1;
      if (remaining <= 0 && !cancelled) setState({ media: [...patched], ready: true });
    };
    const patch = (id: number, data: Partial<MediaGalleryItem>) => {
      const idx = patched.findIndex((item) => item.id === id);
      if (idx >= 0) patched[idx] = { ...patched[idx], ...data };
    };
    const guard = window.setTimeout(() => {
      if (!cancelled) setState({ media: [...patched], ready: true });
    }, PROBE_GUARD_MS);
    for (const item of pending) {
      const probe = new window.Image();
      probe.onload = () => {
        if (cancelled) return;
        patch(item.id, { width: probe.naturalWidth, height: probe.naturalHeight });
        settle();
      };
      probe.onerror = () => {
        if (cancelled) return;
        patch(item.id, {
          url: getCoverPlaceholder(detail.content?.content_type ?? "other", detail.content?.title),
          width: PLACEHOLDER_WIDTH,
          height: PLACEHOLDER_HEIGHT,
        });
        settle();
      };
      /* #409 F1：探针走规范变体（coverRenderSrc）——实测 intrinsic 与变体无关，
         但这一请求恰好预热浮窗首帧将渲染的同一 URL（卡片端已预取则命中缓存）。 */
      probe.src = coverRenderSrc(item.url) ?? item.url;
    }
    return () => {
      cancelled = true;
      window.clearTimeout(guard);
    };
  }, [detail]);

  return state;
}
