// 三大页面交互决策纯函数（票 #853，R5 Q23′-A + §十一② 动态命中区）。
// 几何与指针语义在这里单测；DOM 接线在 SurfaceStage（真浏览器验收补几何）。

export interface HitRect {
  left: number;
  top: number;
  right: number;
  bottom: number;
}

export interface PaneHitInput {
  id: string;
  /** 配图（含 transform 后）视觉边界。 */
  frameRect: HitRect;
  /** 配文视觉边界。 */
  labelRect: Rect;
  isOn: boolean;
}

export type Rect = HitRect;

/** 命中区外扩（px），R5 原型同值。 */
export const HIT_PAD_PX = 8;

/** 触屏 tap 预激活防抖窗口（ms），R4 QA 实证先例。 */
export const TAP_PREACTIVATION_MS = 600;

/**
 * 动态命中区（R5-②）：命中 = 该页 配图+配文 当前实际视觉边界的包围盒
 * （调用方传 getBoundingClientRect 结果，含 transform → 缩放后自动跟随）。
 * 规则：空白处不触发也不复位（由调用方只在命中时切换实现「激活粘性」）；
 * 边界贴近重叠时当前激活者优先。
 */
export function resolveHitPane(panes: PaneHitInput[], x: number, y: number, pad: number = HIT_PAD_PX): string | null {
  let hit: string | null = null;
  for (const pane of panes) {
    const left = Math.min(pane.frameRect.left, pane.labelRect.left) - pad;
    const right = Math.max(pane.frameRect.right, pane.labelRect.right) + pad;
    const top = Math.min(pane.frameRect.top, pane.labelRect.top) - pad;
    const bottom = Math.max(pane.frameRect.bottom, pane.labelRect.bottom) + pad;
    if (x >= left && x <= right && y >= top && y <= bottom) {
      if (pane.isOn) return pane.id;
      hit = hit ?? pane.id;
    }
  }
  return hit;
}

export type PaneAction = "navigate" | "activate" | "collapse" | "ignore";

/**
 * 点击语义分流（Q23′-A）：
 * - 桌面（hover+fine pointer）：整卡点击 = 进入对应路由；
 * - 触屏（hover:none）：轻点 = 放大演示（再点收起）；进入走「进入 →」按钮。
 * 键盘 Enter/空格不经过本函数——无条件导航（修正 R5 原型缺陷：
 * 原型 primary() 在无 fine pointer 设备把 Enter 变成预览切换）。
 */
export function resolvePaneAction(opts: {
  finePointer: boolean;
  isOn: boolean;
  msSincePreActivated: number;
}): PaneAction {
  if (opts.finePointer) return "navigate";
  const justPreActivated = opts.msSincePreActivated < TAP_PREACTIVATION_MS;
  if (opts.isOn && !justPreActivated) return "collapse";
  if (!opts.isOn) return "activate";
  return "ignore";
}
