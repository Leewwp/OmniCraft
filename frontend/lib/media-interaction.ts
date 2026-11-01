/**
 * 媒体交互共享原语（C1 #920）：轮播位（OverlayVariantLayout）、放大态
 * （MediaViewer）、媒体画廊（MediaGallery）三宿主此前各自复制的微工具与
 * 「点击媒体体 = 播放/暂停」规则在此收口为单一来源——行为修复一次生效于
 * 全部宿主。模块保持纯函数形态（无组件依赖、无 hooks），调用方均为客户端
 * 组件；非视频尾巴（图片进查看器等宿主语义）不进共享面。
 */
import type { MouseEvent } from "react";

/** 视频 controls 条近似高度：点击该区域内属于原生控件语义（不切播、不翻页、
 *  不进查看器），宿主容器几何判定的共享常量。 */
export const VIDEO_CONTROLS_STRIP = 44;

/** 触摸滑动翻页位移阈值（px）（画廊滑动翻页 / 放大态触摸翻页共用）。 */
export const SWIPE_THRESHOLD = 40;

/** 序列索引钳制：非有限值/空序列归 0，负数归首、越界归末（不循环）。 */
export function clampIndex(value: number, length: number): number {
  if (!Number.isFinite(value) || length <= 0) return 0;
  return Math.min(Math.max(Math.floor(value), 0), length - 1);
}

/** 浏览器策略可能拒绝 play()（jsdom 返回 undefined 同样防御）：失败静默，
 *  controls 手动播放入口保留。 */
export function attemptPlay(video: HTMLVideoElement): void {
  const attempt = video.play() as unknown as Promise<void> | undefined;
  if (attempt && typeof attempt.catch === "function") {
    attempt.catch(() => {
      /* 自动播放被拒：用户可用 controls 手动播放。 */
    });
  }
}

export interface VideoSlideClickOptions {
  /** 宿主媒体容器上的点击事件（clientY 与 currentTarget 矩形用于 controls 条带判定）。 */
  event: MouseEvent<HTMLElement>;
  /** 视频元素获取：轮播位单 ref（videoRef.current）或画廊按附件 id 查 Map。 */
  getVideo: () => HTMLVideoElement | null | undefined;
  /** 可选用户暂停记账（仅轮播位挂起矩阵使用）：先记账、后播放/暂停（既有顺序）。 */
  onUserPausedChange?: (paused: boolean) => void;
}

/**
 * 视频主体点击 = 单次切换播放/暂停（三宿主共享唯一定义）：
 * - controls 条（容器底部 VIDEO_CONTROLS_STRIP 内）点击属原生控件语义 →
 *   不消费（返回 false，宿主不得叠加任何 JS 动作）；
 * - 原生全屏内点击交还原生控件（Blink 全屏路径不理会 defaultPrevented）→
 *   不消费；
 * - 其余主体点击 preventDefault（WebKit 显示 controls 时对主体 click 有默认
 *   togglePlayState，不抑制会与 JS 切播双触发互相抵消）；
 * - 播放前记账「非用户暂停」、暂停前记账「用户暂停」——顺序不可换，且该
 *   顺序只在这一份定义里。
 */
export function toggleVideoSlideClick({ event, getVideo, onUserPausedChange }: VideoSlideClickOptions): boolean {
  const rect = event.currentTarget.getBoundingClientRect();
  if (rect.height > 0 && event.clientY > rect.bottom - VIDEO_CONTROLS_STRIP) return false;
  if (document.fullscreenElement) return false;
  event.preventDefault();
  const video = getVideo();
  if (!video) return true;
  if (video.paused) {
    onUserPausedChange?.(false);
    attemptPlay(video);
  } else {
    onUserPausedChange?.(true);
    video.pause();
  }
  return true;
}
