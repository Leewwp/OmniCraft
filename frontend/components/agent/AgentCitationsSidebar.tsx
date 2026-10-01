"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { X } from "lucide-react";
import type { AgentCitation } from "@/lib/agent";
import { cn } from "@/lib/utils";
import { useDelayedUnmount } from "@/lib/use-delayed-unmount";
import { AgentCitationCard } from "@/components/agent/AgentCitationCard";

/**
 * FT-4（#696）「参考来源」侧栏：
 * - 桌面（≥768px）：对话区右缘固定宽侧栏（#751：min(440px,38vw)，最大
 *   440px、窄桌面按 38vw 收窄，顶栏下方起高），作为 flex 兄弟列推挤对话
 *   区——消息列 max-w-3xl 与文本宽度不变。
 * - 移动（<768px）：底部抽屉全宽，手写 pointer events 三路关闭
 *   （把手拖拽过半 / 快滑 / 内容滚动到顶后继续下拉），不引第三方依赖。
 * 内容 = 最近一次点击「N 条参考来源」的那条回答的 citations（复用
 * AgentCitationCard）。原内联折叠列表（AgentCitationList）随本票退役。
 * #751：两分支滚动区只留纵向；列表 grid-cols-1 + li min-w-0 + 卡片截断链
 * 共同消灭横向溢出（overflow-x-hidden 仅兜底）。
 */

interface AgentCitationsSidebarProps {
  open: boolean;
  onClose: () => void;
  citations: AgentCitation[];
  onOpen: (citation: AgentCitation, trigger: HTMLElement) => void;
  /** 行内 [n] 角标锚定高亮（透传卡片）。 */
  highlightedIndex?: number | null;
}

/** 抽屉手势阈值：位移过半或快滑（>0.5px/ms）即收起。 */
const DRAG_CLOSE_RATIO = 0.5;
const FAST_FLING_PX_PER_MS = 0.5;

/* #755（#718 双分支收敛）：移动/桌面两分支共享的头部与列表——标题、计数、
   关闭钮与卡片列表单一来源；容器类与关闭钮 hover 底色（移动 canvas-default
   / 桌面 canvas-subtle，历史差异原样保留）由分支以 props 传入，布局壳
   （抽屉手势/遮罩 vs flex 兄弟列）留在各分支。输出 DOM 与收敛前逐字一致。 */

function SidebarHeader({ count, onClose, containerClassName, closeHoverClassName }: {
  count: number;
  onClose: () => void;
  containerClassName: string;
  closeHoverClassName: string;
}) {
  const t = useTranslations();
  return (
    <div className={containerClassName}>
      <h3 className="text-sm font-semibold text-fg-default">
        {t("agent.citations.title")}
        <span className="ml-2 text-xs font-normal text-fg-muted">
          {t("agent.citations.count", { count })}
        </span>
      </h3>
      <button
        type="button"
        aria-label={t("agent.citations.close")}
        onClick={onClose}
        className={cn(
          "inline-flex size-7 items-center justify-center rounded-md text-fg-muted hover:text-foreground focus:outline-none focus-visible:ring-1 focus:ring-ring",
          closeHoverClassName,
        )}
      >
        <X className="size-4" aria-hidden="true" />
      </button>
    </div>
  );
}

function SidebarList({ citations, onOpen, highlightedIndex }: {
  citations: AgentCitation[];
  onOpen: (citation: AgentCitation, trigger: HTMLElement) => void;
  highlightedIndex?: number | null;
}) {
  return (
    <ul className="grid grid-cols-1 gap-2">
      {citations.map((citation, index) => (
        <li key={`${citation.contentId}-${index}`} className="min-w-0">
          <AgentCitationCard
            citation={citation}
            index={index}
            onOpen={onOpen}
            highlighted={highlightedIndex === index}
          />
        </li>
      ))}
    </ul>
  );
}

export function AgentCitationsSidebar({ open, onClose, citations, onOpen, highlightedIndex }: AgentCitationsSidebarProps) {
  const t = useTranslations();
  const [isMobile, setIsMobile] = useState(false);
  const [dragOffset, setDragOffset] = useState(0);
  const dragStateRef = useRef<{ startY: number; startTs: number; fromScrollTop: boolean } | null>(null);
  const contentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    /* matchMedia 缺失（测试环境）按桌面侧栏渲染。 */
    if (typeof window.matchMedia !== "function") return;
    const mq = window.matchMedia("(max-width: 767px)");
    const update = () => setIsMobile(mq.matches);
    update();
    mq.addEventListener("change", update);
    return () => mq.removeEventListener("change", update);
  }, []);

  /* 抽屉手势：把手区 pointerdown 起拖；内容区仅在滚动到顶时接管下拉。
     桌面侧栏不参与手势。 */
  const beginDrag = useCallback((event: React.PointerEvent, fromScrollTop: boolean) => {
    if (event.pointerType === "mouse") return; // 桌面鼠标不走抽屉手势
    dragStateRef.current = { startY: event.clientY, startTs: Date.now(), fromScrollTop };
    (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
  }, []);

  const moveDrag = useCallback((event: React.PointerEvent) => {
    const state = dragStateRef.current;
    if (!state) return;
    const dy = event.clientY - state.startY;
    if (dy > 0) setDragOffset(dy);
  }, []);

  const endDrag = useCallback(() => {
    const state = dragStateRef.current;
    dragStateRef.current = null;
    if (!state) return;
    const dy = dragOffset;
    const duration = Math.max(1, Date.now() - state.startTs);
    const height = window.innerHeight * 0.7; // 抽屉标称高度近似
    if (dy > height * DRAG_CLOSE_RATIO || dy / duration > FAST_FLING_PX_PER_MS) {
      setDragOffset(0);
      onClose();
    } else {
      setDragOffset(0);
    }
  }, [dragOffset, onClose]);

  /* #721：延迟卸载走完退出动画（关闭方向滑出 + 遮罩淡出）；citations
     清空（切换会话）立即卸载不播动画。 */
  const mounted = useDelayedUnmount(open, 220);
  if (!mounted || citations.length === 0) return null;

  if (isMobile) {
    return (
      <div
        className="fixed inset-0 z-40 md:hidden"
        {...(open ? { role: "dialog", "aria-modal": true } : { "aria-hidden": true, "inert": true as never })}
        aria-label={t("agent.citations.title")}
        onKeyDown={(event) => {
          if (event.key === "Escape") onClose();
        }}
      >
        {/* 轻遮罩：点按即收（第三路兜底关闭） */}
        <button
          type="button"
          aria-label={t("agent.citations.close")}
          className={cn(
            "absolute inset-0 bg-black/30 transition-opacity duration-200 motion-reduce:transition-none",
            open ? "opacity-100" : "opacity-0",
          )}
          onClick={onClose}
        />
        <div
          className={cn(
            "absolute inset-x-0 bottom-0 flex max-h-[70vh] flex-col rounded-t-xl border-t border-border-default bg-canvas-default shadow-[var(--elevation-3)]",
            "transition-transform duration-200 ease-out motion-reduce:transition-none",
            !open && "translate-y-full",
          )}
          style={{ transform: dragOffset > 0 ? `translateY(${dragOffset}px)` : undefined, transition: dragOffset > 0 ? "none" : undefined }}
        >
          {/* 把手区：可拖拽关闭 */}
          <div
            className="flex cursor-grab touch-none items-center justify-center py-2"
            onPointerDown={(e) => beginDrag(e, false)}
            onPointerMove={moveDrag}
            onPointerUp={endDrag}
            onPointerCancel={endDrag}
          >
            <span className="h-1.5 w-10 rounded-full bg-border-strong" aria-hidden="true" />
          </div>
          <SidebarHeader
            count={citations.length}
            onClose={onClose}
            containerClassName="flex items-center justify-between px-4 pb-2"
            closeHoverClassName="hover:bg-canvas-default"
          />
          {/* 内容区：滚动到顶后继续下拉 → 接管手势关闭（#751：横向同兜底） */}
          <div
            ref={contentRef}
            className="min-h-0 flex-1 touch-pan-y overflow-y-auto overflow-x-hidden px-3 pb-6"
            onPointerDown={(e) => {
              if ((contentRef.current?.scrollTop ?? 0) <= 0) beginDrag(e, true);
            }}
            onPointerMove={(event) => {
              const state = dragStateRef.current;
              /* 滚动接管优先：除非已起拖且继续下拉，否则不动手势。 */
              if (state?.fromScrollTop) moveDrag(event);
            }}
            onPointerUp={endDrag}
            onPointerCancel={endDrag}
          >
            <SidebarList citations={citations} onOpen={onOpen} highlightedIndex={highlightedIndex} />
          </div>
        </div>
      </div>
    );
  }

  /* 桌面：对话区右缘侧栏（flex 兄弟列推挤，非覆盖）。#751：最大 440px、
     窄桌面按 38vw 收窄（min(440px,38vw)）；内容滚动只留纵向，横向以
     overflow-x-hidden 兜底（截断链修复见卡片与列表层，不靠裁内容冒充）。 */
  return (
    <aside
      aria-label={t("agent.citations.title")}
      data-testid="citations-sidebar"
      className={cn(
        "hidden w-[min(440px,38vw)] shrink-0 flex-col border-l border-border-default bg-canvas-default md:flex",
        "transition-[opacity,transform] duration-200 ease-out motion-reduce:transition-none",
        open ? "translate-x-0 opacity-100" : "translate-x-2 opacity-0",
      )}
    >
      <SidebarHeader
        count={citations.length}
        onClose={onClose}
        containerClassName="flex items-center justify-between border-b border-border-default px-4 py-3"
        closeHoverClassName="hover:bg-canvas-subtle"
      />
      <div className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden p-3">
        <SidebarList citations={citations} onOpen={onOpen} highlightedIndex={highlightedIndex} />
      </div>
    </aside>
  );
}
