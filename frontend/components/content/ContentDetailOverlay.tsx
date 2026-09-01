"use client";

import { useCallback, useEffect, useId, useRef, useState, type RefObject } from "react";
import { useTranslations } from "next-intl";
import { ArrowLeft, X } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  OVERLAY_CARD_COVER_SLOT,
  OVERLAY_COVER_SLOT,
  OVERLAY_MOTION,
  OVERLAY_VT_NAME,
  computeFlipTransform,
  flipTransformToCss,
  measureSourceRect,
  nextFrame,
  readElementRect,
  rectHasArea,
  reducedMotionEnabled,
  selectMotionPath,
  viewTransitionAvailable,
  type OverlayRect,
} from "@/lib/overlay-motion";
import {
  ContentDetailOverlayLayer,
  type OverlayEntry,
  type OverlaySource,
} from "./ContentDetailOverlayLayer";

const MAX_STACK_DEPTH = 5;
const HISTORY_KEY = "contentOverlayDepth";
const OVERLAY_EASING = "cubic-bezier(0.22,0.61,0.36,1)";
/** 入场转场等待层数据的保险时限：超时按降级路径淡入，避免不可见卡死。 */
const ENTRANCE_SAFETY_MS = 2000;

/** 层布局：single = 单列（overlay-scroller 滚动）；split-media = 桌面双栏
    （≥1100px 时唯一滚动容器为层内 layer-scroller）。 */
type LayerLayout = "single" | "split-media";

/** #88 桌面双栏视口判定：与 ui-spec 全局三档（PC > 1100px）一致。 */
function isSplitViewport(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return false;
  return window.matchMedia("(min-width: 1100px)").matches;
}

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
  const shellRef = useRef<HTMLDivElement>(null);

  const [stack, setStack] = useState<OverlayLayerState[]>([]);
  const [closing, setClosing] = useState(false);
  const [stackMove, setStackMove] = useState<"push" | "pop" | null>(null);
  const [popFocus, setPopFocus] = useState<HTMLElement | null>(null);
  const [layerLayouts, setLayerLayouts] = useState<Record<number, LayerLayout>>({});

  const stackRef = useRef<OverlayLayerState[]>(stack);
  const popFocusRef = useRef<HTMLElement | null>(popFocus);
  const closingRef = useRef(false);
  const closeTimerRef = useRef<number | null>(null);
  const restoreRef = useRef<{ trigger: HTMLElement | null; windowY: number } | null>(null);
  const lastOpenRef = useRef(false);
  const topKeyRef = useRef<string | null>(null);
  const onOpenChangeRef = useRef(onOpenChange);
  const layerLayoutsRef = useRef(layerLayouts);

  /* 转场状态（#67 原型 §5 契约）：run token 拦截过期回调；入场只跑一次。 */
  const motionRunRef = useRef(0);
  const motionTimerRef = useRef<number | null>(null);
  const safetyTimerRef = useRef<number | null>(null);
  const entranceDoneRef = useRef(false);
  const sourceRectRef = useRef<OverlayRect | null>(null);
  const sourceAnchorRef = useRef<HTMLElement | null>(null);
  const transitionRef = useRef<ViewTransition | null>(null);

  useEffect(() => {
    stackRef.current = stack;
  }, [stack]);

  useEffect(() => {
    popFocusRef.current = popFocus;
  }, [popFocus]);

  useEffect(() => {
    onOpenChangeRef.current = onOpenChange;
  }, [onOpenChange]);

  useEffect(() => {
    layerLayoutsRef.current = layerLayouts;
  }, [layerLayouts]);

  const handleLayoutChange = useCallback((index: number, layout: LayerLayout) => {
    setLayerLayouts((prev) => (prev[index] === layout ? prev : { ...prev, [index]: layout }));
  }, []);

  /* 当前唯一滚动容器：#88 桌面双栏（split-media 且 ≥1100px）时取顶层可见层的
     layer-scroller（跳过 display:none 的底层），否则回到 overlay-scroller。 */
  const resolveActiveScroller = useCallback((): HTMLElement | null => {
    const scroller = scrollerRef.current;
    if (!scroller) return null;
    if (layerLayoutsRef.current[stackRef.current.length - 1] !== "split-media" || !isSplitViewport()) {
      return scroller;
    }
    const candidates = scroller.querySelectorAll<HTMLElement>('[data-slot="layer-scroller"]');
    for (let i = candidates.length - 1; i >= 0; i -= 1) {
      if (candidates[i].offsetParent !== null) return candidates[i];
    }
    return scroller;
  }, []);

  function pushHistoryState(depth: number) {
    window.history.pushState({ ...(window.history.state ?? {}), [HISTORY_KEY]: depth }, "");
  }

  /* 打开/关闭契约：open 翻转时初始化首层（保存触发元素、页面滚动与 source 几何），
     关闭时走 finalizeClose（幂等）。source 测量必须先于任何布局变更（滚动锁、
     overlay 插入），见原型 §4.3。 */
  useEffect(() => {
    if (open && !lastOpenRef.current) {
      const trigger =
        returnFocusRef?.current ??
        (document.activeElement instanceof HTMLElement ? document.activeElement : null);
      const sourceRect = measureSourceRect(trigger);
      sourceRectRef.current = sourceRect;
      sourceAnchorRef.current = trigger
        ? (trigger.querySelector<HTMLElement>(`[data-slot="${OVERLAY_CARD_COVER_SLOT}"]`) ?? trigger)
        : null;
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

  /* 原生 modal dialog + html/body 双重滚动锁定（含滚动条宽度 padding 补偿）。
     外壳先以 opacity 0 呈现，入场转场从 source 几何放大到最终封面几何；
     层数据迟迟未就绪时由保险定时器兜底淡入。 */
  useEffect(() => {
    if (stack.length === 0) return;
    const dialog = dialogRef.current;
    if (dialog && !dialog.open && typeof dialog.showModal === "function") {
      dialog.showModal();
    }
    const shell = shellRef.current;
    if (shell && !entranceDoneRef.current) {
      shell.style.opacity = "0";
      shell.style.transform = "none";
      shell.style.transition = "";
    }
    if (safetyTimerRef.current === null && !entranceDoneRef.current) {
      safetyTimerRef.current = window.setTimeout(() => {
        safetyTimerRef.current = null;
        if (!entranceDoneRef.current) runEntranceMotion();
      }, ENTRANCE_SAFETY_MS);
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
    const activeScroller = resolveActiveScroller();
    if (activeScroller) activeScroller.scrollTop = layer.scrollTop;
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
    const activeScroller = resolveActiveScroller();
    const next = current.map((layer, index) =>
      index === current.length - 1 ? { ...layer, scrollTop: activeScroller?.scrollTop ?? 0 } : layer,
    );
    next.push({ entry, trigger, scrollTop: 0, title: null });
    setStackMove("push");
    setStack(next);
    pushHistoryState(next.length);
  }, [resolveActiveScroller]);

  /* #89 连续浏览：原地替换顶层为上下文列表下一篇（不压栈、不写历史）。
     新 entry 驱动层 remount（key 含 contentId），媒体区/信息区状态与滚动
     位置全部重置为新内容的初始态。 */
  const switchTopLayer = useCallback((nextEntry: OverlayEntry) => {
    if (closingRef.current || stackRef.current.length === 0) return;
    setStack((prev) =>
      prev.map((layer, index) =>
        index === prev.length - 1
          ? { ...layer, entry: nextEntry, scrollTop: 0, title: null }
          : layer,
      ),
    );
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

  /* ---------- 共享元素转场（#67 原型 §5 契约 / #64 决策 7-12） ---------- */

  const getTopCover = useCallback((): HTMLElement | null => {
    const scroller = scrollerRef.current;
    if (!scroller) return null;
    const covers = scroller.querySelectorAll<HTMLElement>(`[data-slot="${OVERLAY_COVER_SLOT}"]`);
    /* #88 双栏：行内媒体区（min-[1100px]:hidden）也是 detail-cover 锚点，须跳过
       display:none 的隐藏实例，取可见的左栏媒体列。 */
    for (let i = covers.length - 1; i >= 0; i -= 1) {
      if (covers[i].offsetParent !== null || covers[i].getClientRects().length > 0) {
        return covers[i];
      }
    }
    return null;
  }, []);

  /* 中断处理：run token 递增使所有在途回调过期；清过渡、清定时器、清 VT 命名。 */
  const cancelActiveMotion = useCallback(() => {
    motionRunRef.current += 1;
    if (motionTimerRef.current !== null) {
      window.clearTimeout(motionTimerRef.current);
      motionTimerRef.current = null;
    }
    if (safetyTimerRef.current !== null) {
      window.clearTimeout(safetyTimerRef.current);
      safetyTimerRef.current = null;
    }
    const shell = shellRef.current;
    if (shell) {
      shell.style.transition = "";
      shell.style.transform = "";
    }
    const cover = getTopCover();
    if (cover) {
      cover.style.transition = "";
      cover.style.transform = "";
      cover.style.transformOrigin = "";
      cover.style.removeProperty("view-transition-name");
    }
    sourceAnchorRef.current?.style.removeProperty("view-transition-name");
    const transition = transitionRef.current;
    transitionRef.current = null;
    if (transition && typeof transition.skipTransition === "function") {
      try {
        transition.skipTransition();
      } catch {
        /* 忽略：转场可能已结束 */
      }
    }
  }, [getTopCover]);

  /* 不可定位降级：居中 scale(0.96) + 淡化（开 300ms / 关 240ms，共享缓动）。 */
  const runFallbackOpen = useCallback(
    (token: number, shell: HTMLElement) => {
      shell.style.transition = "none";
      shell.style.transform = `scale(${OVERLAY_MOTION.fallbackScale})`;
      shell.style.opacity = "0";
      void nextFrame().then(() => {
        if (token !== motionRunRef.current) return;
        shell.style.transition =
          `opacity ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.easing}, ` +
          `transform ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.easing}`;
        shell.style.opacity = "1";
        shell.style.transform = "none";
      });
    },
    [],
  );

  /* FLIP 开：First = source rect → Last = 浮层封面自然位姿 → Invert（transition:none）
     → 双 rAF 确保绘制 → Play 300ms 共享缓动 → transform:none。 */
  const runFlipOpen = useCallback(
    (token: number, shell: HTMLElement, cover: HTMLElement) => {
      const sourceRect = sourceRectRef.current;
      const targetRect = readElementRect(cover);
      if (!sourceRect || !rectHasArea(targetRect)) {
        runFallbackOpen(token, shell);
        return;
      }
      const invert = computeFlipTransform(sourceRect, targetRect);
      cover.style.transition = "none";
      cover.style.transformOrigin = "0 0";
      cover.style.transform = flipTransformToCss(invert);
      shell.style.transition = `opacity ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.easing}`;
      shell.style.opacity = "1";
      void nextFrame().then(() => {
        if (token !== motionRunRef.current) return;
        cover.style.transition = `transform ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.easing}`;
        cover.style.transform = "none";
        motionTimerRef.current = window.setTimeout(() => {
          if (token !== motionRunRef.current) return;
          cover.style.transition = "";
          cover.style.transform = "";
          cover.style.transformOrigin = "";
          shell.style.transition = "";
        }, OVERLAY_MOTION.openDuration);
      });
    },
    [runFallbackOpen],
  );

  /* VT 开：回调内同步完成 DOM 换名（快照内命名唯一），t.finished reject 或
     同步 Abort 时恢复并走 FLIP 兜底。 */
  const runVtOpen = useCallback(
    (token: number, shell: HTMLElement, cover: HTMLElement) => {
      const cardAnchor = sourceAnchorRef.current;
      shell.style.transition = "";
      shell.style.opacity = "1";
      shell.style.transform = "";
      try {
        cardAnchor?.style.setProperty("view-transition-name", OVERLAY_VT_NAME);
        const transition = document.startViewTransition(() => {
          cover.style.setProperty("view-transition-name", OVERLAY_VT_NAME);
          cardAnchor?.style.removeProperty("view-transition-name");
          shell.style.opacity = "1";
        });
        transitionRef.current = transition;
        transition.finished.then(
          () => {
            if (transitionRef.current !== transition) return;
            transitionRef.current = null;
            cover.style.removeProperty("view-transition-name");
          },
          () => {
            if (transitionRef.current !== transition) return;
            transitionRef.current = null;
            cover.style.removeProperty("view-transition-name");
            cardAnchor?.style.removeProperty("view-transition-name");
            runFlipOpen(token, shell, cover);
          },
        );
      } catch {
        if (transitionRef.current !== null) return;
        cardAnchor?.style.removeProperty("view-transition-name");
        runFlipOpen(token, shell, cover);
      }
    },
    [runFlipOpen],
  );

  /* 入场：reducedMotion() ? fade : (!sourceRect ? fallback : (vtEnabled() ? vt : flip))。
     由顶层 onMotionReady 触发（层数据落定、封面几何可测时），每次打开只跑一次。 */
  const runEntranceMotion = useCallback(() => {
    if (entranceDoneRef.current) return;
    entranceDoneRef.current = true;
    cancelActiveMotion();
    const token = motionRunRef.current;
    const shell = shellRef.current;
    if (!shell) return;

    const path = selectMotionPath(
      reducedMotionEnabled(),
      sourceRectRef.current,
      viewTransitionAvailable(),
    );
    if (path === "fade") {
      shell.style.transition = `opacity ${OVERLAY_MOTION.reducedDuration}ms ease-out`;
      shell.style.opacity = "1";
      return;
    }
    if (path === "fallback") {
      runFallbackOpen(token, shell);
      return;
    }
    const cover = getTopCover();
    if (path === "vt" && cover) {
      runVtOpen(token, shell, cover);
      return;
    }
    if (path === "flip" && cover) {
      runFlipOpen(token, shell, cover);
      return;
    }
    runFallbackOpen(token, shell);
  }, [cancelActiveMotion, getTopCover, runFallbackOpen, runFlipOpen, runVtOpen]);

  const handleMotionReady = useCallback(() => {
    if (entranceDoneRef.current || closingRef.current) return;
    runEntranceMotion();
  }, [runEntranceMotion]);

  /* 完全退出后的恢复契约：还原触发入口、页面滚动位置与焦点。 */
  const finalizeClose = useCallback(() => {
    if (closeTimerRef.current !== null) {
      window.clearTimeout(closeTimerRef.current);
      closeTimerRef.current = null;
    }
    cancelActiveMotion();
    entranceDoneRef.current = false;
    sourceRectRef.current = null;
    sourceAnchorRef.current = null;
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
    setLayerLayouts({});
    const restore = restoreRef.current;
    restoreRef.current = null;
    onOpenChangeRef.current(false);
    window.requestAnimationFrame(() => {
      if (restore) {
        window.scrollTo({ top: restore.windowY, left: 0, behavior: "auto" });
        restore.trigger?.focus({ preventScroll: true });
      }
    });
  }, [cancelActiveMotion]);

  /* 退场：关闭时重新测量 source（用户可能已滚动/虚拟化卸载，原型 §4.4），
     可测 → FLIP 反向回归（VT 可用时走 VT），不可测 → 居中缩淡；reduced-motion
     100ms 纯 opacity。 */
  const runCloseMotion = useCallback(() => {
    const token = motionRunRef.current;
    const shell = shellRef.current;
    if (!shell) return;
    const path = selectMotionPath(
      reducedMotionEnabled(),
      measureSourceRect(restoreRef.current?.trigger ?? null),
      viewTransitionAvailable(),
    );
    const duration =
      path === "fade"
        ? OVERLAY_MOTION.reducedDuration
        : OVERLAY_MOTION.closeDuration;

    if (path === "fade") {
      shell.style.transition = `opacity ${OVERLAY_MOTION.reducedDuration}ms ease-out`;
      shell.style.opacity = "0";
    } else if (path === "fallback") {
      shell.style.transition =
        `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}, ` +
        `transform ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
      shell.style.opacity = "0";
      shell.style.transform = `scale(${OVERLAY_MOTION.fallbackScale})`;
    } else {
      const cover = getTopCover();
      const sourceRect = measureSourceRect(restoreRef.current?.trigger ?? null);
      if (!cover || !sourceRect || !rectHasArea(readElementRect(cover))) {
        shell.style.transition =
          `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}, ` +
          `transform ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
        shell.style.opacity = "0";
        shell.style.transform = `scale(${OVERLAY_MOTION.fallbackScale})`;
      } else if (path === "vt") {
        const cardAnchor = sourceAnchorRef.current;
        cover.style.setProperty("view-transition-name", OVERLAY_VT_NAME);
        try {
          const transition = document.startViewTransition(() => {
            cardAnchor?.style.setProperty("view-transition-name", OVERLAY_VT_NAME);
            cover.style.removeProperty("view-transition-name");
            shell.style.transition = "none";
            shell.style.opacity = "0";
          });
          transitionRef.current = transition;
          transition.finished.then(
            () => {
              if (transitionRef.current !== transition) return;
              transitionRef.current = null;
              cardAnchor?.style.removeProperty("view-transition-name");
            },
            () => {
              if (transitionRef.current !== transition) return;
              transitionRef.current = null;
              cardAnchor?.style.removeProperty("view-transition-name");
              shell.style.transition = "none";
              shell.style.opacity = "1";
              runFlipClose(token, shell, cover);
            },
          );
        } catch {
          if (transitionRef.current !== null) return;
          cardAnchor?.style.removeProperty("view-transition-name");
          runFlipClose(token, shell, cover);
        }
      } else {
        runFlipClose(token, shell, cover);
      }
    }
    closeTimerRef.current = window.setTimeout(finalizeClose, duration + (path === "vt" ? 120 : 0));
  }, [finalizeClose, getTopCover]);

  /* FLIP 关：起点 identity → Play 到 invert 位姿（240ms）+ 外壳淡化。
     关闭方向必须重新测量 source（原型 §4.4）；测量失败降级为居中缩淡。 */
  const runFlipClose = useCallback((token: number, shell: HTMLElement, cover: HTMLElement) => {
    const sourceRect = measureSourceRect(restoreRef.current?.trigger ?? null);
    if (!sourceRect || !rectHasArea(readElementRect(cover))) {
      shell.style.transition =
        `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}, ` +
        `transform ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
      shell.style.opacity = "0";
      shell.style.transform = `scale(${OVERLAY_MOTION.fallbackScale})`;
      return;
    }
    const invert = computeFlipTransform(sourceRect, readElementRect(cover));
    cover.style.transition = "none";
    cover.style.transformOrigin = "0 0";
    cover.style.transform = "none";
    shell.style.transition = `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
    shell.style.opacity = "0";
    void nextFrame().then(() => {
      if (token !== motionRunRef.current) return;
      cover.style.transition = `transform ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
      cover.style.transform = flipTransformToCss(invert);
    });
  }, []);

  /* 退出整个浮层（X / 背板）：按路径执行退场动效（FLIP 反向 / 居中缩淡 /
     reduced-motion 纯淡化），随后 finalizeClose 收尾。 */
  const beginExit = useCallback(() => {
    if (closingRef.current || stackRef.current.length === 0) return;
    closingRef.current = true;
    setClosing(true);
    cancelActiveMotion();
    runCloseMotion();
  }, [cancelActiveMotion, runCloseMotion]);

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
  const topLayout: LayerLayout = (layerLayouts[depth - 1] ?? "single") as LayerLayout;

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
        "lg:m-auto lg:h-[min(92dvh,900px)] lg:w-[min(1120px,calc(100%-2rem))] lg:rounded-lg lg:border lg:border-border lg:bg-card lg:shadow-[var(--elevation-3)]",
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
        ref={shellRef}
        className={cn(
          "grid h-full w-full grid-rows-[auto_minmax(0,1fr)] bg-card",
          closing && "pointer-events-none",
        )}
      >
        <header className="flex items-center gap-2 border-b border-border bg-card px-3 pb-2 pt-[max(0.5rem,env(safe-area-inset-top))] lg:px-4 lg:pb-2.5">
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
          className={cn(
            "min-h-0 overflow-y-auto overscroll-contain px-4 pb-[max(1.5rem,env(safe-area-inset-bottom))] pt-4 lg:px-6",
            /* #88 桌面双栏：顶层为 split-media 时滚动改由层内信息列承担。 */
            topLayout === "split-media" &&
              "min-[1100px]:h-full min-[1100px]:overflow-hidden",
          )}
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
                layerIndex={index}
                onLayoutChange={handleLayoutChange}
                onPush={pushLayer}
                onSwitchNext={switchTopLayer}
                onTitleChange={handleTitleChange(index)}
                onMotionReady={handleMotionReady}
              />
            </div>
          ))}
        </div>
      </div>
    </dialog>
  );
}
