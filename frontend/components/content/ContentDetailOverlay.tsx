"use client";

import { useCallback, useEffect, useId, useRef, useState, type RefObject } from "react";
import { useTranslations } from "next-intl";
import { ArrowLeft, X } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  ContentDetailOverlayLayer,
  type OverlayEntry,
  type OverlaySource,
} from "./ContentDetailOverlayLayer";

const MAX_STACK_DEPTH = 5;
const HISTORY_KEY = "contentOverlayDepth";
const OVERLAY_EASING = "cubic-bezier(0.22,0.61,0.36,1)";

export interface ContentDetailOverlayProps {
  contentId: number;
  zone: "original" | "fanwork";
  source: OverlaySource;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  returnFocusRef?: RefObject<HTMLElement | null>;
  /** #89 连续浏览：触发上下文列表与当前索引（移动端从卡片网格进入时传入）。 */
  contextList?: Array<{ id: number; zone: "original" | "fanwork" }>;
  contextIndex?: number;
}

interface OverlayLayerState {
  entry: OverlayEntry;
  trigger: HTMLElement | null;
  scrollTop: number;
  title: string | null;
}

/** 全站共享内容详情浮层：内部导航栈（深度 ≤ 5）+ 每层滚动记忆 + 退出恢复契约。 */
export function ContentDetailOverlay({
  contentId,
  zone,
  source,
  open,
  onOpenChange,
  returnFocusRef,
  contextList,
  contextIndex,
}: ContentDetailOverlayProps) {
  const t = useTranslations();
  const titleId = useId();

  const dialogRef = useRef<HTMLDialogElement>(null);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const scrollerRef = useRef<HTMLDivElement>(null);

  const [stack, setStack] = useState<OverlayLayerState[]>([]);
  const [closing, setClosing] = useState(false);
  const [stackMove, setStackMove] = useState<"push" | "pop" | null>(null);
  const [popFocus, setPopFocus] = useState<HTMLElement | null>(null);

  const stackRef = useRef<OverlayLayerState[]>(stack);
  const popFocusRef = useRef<HTMLElement | null>(popFocus);
  const closingRef = useRef(false);
  const closeTimerRef = useRef<number | null>(null);
  const restoreRef = useRef<{ trigger: HTMLElement | null; windowY: number } | null>(null);
  const lastOpenRef = useRef(false);
  const topKeyRef = useRef<string | null>(null);
  const onOpenChangeRef = useRef(onOpenChange);

  useEffect(() => {
    stackRef.current = stack;
  }, [stack]);

  useEffect(() => {
    popFocusRef.current = popFocus;
  }, [popFocus]);

  useEffect(() => {
    onOpenChangeRef.current = onOpenChange;
  }, [onOpenChange]);

  function pushHistoryState(depth: number) {
    window.history.pushState({ ...(window.history.state ?? {}), [HISTORY_KEY]: depth }, "");
  }

  /* 打开/关闭契约：open 翻转时初始化首层（保存触发元素与页面滚动），
     关闭时走 finalizeClose（幂等）。 */
  useEffect(() => {
    if (open && !lastOpenRef.current) {
      const trigger =
        returnFocusRef?.current ??
        (document.activeElement instanceof HTMLElement ? document.activeElement : null);
      restoreRef.current = { trigger, windowY: window.scrollY };
      setStack([
        { entry: { contentId, zone, source, contextList, contextIndex }, trigger, scrollTop: 0, title: null },
      ]);
      pushHistoryState(1);
    }
    lastOpenRef.current = open;
    if (!open) finalizeClose();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  /* 原生 modal dialog + html/body 双重滚动锁定（含滚动条宽度 padding 补偿）。 */
  useEffect(() => {
    if (stack.length === 0) return;
    const dialog = dialogRef.current;
    if (dialog && !dialog.open && typeof dialog.showModal === "function") {
      dialog.showModal();
    }
    const docEl = document.documentElement;
    const scrollbarWidth = window.innerWidth - docEl.clientWidth;
    docEl.style.overflow = "hidden";
    document.body.style.overflow = "hidden";
    if (scrollbarWidth > 0) document.body.style.paddingRight = `${scrollbarWidth}px`;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stack.length > 0]);

  useEffect(() => {
    return () => {
      document.documentElement.style.overflow = "";
      document.body.style.overflow = "";
      document.body.style.paddingRight = "";
      const dialog = dialogRef.current;
      if (dialog?.open && typeof dialog.close === "function") dialog.close();
    };
  }, []);

  /* 层切换：新层回到顶部，弹层恢复该层记忆的滚动位置；焦点随之管理。 */
  const topKey =
    stack.length > 0 ? `${stack.length}:${stack[stack.length - 1].entry.contentId}` : null;

  useEffect(() => {
    if (stack.length === 0) {
      topKeyRef.current = null;
      return;
    }
    const previousKey = topKeyRef.current;
    topKeyRef.current = topKey;
    const layer = stack[stack.length - 1];
    const scroller = scrollerRef.current;
    if (scroller) scroller.scrollTop = layer.scrollTop;
    if (previousKey === null) {
      window.setTimeout(() => titleRef.current?.focus({ preventScroll: true }), 0);
      return;
    }
    if (stackMove === "pop" && popFocusRef.current) {
      popFocusRef.current.focus({ preventScroll: true });
      setPopFocus(null);
    } else {
      titleRef.current?.focus({ preventScroll: true });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [topKey]);

  /* 压栈（浮层内关联内容）。 */
  const pushLayer = useCallback((entry: OverlayEntry, trigger: HTMLElement | null) => {
    if (closingRef.current || stackRef.current.length === 0) return;
    if (stackRef.current.length >= MAX_STACK_DEPTH) return;
    const current = stackRef.current;
    const scroller = scrollerRef.current;
    const next = current.map((layer, index) =>
      index === current.length - 1 ? { ...layer, scrollTop: scroller?.scrollTop ?? 0 } : layer,
    );
    next.push({ entry, trigger, scrollTop: 0, title: null });
    setStackMove("push");
    setStack(next);
    pushHistoryState(next.length);
  }, []);

  /* 弹出一层（返回按钮 / Esc / 浏览器后退）。 */
  const popLayer = useCallback(() => {
    if (closingRef.current) return;
    const current = stackRef.current;
    if (current.length < 2) return;
    const popped = current[current.length - 1];
    setPopFocus(popped.trigger);
    setStackMove("pop");
    setStack(current.slice(0, -1));
  }, []);

  /* 完全退出后的恢复契约：还原触发入口、页面滚动位置与焦点。 */
  const finalizeClose = useCallback(() => {
    if (closeTimerRef.current !== null) {
      window.clearTimeout(closeTimerRef.current);
      closeTimerRef.current = null;
    }
    closingRef.current = false;
    setClosing(false);
    document.documentElement.style.overflow = "";
    document.body.style.overflow = "";
    document.body.style.paddingRight = "";
    const dialog = dialogRef.current;
    if (dialog?.open && typeof dialog.close === "function") dialog.close();
    setStack([]);
    setStackMove(null);
    setPopFocus(null);
    const restore = restoreRef.current;
    restoreRef.current = null;
    onOpenChangeRef.current(false);
    window.requestAnimationFrame(() => {
      if (restore) {
        window.scrollTo({ top: restore.windowY, left: 0, behavior: "auto" });
        restore.trigger?.focus({ preventScroll: true });
      }
    });
  }, []);

  /* 退出整个浮层（X / 背板）：退场动效 240ms（reduced-motion 100ms）后收尾。 */
  const beginExit = useCallback(() => {
    if (closingRef.current || stackRef.current.length === 0) return;
    closingRef.current = true;
    setClosing(true);
    const reduced = window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ?? false;
    closeTimerRef.current = window.setTimeout(finalizeClose, reduced ? 100 : 240);
  }, [finalizeClose]);

  const handleExit = useCallback(() => {
    if (closingRef.current || stackRef.current.length === 0) return;
    const depth = stackRef.current.length;
    beginExit();
    if (window.history.state?.[HISTORY_KEY]) window.history.go(-depth);
  }, [beginExit]);

  const handleBack = useCallback(() => {
    if (closingRef.current || stackRef.current.length === 0) return;
    if (stackRef.current.length > 1) {
      popLayer();
    } else {
      beginExit();
    }
  }, [beginExit, popLayer]);

  /* 浏览器后退：每个压栈动作对应一条 history 记录，popstate 逐层弹出；
     栈底（depth 1）时的后退视为全退。 */
  useEffect(() => {
    const handlePopState = () => {
      if (closingRef.current) return;
      const current = stackRef.current;
      if (current.length === 0) return;
      if (current.length > 1) {
        popLayer();
      } else {
        beginExit();
      }
    };
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, [beginExit, popLayer]);

  const handleTitleChange = useCallback((index: number) => {
    return (title: string) => {
      setStack((prev) =>
        prev.map((layer, i) => (i === index && layer.title !== title ? { ...layer, title } : layer)),
      );
    };
  }, []);

  const depth = stack.length;
  const top = depth > 0 ? stack[depth - 1] : null;
  const previous = depth > 1 ? stack[depth - 2] : null;

  function sourceReturnLabel(entrySource: OverlaySource): string {
    switch (entrySource) {
      case "agent-citation":
        return t("contentDetailOverlay.backToAgent");
      case "recommendation":
        return t("contentDetailOverlay.backToRecommendation");
      case "ip-page":
        return t("contentDetailOverlay.backToIpPage");
      default:
        return t("contentDetailOverlay.backToZonePage");
    }
  }

  const returnLabel = previous
    ? previous.title
      ? t("contentDetailOverlay.returnTo", { title: previous.title })
      : sourceReturnLabel(previous.entry.source)
    : top
      ? sourceReturnLabel(top.entry.source)
      : "";

  const topTitle = top?.title ?? "";

  if (depth === 0 && !closing) return null;

  return (
    <dialog
      ref={dialogRef}
      className={cn(
        "content-detail-overlay fixed inset-0 m-0 h-dvh w-full max-h-none max-w-none overflow-hidden border-0 bg-transparent p-0 text-foreground",
        "lg:m-auto lg:h-[min(92dvh,900px)] lg:w-[min(1120px,calc(100%-2rem))] lg:rounded-lg lg:border lg:border-border lg:bg-canvas-default lg:shadow-[var(--elevation-3)]",
      )}
      aria-labelledby={titleId}
      data-closing={closing ? "true" : undefined}
      onCancel={(event) => {
        event.preventDefault();
        handleBack();
      }}
      onClick={(event) => {
        if (event.target === event.currentTarget) handleExit();
      }}
    >
      <div
        className={cn(
          "grid h-full w-full grid-rows-[auto_minmax(0,1fr)] bg-canvas-default",
          "animate-in fade-in-0 zoom-in-95 duration-300 ease-[cubic-bezier(0.22,0.61,0.36,1)]",
          closing && `animate-out fade-out-0 duration-[240ms] ease-[${OVERLAY_EASING}]`,
        )}
      >
        <header className="flex items-center gap-2 border-b border-border bg-canvas-default px-3 pb-2 pt-[max(0.5rem,env(safe-area-inset-top))] lg:px-4 lg:pb-2.5">
          <button
            type="button"
            onClick={handleBack}
            aria-label={returnLabel}
            className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          </button>
          <div className="min-w-0 flex-1">
            <h2
              id={titleId}
              ref={titleRef}
              tabIndex={-1}
              className="truncate text-base font-semibold text-foreground focus:outline-none"
            >
              {topTitle || t("contentDetailOverlay.title")}
            </h2>
            <p className="truncate text-xs text-muted-foreground">{returnLabel}</p>
          </div>
          <button
            type="button"
            onClick={handleExit}
            aria-label={t("contentDetailOverlay.close")}
            className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <X className="h-4 w-4" aria-hidden="true" />
          </button>
        </header>

        <div
          ref={scrollerRef}
          data-slot="overlay-scroller"
          className="min-h-0 overflow-y-auto overscroll-contain px-4 pb-[max(1.5rem,env(safe-area-inset-bottom))] pt-4 lg:px-6"
        >
          {stack.map((layer, index) => (
            <div
              key={`${index}:${layer.entry.contentId}`}
              className={cn(
                index === depth - 1
                  ? cn(
                      "h-full",
                      stackMove === "push" &&
                        `animate-in fade-in-0 slide-in-from-right-12 duration-[240ms] ease-[${OVERLAY_EASING}]`,
                      stackMove === "pop" &&
                        `animate-in fade-in-0 slide-in-from-left-12 duration-[240ms] ease-[${OVERLAY_EASING}]`,
                    )
                  : "hidden",
              )}
            >
              <ContentDetailOverlayLayer
                entry={layer.entry}
                onPush={pushLayer}
                onTitleChange={handleTitleChange(index)}
              />
            </div>
          ))}
        </div>
      </div>
    </dialog>
  );
}
