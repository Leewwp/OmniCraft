/**
 * 内容详情浮层共享元素转场核心（#67 原型 §5 契约 / #64 决策 7-10）。
 *
 * FLIP 几何为可靠核心，View Transition API 是渐进增强；路径选择、测量门、
 * 时序与缓动常量全部在此收敛，组件只负责把这些纯函数接到 DOM 生命周期上。
 * 浏览器级几何断言在 e2e/Playwright 层做（Testing Decision 5），组件测试
 * 只验证纯函数与状态机行为。
 */

export interface OverlayRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

export type OverlayMotionPath = "fade" | "fallback" | "vt" | "flip";

export const OVERLAY_MOTION = {
  openDuration: 300,
  closeDuration: 240,
  reducedDuration: 100,
  easing: "cubic-bezier(0.22, 0.61, 0.36, 1)",
  fallbackScale: 0.96,
} as const;

/** 浮层封面容器 data-slot（FLIP/VT 的目标锚点）。 */
export const OVERLAY_COVER_SLOT = "detail-cover";
/** 触发卡片封面 data-slot（source 视觉锚点，缺省回退到整卡）。 */
export const OVERLAY_CARD_COVER_SLOT = "card-cover";
/** VT 快照内的共享命名，快照结束后必须清除。 */
export const OVERLAY_VT_NAME = "content-detail-cover";

export function rectHasArea(rect: OverlayRect | null): boolean {
  return Boolean(rect && rect.width > 0 && rect.height > 0);
}

export function rectIntersectsViewport(
  rect: OverlayRect,
  viewportWidth: number,
  viewportHeight: number,
): boolean {
  return (
    rect.x < viewportWidth &&
    rect.y < viewportHeight &&
    rect.x + rect.width > 0 &&
    rect.y + rect.height > 0
  );
}

/**
 * 路径选择顺序（原型 §5）：每次操作独立判定，不缓存：
 * reducedMotion() ? fade : (!sourceRect ? fallback : (vtEnabled() ? vt : flip))
 */
export function selectMotionPath(
  reducedMotion: boolean,
  sourceRect: OverlayRect | null,
  vtAvailable: boolean,
): OverlayMotionPath {
  if (reducedMotion) return "fade";
  if (!rectHasArea(sourceRect)) return "fallback";
  return vtAvailable ? "vt" : "flip";
}

export interface FlipTransform {
  x: number;
  y: number;
  scaleX: number;
  scaleY: number;
}

/** Invert 数学：把 target 矩形（元素自然位姿）映射回 source 矩形（触发卡片封面）。 */
export function computeFlipTransform(source: OverlayRect, target: OverlayRect): FlipTransform {
  return {
    x: source.x - target.x,
    y: source.y - target.y,
    scaleX: target.width > 0 ? source.width / target.width : 1,
    scaleY: target.height > 0 ? source.height / target.height : 1,
  };
}

/** 必须配合 transform-origin: 0 0 使用，比例才围绕元素左上角计算。 */
export function flipTransformToCss(transform: FlipTransform): string {
  return `translate(${transform.x}px, ${transform.y}px) scale(${transform.scaleX}, ${transform.scaleY})`;
}

export function readElementRect(el: Element): OverlayRect {
  const rect = el.getBoundingClientRect();
  return { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
}

/**
 * 测量门（原型 §3）：isConnected + 尺寸 > 0 + 与视口矩形交集，三者全过才返回；
 * 视觉锚点优先取触发元素的卡片封面，缺省回退到触发元素本身。
 */
export function measureSourceRect(
  trigger: HTMLElement | null | undefined,
  viewportWidth = window.innerWidth,
  viewportHeight = window.innerHeight,
): OverlayRect | null {
  if (!trigger || !trigger.isConnected) return null;
  const anchor =
    trigger.querySelector<HTMLElement>(`[data-slot="${OVERLAY_CARD_COVER_SLOT}"]`) ?? trigger;
  const rect = readElementRect(anchor);
  if (!rectHasArea(rect)) return null;
  if (!rectIntersectsViewport(rect, viewportWidth, viewportHeight)) return null;
  return rect;
}

export function reducedMotionEnabled(): boolean {
  return (
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

export function viewTransitionAvailable(): boolean {
  return (
    typeof document !== "undefined" && typeof document.startViewTransition === "function"
  );
}

/** 双 rAF + 120ms setTimeout 双保险（原型 §4.2：避免 VT 期间 rAF 挂起）。 */
export function nextFrame(): Promise<void> {
  return new Promise((resolve) => {
    let settled = false;
    const finish = () => {
      if (!settled) {
        settled = true;
        resolve();
      }
    };
    window.requestAnimationFrame(() => window.requestAnimationFrame(finish));
    window.setTimeout(finish, 120);
  });
}
