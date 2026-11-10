import type { MediaGalleryItem } from "@/components/content/MediaGallery";
import { itemAspectRatio } from "@/lib/overlay-media";

/**
 * 详情浮窗几何引擎（E1 #953，纯函数）：内容推导壳宽 + 按内容冻结 + 阅读宽
 * 舞台档。设计输入 = docs/superpowers/specs/2026-11-07-omnicraft-overlay-xhs-parity-design.md
 * （四项裁决 2/4 + 实测参数：高 92.9vh、宽 = 媒体列（=高×封面比）+ 互动列 440）
 * 与 prototypes/2026-11-03-overlay-xhs-parity/（12 条切换过渡规范第 11 条、
 * 1320 上限验证）。宽度布局动画只发生在真实值变化（档位切换/跨内容）时，
 * 同档内翻页宽度恒定——逐 slide 重算漂移路径已退役。
 */

/** 固定右列宽（互动列，小红书实测 440px）。 */
export const OVERLAY_RIGHT_COL_WIDTH = 440;
/** 壳宽上限（原型验证：1120 上限会让宽舞台无从体现，放宽到 1320）。 */
export const OVERLAY_SHELL_MAX_WIDTH = 1320;
/** 壳宽的视口留白（min-[960px]:w-[…,calc(100%-2rem)] 同口径，2rem）。 */
export const OVERLAY_SHELL_VIEWPORT_MARGIN = 32;
/** 媒体列最小宽度下限。 */
export const OVERLAY_PANE_MIN_WIDTH = 280;
/** 档位切换的宽度布局动画（SP-12 缓动口径，240ms）。 */
export const OVERLAY_GEOMETRY_TRANSITION = "width 240ms cubic-bezier(0.22,0.61,0.36,1)";

/** 舞台档位：visual = 比例定宽；reading = 宽舞台（取满可用宽）。 */
export type OverlayStageTier = "visual" | "reading";

/** 档位判定输入（几何推导经 Layer/布局侧消费，见 OverlayVariantLayout）。 */
export interface OverlayGeometrySpec {
  tier: OverlayStageTier;
  /** 媒体列宽（px）。 */
  paneWidth: number;
  /** 壳宽（px）= 媒体列 + 固定右列，受视口上限约束。 */
  shellWidth: number;
}

/**
 * 逐 slide 档位判定：文档族（document / text_pdf / 乐谱族 / model3d，对齐
 * 原型 READING 集与生产 MediaSlideType）= reading 宽舞台；image/video/audio
 * = visual 比例定宽。default = 编译期穷尽检查（slideFamilyKey 同款）。
 */
export function slideTierOf(item: MediaGalleryItem): OverlayStageTier {
  switch (item.type) {
    case "document":
    case "text_pdf":
    case "sheet_music_xml":
    case "sheet_music_midi":
    case "sheet_music_pdf":
    case "model3d":
      return "reading";
    case "image":
    case "video":
    case "audio":
      return "visual";
    default: {
      const _exhaustive: never = item.type;
      return _exhaustive;
    }
  }
}

/**
 * 内容冻结比例 = 媒体链首 slide 的 intrinsic 比例（itemAspectRatio 语义：
 * 缺尺寸回退 3/4 防御比例，查看器族为名义 3:4）。开窗即锁定，内容内翻页
 * 零重算；E3 封面恒首屏落地后首 slide 自然变为封面。
 */
export function contentFrozenRatio(media: MediaGalleryItem[]): number {
  return itemAspectRatio(media[0]);
}

/** 壳宽上限 = min(1320, 视口宽 − 2rem)。 */
export function overlayShellMaxWidth(viewportWidth: number): number {
  return Math.min(OVERLAY_SHELL_MAX_WIDTH, viewportWidth - OVERLAY_SHELL_VIEWPORT_MARGIN);
}

export interface OverlayGeometryInput {
  tier: OverlayStageTier;
  frozenRatio: number;
  /** 壳内容高（媒体列高，实测；测量未就绪时由布局侧用 CSS 同式估计）。 */
  shellHeight: number;
  viewportWidth: number;
}

/**
 * 宽度推导：visual 档媒体列 = 壳高 × 冻结比例（clamp 最小宽度/可用宽上限，
 * 横图自然收窄 letterbox）；reading 档 = 取满可用宽（受壳宽上限约束）。
 * 壳宽 = 媒体列 + 固定右列，同样受视口上限约束。
 */
export function computeOverlayGeometry(input: OverlayGeometryInput): OverlayGeometrySpec {
  const shellMax = overlayShellMaxWidth(input.viewportWidth);
  /* 可用宽（媒体列宽上限）：极窄视口下保底媒体列最小宽度。 */
  const avail = Math.max(shellMax - OVERLAY_RIGHT_COL_WIDTH, OVERLAY_PANE_MIN_WIDTH);
  const byHeight = input.shellHeight * input.frozenRatio;
  const paneWidth =
    input.tier === "reading"
      ? avail
      : Math.min(Math.max(byHeight, OVERLAY_PANE_MIN_WIDTH), avail);
  return {
    tier: input.tier,
    paneWidth: Math.round(paneWidth),
    shellWidth: Math.round(Math.min(paneWidth + OVERLAY_RIGHT_COL_WIDTH, shellMax)),
  };
}
