import { cn } from "@/lib/utils";

// #412 F4+F5：三套侧边栏壳（公共/Studio/Admin）共享的垂直节奏与分区头
// primitive。两态宽度 48↔228、行高 44px、动画属性白名单 =
// width / opacity / translateX——任何元素不得发生垂直位移。

/** 分区头高度（px）：两态同引单源，收起态等高占位。 */
export const SECTION_HEADER_HEIGHT = 36;

/** 两态一致的导航行高（px）。 */
export const SIDEBAR_ROW_HEIGHT = 44;

/**
 * 分区头（同一组件两态复用，禁止两套元素）：
 * - 展开态：文字可见；文字上方的装饰分隔线（16×1px、水平居中、语义
 *   token 色、aria-hidden、无交互）同步淡出。
 * - 收起态：等高占位，文字 opacity:0，仅分隔线可见。
 * - 文字与分隔线同一布局槽位叠放（不上下堆叠），高度恒为
 *   SECTION_HEADER_HEIGHT，两态切换不产生任何垂直位移。
 */
export function SidebarSectionHeader({
  label,
  collapsed,
}: {
  label: string;
  collapsed: boolean;
}) {
  return (
    <div
      data-sidebar-anchor="section-header"
      style={{ height: SECTION_HEADER_HEIGHT }}
      className="relative w-full shrink-0"
    >
      <span
        aria-hidden="true"
        className={cn(
          "absolute left-1/2 top-1/2 h-px w-4 -translate-x-1/2 -translate-y-1/2 bg-border-default transition-opacity duration-150 motion-reduce:transition-none",
          collapsed ? "opacity-100" : "opacity-0"
        )}
      />
      <span
        className={cn(
          "absolute inset-0 flex items-center px-3.5 pb-1.5 pt-2 text-[10.5px] font-semibold uppercase tracking-wider text-fg-subtle transition-[opacity,transform] duration-100 motion-reduce:transition-none",
          collapsed
            ? "pointer-events-none -translate-x-1 opacity-0"
            : "translate-x-0 opacity-100"
        )}
      >
        {label}
      </span>
    </div>
  );
}

/**
 * 收起态即时 tooltip：纯 CSS hover/focus（无延迟——不得引入 delay-*）。
 * 层策略 = absolute left-full 逃逸侧栏右缘；要求收起态列表不做横向裁切。
 * 装饰性副本（aria-hidden），行的可访问名由 aria-label 承担。
 */
export const SIDEBAR_TOOLTIP_CLASS =
  "pointer-events-none absolute left-full top-1/2 z-50 ml-2 -translate-y-1/2 whitespace-nowrap rounded-md border border-border bg-canvas-default px-3 py-1.5 text-sm text-foreground opacity-0 shadow-md transition-opacity duration-100 motion-reduce:transition-none group-hover:opacity-100 group-focus-visible:opacity-100";

export function SidebarTooltip({ label }: { label: string }) {
  return (
    <span aria-hidden="true" className={SIDEBAR_TOOLTIP_CLASS}>
      {label}
    </span>
  );
}

/**
 * 两态一致的导航行基础样式：44px 行高、图标 16px。文字标签/徽标不进入
 * flex 流（由消费方以 absolute overlay 挂载），保证进出动画
 * （opacity + translateX）不推动图标位置。
 */
export const SIDEBAR_ITEM_BASE =
  "flex min-h-[44px] w-full items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium outline-none transition-[color,background-color] duration-150 select-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background";

/** 收起态行的文字 overlay：绝对定位（不推动图标），opacity+translateX 进出。 */
export const SIDEBAR_ITEM_TEXT_CLASS =
  "pointer-events-none absolute inset-y-0 left-[38px] right-2 flex items-center gap-2 transition-[opacity,transform] duration-150 motion-reduce:transition-none";

/** 列表滚动区两态策略：收起态不裁切（tooltip 逃逸）；展开态独立滚动。 */
export const SIDEBAR_LIST_SCROLL_CLASS =
  "min-h-0 flex-1 overflow-y-auto overflow-x-hidden";
export const SIDEBAR_LIST_NOSCROLL_CLASS =
  "min-h-0 flex-1 overflow-visible";
