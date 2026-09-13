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
import { useAuthGate } from "@/components/auth/AuthGateProvider";

const MAX_STACK_DEPTH = 5;
const HISTORY_KEY = "contentOverlayDepth";
const OVERLAY_EASING = "cubic-bezier(0.22,0.61,0.36,1)";
/** 入场转场等待层数据的保险时限：超时按降级路径淡入，避免不可见卡死。 */
const ENTRANCE_SAFETY_MS = 2000;
/** 封面媒体就绪门上限：转场开始前最多等这么久让封面图完成加载。 */
const COVER_READY_CAP_MS = 150;
/** #409 F1：关闭方向 VT 的方向标记（root/group 快照时长 240ms 档，
    见 globals.css :root[data-vt-close] 规则）。 */
const VT_CLOSE_ATTR = "data-vt-close";

/** 层布局：single = 单列（overlay-scroller 滚动）；split-media = 桌面双栏
    （≥1100px 时唯一滚动容器为层内 layer-scroller）；variant = #397 竖屏集新版
    布局（滚动归属同 split-media；壳层去 header，返回/关闭改悬浮半透明圆钮）。 */
type LayerLayout = "single" | "split-media" | "variant";

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
  const { setPortalContainer, isGateOpen } = useAuthGate();

  const dialogRef = useRef<HTMLDialogElement>(null);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const scrollerRef = useRef<HTMLDivElement>(null);
  const shellRef = useRef<HTMLDivElement>(null);
  /* 遮罩同钟（2026-09-09）：dialog 外兄弟层承载遮罩视觉，与壳层/封面同一
     时钟驱动。::backdrop 动画废弃——VT 快照期间真实元素不渲染、Safari 也不
     执行 ::backdrop 动画，实测遮罩瞬现瞬消与图片放缩完全脱钩。 */
  const backdropRef = useRef<HTMLDivElement>(null);

  const [stack, setStack] = useState<OverlayLayerState[]>([]);
  const [closing, setClosing] = useState(false);
  const [stackMove, setStackMove] = useState<"push" | "pop" | null>(null);
  const [popFocus, setPopFocus] = useState<HTMLElement | null>(null);
  const [layerLayouts, setLayerLayouts] = useState<Record<number, LayerLayout>>({});
  /* #409 F1 单一时间轴配套：入场转场落定标记（驱动首帧保持解除与媒体列
     几何解冻）+ 首帧保持 src（打开瞬间卡片封面实际渲染的地址）。 */
  const [entranceSettled, setEntranceSettled] = useState(false);
  const [motionHoldSrc, setMotionHoldSrc] = useState<string | null>(null);

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

  /* 当前唯一滚动容器：#88 桌面双栏（split-media/variant 且 ≥1100px）时取顶层
     可见层的 layer-scroller（跳过 display:none 的底层），否则回到 overlay-scroller。 */
  const resolveActiveScroller = useCallback((): HTMLElement | null => {
    const scroller = scrollerRef.current;
    if (!scroller) return null;
    const mode = layerLayoutsRef.current[stackRef.current.length - 1];
    if ((mode !== "split-media" && mode !== "variant") || !isSplitViewport()) {
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
     overlay 插入），见原型 §4.3。
     #398 C3：打开瞬间锁定触发卡片的 hover 缩放（globals.css 以
     [data-overlay-motion-lock] 归位），VT 旧快照捕获时 <img> 处于未变换位姿；
     finalizeClose 移除锁。 */
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
      /* #409 F1 首帧保持：记录卡片封面此刻实际渲染的地址（currentSrc 反映已选
         变体）；媒体链选出不同文件时，入场窗口内浮窗首帧仍渲染该地址，
         落定后再切换（动效期间零换图）。 */
      const cardImg = sourceAnchorRef.current?.querySelector("img") ?? null;
      setMotionHoldSrc(cardImg?.currentSrc || cardImg?.getAttribute("src") || null);
      setEntranceSettled(false);
      trigger?.setAttribute("data-overlay-motion-lock", "");
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
     #409 F1 单一时间轴：外壳保持 opacity 0 直到入场转场起跑（壳层不透明度与
     封面几何同帧起跑/同长结束）——旧「160ms 先行反馈淡入」与 VT root 180ms
     交叉淡化并存的档位差已移除；数据迟迟未就绪由保险定时器兜底降级淡入。 */
  useEffect(() => {
    if (stack.length === 0) return;
    const dialog = dialogRef.current;
    if (dialog && !dialog.open && typeof dialog.showModal === "function") {
      dialog.showModal();
    }
    const shell = shellRef.current;
    if (shell && !entranceDoneRef.current) {
      shell.style.transition = "none";
      shell.style.transform = "none";
      shell.style.opacity = "0";
    }
    /* 遮罩按压反馈层：点击确认（数据未就绪期的唯一视觉反馈），随后由入场
       动效同钟接棒升至 1。 */
    const backdrop = backdropRef.current;
    if (backdrop && !entranceDoneRef.current) {
      backdrop.style.transition = `opacity ${OVERLAY_MOTION.backdropFeedbackMs}ms ease-out`;
      backdrop.style.opacity = `${OVERLAY_MOTION.backdropFeedbackOpacity}`;
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

  /* SP-17/T2：登录浮窗渲染进 dialog 内部（top layer 子树），避免被压层；
     栈清空（浮窗关闭/卸载）时必须回收容器，防 portal 进 detached 元素。 */
  useEffect(() => {
    if (stack.length > 0) {
      const dialog = dialogRef.current;
      if (dialog) setPortalContainer(dialog);
      return;
    }
    setPortalContainer(null);
    return undefined;
  }, [stack.length, setPortalContainer]);

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

  /* #398 C1：VT 命名下沉到封面 <img> 本身（盒内 object-contain、同源变体）——
     命名盒会把底色/边框/控件条差异一起烘进快照；命名 <img> 让 group 只承载
     图像内容，两端底色差异随 root 交叉淡化消化。无 <img>（视频/骨架）时回退
     命名盒。 */
  function getVtElement(host: HTMLElement): HTMLElement {
    return host.querySelector<HTMLElement>("img") ?? host;
  }

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
    document.documentElement.removeAttribute(VT_CLOSE_ATTR);
    const shell = shellRef.current;
    if (shell) {
      shell.style.transition = "";
      shell.style.transform = "";
      shell.style.removeProperty("will-change");
    }
    const cover = getTopCover();
    if (cover) {
      cover.style.transition = "";
      cover.style.transform = "";
      cover.style.transformOrigin = "";
      cover.style.removeProperty("will-change");
      cover.style.removeProperty("view-transition-name");
      getVtElement(cover).style.removeProperty("view-transition-name");
    }
    const cardAnchor = sourceAnchorRef.current;
    if (cardAnchor) {
      cardAnchor.style.removeProperty("view-transition-name");
      getVtElement(cardAnchor).style.removeProperty("view-transition-name");
    }
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

  /* #409 F1 入场落定：单一时间轴动画全部结束后标记——解除首帧保持
     （媒体链不同文件时此刻才切换）与媒体列几何冻结。 */
  const markEntranceSettled = useCallback((token: number) => {
    if (token !== motionRunRef.current) return;
    setEntranceSettled(true);
  }, []);

  /* 不可定位降级：居中 scale(0.96) + 淡化（开 300ms / 关 240ms，共享缓动；
     #409 F1：动效元素临时提升合成层，落定后释放）。 */
  const runFallbackOpen = useCallback(
    (token: number, shell: HTMLElement) => {
      shell.style.transition = "none";
      shell.style.transform = `scale(${OVERLAY_MOTION.fallbackScale})`;
      shell.style.opacity = "0";
      void nextFrame().then(() => {
        if (token !== motionRunRef.current) return;
        shell.style.willChange = "opacity, transform";
        shell.style.transition =
          `opacity ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.easing}, ` +
          `transform ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.easing}`;
        shell.style.opacity = "1";
        shell.style.transform = "none";
        const backdrop = backdropRef.current;
        if (backdrop) {
          backdrop.style.transition = `opacity ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.easing}`;
          backdrop.style.opacity = "1";
        }
        motionTimerRef.current = window.setTimeout(() => {
          if (token !== motionRunRef.current) return;
          shell.style.removeProperty("will-change");
          markEntranceSettled(token);
        }, OVERLAY_MOTION.openDuration);
      });
    },
    [markEntranceSettled],
  );

  /* FLIP 开：First = source rect → Last = 浮层封面自然位姿 → Invert（transition:none）
     → 双 rAF 确保绘制 → Play 300ms 共享缓动 → transform:none。壳层不透明度与
     封面几何同帧起跑/同长结束（#409 F1 单一时间轴）。 */
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
      shell.style.transition = "none";
      shell.style.opacity = "0";
      void nextFrame().then(() => {
        if (token !== motionRunRef.current) return;
        /* #409 F1：合成层提升（动效期间消除重绘型闪烁，结束释放）。 */
        cover.style.willChange = "transform";
        shell.style.willChange = "opacity";
        cover.style.transition = `transform ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.easing}`;
        cover.style.transform = "none";
        shell.style.transition = `opacity ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.easing}`;
        shell.style.opacity = "1";
        const backdrop = backdropRef.current;
        if (backdrop) {
          backdrop.style.transition = `opacity ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.easing}`;
          backdrop.style.opacity = "1";
        }
        motionTimerRef.current = window.setTimeout(() => {
          if (token !== motionRunRef.current) return;
          cover.style.transition = "";
          cover.style.transform = "";
          cover.style.transformOrigin = "";
          cover.style.removeProperty("will-change");
          shell.style.transition = "";
          shell.style.removeProperty("will-change");
          markEntranceSettled(token);
        }, OVERLAY_MOTION.openDuration);
      });
    },
    [markEntranceSettled, runFallbackOpen],
  );

  /* VT 开：回调内同步完成 DOM 换名（快照内命名唯一）。兜底挂在 ready 上
     （2026-09-06 实测修复）：VT 被浏览器跳过时 ready reject 而 finished 可能
     仍正常 resolve，只挂 finished 的失败分支会漏掉 FLIP 兜底、浮层瞬间凭空
     出现。#398：命名落在两端 <img>（getVtElement）。#409 F1：root 交叉淡化
     时长/缓动与命名组同源（globals.css 300ms 档），finished 即入场落定。 */
  const runVtOpen = useCallback(
    (token: number, shell: HTMLElement, cover: HTMLElement) => {
      const cardAnchor = sourceAnchorRef.current;
      const coverVt = getVtElement(cover);
      const cardVt = cardAnchor ? getVtElement(cardAnchor) : null;
      /* 快照前壳层必须保持 opacity 0（旧态=纯信息流）：置 1 只能在回调内
         （新态）。2026-09-09 实测修复——回调前置 1 会烘进旧快照，root 交叉
         淡化失去壳层渐显，观感为壳层瞬现（用户录屏实锤）；桌面面板视觉
         （底色/边框/圆角/阴影）已随壳层走，dialog 永久透明，旧快照无白板。 */
      shell.style.transition = "";
      shell.style.transform = "";
      /* 遮罩经 VT root 交叉淡化承载：回调内瞬时置 1（transition 必须为 none），
         新旧快照的遮罩差值随 300ms 交叉淡化自然与封面组放缩同步呈现。 */
      const backdrop = backdropRef.current;
      if (backdrop) backdrop.style.transition = "none";
      try {
        cardVt?.style.setProperty("view-transition-name", OVERLAY_VT_NAME);
        const transition = document.startViewTransition(() => {
          coverVt.style.setProperty("view-transition-name", OVERLAY_VT_NAME);
          cardVt?.style.removeProperty("view-transition-name");
          shell.style.opacity = "1";
          if (backdropRef.current) backdropRef.current.style.opacity = "1";
        });
        transitionRef.current = transition;
        let fallbackStarted = false;
        const startFallback = () => {
          if (fallbackStarted) return;
          fallbackStarted = true;
          transitionRef.current = null;
          coverVt.style.removeProperty("view-transition-name");
          cardVt?.style.removeProperty("view-transition-name");
          runFlipOpen(token, shell, cover);
        };
        transition.ready.then(() => {}, startFallback);
        transition.finished.then(
          () => {
            if (fallbackStarted || transitionRef.current !== transition) return;
            transitionRef.current = null;
            coverVt.style.removeProperty("view-transition-name");
            markEntranceSettled(token);
          },
          () => {
            if (transitionRef.current !== transition) return;
            startFallback();
          },
        );
      } catch {
        if (transitionRef.current !== null) return;
        cardVt?.style.removeProperty("view-transition-name");
        runFlipOpen(token, shell, cover);
      }
    },
    [markEntranceSettled, runFlipOpen],
  );

  /* 入场：reducedMotion() ? fade : (!sourceRect ? fallback : (vtEnabled() ? vt : flip))。
     由顶层 onMotionReady 触发（层数据落定、封面几何可测时），每次打开只跑一次。
     vt/flip 启动前有封面媒体就绪门（#398 C4：decode 真就绪 + 150ms 极端网络
     兜底；点击卡片的同源变体预取使解码通常已完成），避免变形过程中是空框、
     图片加载完成后突现（2026-09-06 实测修复）。超高图首项走 fallback（C2）。 */
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
      const backdrop = backdropRef.current;
      if (backdrop) {
        backdrop.style.transition = `opacity ${OVERLAY_MOTION.reducedDuration}ms ease-out`;
        backdrop.style.opacity = "1";
      }
      motionTimerRef.current = window.setTimeout(() => {
        markEntranceSettled(token);
      }, OVERLAY_MOTION.reducedDuration);
      return;
    }
    if (path === "fallback") {
      runFallbackOpen(token, shell);
      return;
    }
    const cover = getTopCover();
    if (!cover) {
      runFallbackOpen(token, shell);
      return;
    }
    /* #398 C2：超高图首项不共享元素转场——卡片端按 400px 高度上限 contain 整图、
       浮窗端按 3:4 名义宽 + 内部滚动（顶部裁切），两端取景语义无法统一，强行
       变形必现「先完整展示后被截断」；退化为居中缩淡（无换图、无跳变）。 */
    if (cover.dataset.ultraTall === "true") {
      runFallbackOpen(token, shell);
      return;
    }
    /* #398 C4 真就绪：等浮窗实际渲染的封面 <img> 解码完成（decode 优于 load——
       保证已光栅化）；点击卡片瞬间的同源变体预取（ContentCard）使解码在数据
       落定前大概率已完成，150ms 上限仅极端网络兜底。 */
    const media = cover.querySelector("img");
    const ready =
      media instanceof Element && media.tagName === "IMG"
        ? (media as HTMLImageElement).complete && (media as HTMLImageElement).naturalWidth > 0
          ? Promise.resolve()
          : typeof (media as HTMLImageElement).decode === "function"
            ? (media as HTMLImageElement).decode().catch(() => {})
            : new Promise<void>((resolve) => {
                media.addEventListener("load", () => resolve(), { once: true });
                media.addEventListener("error", () => resolve(), { once: true });
              })
        : Promise.resolve();
    void Promise.race([
      ready,
      new Promise<void>((resolve) => {
        window.setTimeout(resolve, COVER_READY_CAP_MS);
      }),
    ]).then(() => {
      if (token !== motionRunRef.current || closingRef.current) return;
      if (path === "vt") runVtOpen(token, shell, cover);
      else runFlipOpen(token, shell, cover);
    });
  }, [cancelActiveMotion, getTopCover, markEntranceSettled, runFallbackOpen, runFlipOpen, runVtOpen]);

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
    setEntranceSettled(false);
    setMotionHoldSrc(null);
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
    /* #398 C3：解除触发卡片的 hover 缩放锁定（打开时设置）。 */
    restore?.trigger?.removeAttribute("data-overlay-motion-lock");
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
     100ms 纯 opacity。#409 F1：关闭方向单一时钟 240ms——VT 经 data-vt-close
     把 root 交叉淡化与命名组一同压到 240ms 档；FLIP 壳层与封面同帧起跑。 */
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
      const backdrop = backdropRef.current;
      if (backdrop) {
        backdrop.style.transition = `opacity ${OVERLAY_MOTION.reducedDuration}ms ease-out`;
        backdrop.style.opacity = "0";
      }
    } else if (path === "fallback") {
      shell.style.willChange = "opacity, transform";
      shell.style.transition =
        `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}, ` +
        `transform ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
      shell.style.opacity = "0";
      shell.style.transform = `scale(${OVERLAY_MOTION.fallbackScale})`;
      const backdrop = backdropRef.current;
      if (backdrop) {
        backdrop.style.transition = `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
        backdrop.style.opacity = "0";
      }
    } else {
      const cover = getTopCover();
      const sourceRect = measureSourceRect(restoreRef.current?.trigger ?? null);
      /* 超高图当前项同样退化为居中缩淡（与开路径同因，见 runEntranceMotion）。 */
      if (!cover || !sourceRect || !rectHasArea(readElementRect(cover)) || cover.dataset.ultraTall === "true") {
        shell.style.willChange = "opacity, transform";
        shell.style.transition =
          `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}, ` +
          `transform ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
        shell.style.opacity = "0";
        shell.style.transform = `scale(${OVERLAY_MOTION.fallbackScale})`;
        const backdrop = backdropRef.current;
        if (backdrop) {
          backdrop.style.transition = `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
          backdrop.style.opacity = "0";
        }
      } else if (path === "vt") {
        const cardAnchor = sourceAnchorRef.current;
        const coverVt = getVtElement(cover);
        const cardVt = cardAnchor ? getVtElement(cardAnchor) : null;
        coverVt.style.setProperty("view-transition-name", OVERLAY_VT_NAME);
        /* #409 F1：关闭方向标记（globals.css 把 root 交叉淡化与命名组压到
           240ms 档）；转场结束移除。 */
        document.documentElement.setAttribute(VT_CLOSE_ATTR, "");
        /* 遮罩同钟（关）：回调内瞬时置 0，root 交叉淡化把 1→0 与封面组
           回归放缩同窗呈现（transition 置 none 防真实元素在快照期自跑）。 */
        const backdrop = backdropRef.current;
        if (backdrop) backdrop.style.transition = "none";
        try {
          const transition = document.startViewTransition(() => {
            cardVt?.style.setProperty("view-transition-name", OVERLAY_VT_NAME);
            coverVt.style.removeProperty("view-transition-name");
            shell.style.transition = "none";
            shell.style.opacity = "0";
            if (backdropRef.current) backdropRef.current.style.opacity = "0";
          });
          transitionRef.current = transition;
          /* 兜底挂在 ready 上（与开路径同因）：跳过场景 finished 可能正常
             resolve，只挂 finished 失败分支会漏掉 FLIP 兜底。 */
          let fallbackStarted = false;
          const startFallback = () => {
            if (fallbackStarted) return;
            fallbackStarted = true;
            transitionRef.current = null;
            document.documentElement.removeAttribute(VT_CLOSE_ATTR);
            cardVt?.style.removeProperty("view-transition-name");
            shell.style.transition = "none";
            shell.style.opacity = "1";
            runFlipClose(token, shell, cover);
          };
          transition.ready.then(() => {}, startFallback);
          transition.finished.then(
            () => {
              if (fallbackStarted || transitionRef.current !== transition) return;
              transitionRef.current = null;
              document.documentElement.removeAttribute(VT_CLOSE_ATTR);
              cardVt?.style.removeProperty("view-transition-name");
            },
            () => {
              if (transitionRef.current !== transition) return;
              startFallback();
            },
          );
        } catch {
          document.documentElement.removeAttribute(VT_CLOSE_ATTR);
          if (transitionRef.current !== null) return;
          cardVt?.style.removeProperty("view-transition-name");
          runFlipClose(token, shell, cover);
        }
      } else {
        runFlipClose(token, shell, cover);
      }
    }
    closeTimerRef.current = window.setTimeout(finalizeClose, duration + (path === "vt" ? 120 : 0));
  }, [finalizeClose, getTopCover]);

  /* FLIP 关：起点 identity → Play 到 invert 位姿（240ms）+ 外壳淡化（同一时钟）。
     关闭方向必须重新测量 source（原型 §4.4）；测量失败降级为居中缩淡。 */
  const runFlipClose = useCallback((token: number, shell: HTMLElement, cover: HTMLElement) => {
    const sourceRect = measureSourceRect(restoreRef.current?.trigger ?? null);
    if (!sourceRect || !rectHasArea(readElementRect(cover))) {
      shell.style.willChange = "opacity, transform";
      shell.style.transition =
        `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}, ` +
        `transform ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
      shell.style.opacity = "0";
      shell.style.transform = `scale(${OVERLAY_MOTION.fallbackScale})`;
      const backdrop = backdropRef.current;
      if (backdrop) {
        backdrop.style.transition = `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
        backdrop.style.opacity = "0";
      }
      return;
    }
    const invert = computeFlipTransform(sourceRect, readElementRect(cover));
    cover.style.transition = "none";
    cover.style.transformOrigin = "0 0";
    cover.style.transform = "none";
    shell.style.transition = "none";
    shell.style.opacity = "1";
    void nextFrame().then(() => {
      if (token !== motionRunRef.current) return;
      cover.style.willChange = "transform";
      shell.style.willChange = "opacity";
      cover.style.transition = `transform ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
      cover.style.transform = flipTransformToCss(invert);
      shell.style.transition = `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
      shell.style.opacity = "0";
      const backdrop = backdropRef.current;
      if (backdrop) {
        backdrop.style.transition = `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.easing}`;
        backdrop.style.opacity = "0";
      }
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

  /* #409 F1：入场落定后解除首帧保持——媒体链与卡片封面不同文件时，
     此刻才切换到真实媒体（动效期间零换图）。 */
  useEffect(() => {
    if (entranceSettled && motionHoldSrc !== null) setMotionHoldSrc(null);
  }, [entranceSettled, motionHoldSrc]);

  const depth = stack.length;
  const top = depth > 0 ? stack[depth - 1] : null;
  const previous = depth > 1 ? stack[depth - 2] : null;
  const topLayout: LayerLayout = (layerLayouts[depth - 1] ?? "single") as LayerLayout;
  /* #397 方案二 float 壳层：顶层为 variant 时整个移除 header（grid 单行），
     返回/关闭改悬浮半透明圆钮；sr-only 标题保留（无障碍名称/初始焦点/多层栈
     返回文案三职迁移不可遗漏）。 */
  const topIsVariant = topLayout === "variant";

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

  function sourceNounLabel(entrySource: OverlaySource): string {
    switch (entrySource) {
      case "agent-citation":
        return t("contentDetailOverlay.sourceNounAgent");
      case "recommendation":
        return t("contentDetailOverlay.sourceNounRecommendation");
      case "ip-page":
        return t("contentDetailOverlay.sourceNounIpPage");
      default:
        return t("contentDetailOverlay.sourceNounZonePage");
    }
  }

  const returnLabel = previous
    ? previous.title
      ? t("contentDetailOverlay.returnTo", { title: previous.title })
      : sourceReturnLabel(previous.entry.source)
    : top
      ? sourceReturnLabel(top.entry.source)
      : "";

  /* 悬浮返回钮文案（#397 用户裁决格式「返回到：XXX」）：多层栈 = 上一层标题；
     栈底 = 来源入口名词（推荐流/内容列表/IP 详情页/AI 助手）。 */
  const backTooltip = t("contentDetailOverlay.backToTarget", {
    target: previous?.title
      ? previous.title
      : sourceNounLabel((previous ?? top)?.entry.source ?? "zone-page"),
  });

  const topTitle = top?.title ?? "";

  if (depth === 0 && !closing) return null;

  return (
    <>
      {/* 遮罩同钟层：top-layer 语义保证它在 dialog 面板之下、页面之上；
          pointer-events-none——背板点击仍由原生（透明）::backdrop 命中
          dialog 承担。z-[70] 盖过页面 chrome 全部层级（最高 z-[60]）。 */}
      <div
        ref={backdropRef}
        aria-hidden="true"
        className="content-detail-backdrop pointer-events-none fixed inset-0 z-[70]"
        style={{ opacity: 0 }}
      />
      <dialog
      ref={dialogRef}
      className={cn(
        /* 2026-09-09 遮罩同钟配套：dialog 保留桌面居中面板的定位与尺寸，但
           视觉（底色/边框/圆角/阴影）全部随壳层走——入场前壳层 opacity 0 时
           旧快照里不残留白板面板。 */
        "content-detail-overlay fixed inset-0 m-0 h-dvh w-full max-h-none max-w-none overflow-hidden border-0 bg-transparent p-0 text-foreground",
        "lg:m-auto lg:h-[min(92dvh,900px)] lg:w-[min(1120px,calc(100%-2rem))]",
      )}
      aria-labelledby={titleId}
      onCancel={(event) => {
        // SP-17/T2：登录浮窗打开时 Esc 只关浮窗（浮窗自有关闭链路）。
        if (isGateOpen) {
          event.preventDefault();
          return;
        }
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
          "relative grid h-full w-full overflow-hidden bg-card lg:rounded-lg lg:border lg:border-border lg:shadow-[var(--elevation-3)]",
          topIsVariant ? "grid-rows-[minmax(0,1fr)]" : "grid-rows-[auto_minmax(0,1fr)]",
          closing && "pointer-events-none",
        )}
      >
        {/* #397 方案二 float：variant 顶层整个不渲染 header，返回/关闭由壳层悬浮
            圆钮承担（返回钮 hover 显示「返回到：XXX」）；sr-only 标题保留 dialog
            无障碍名称（aria-labelledby）、初始焦点锚点与多层栈返回文案三职。 */}
        {!topIsVariant && (
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
        )}
        {topIsVariant && (
          <>
            <h2
              id={titleId}
              ref={titleRef}
              tabIndex={-1}
              className="sr-only focus:outline-none"
            >
              {topTitle || t("contentDetailOverlay.title")}
            </h2>
            <div className="group/back absolute left-3 top-3 z-20">
              <button
                type="button"
                onClick={handleBack}
                aria-label={backTooltip}
                className="flex h-10 w-10 items-center justify-center rounded-full bg-black/35 text-white backdrop-blur transition-colors hover:bg-black/50 focus:outline-none focus:ring-2 focus:ring-ring"
              >
                <ArrowLeft className="h-5 w-5" aria-hidden="true" />
              </button>
              <span
                role="tooltip"
                className="pointer-events-none absolute left-12 top-1/2 z-30 -translate-y-1/2 whitespace-nowrap rounded-md bg-black/70 px-2.5 py-1 text-xs text-white opacity-0 backdrop-blur transition-opacity duration-150 group-hover/back:opacity-100"
              >
                {backTooltip}
              </span>
            </div>
            <button
              type="button"
              onClick={handleExit}
              aria-label={t("contentDetailOverlay.close")}
              title={t("contentDetailOverlay.close")}
              className="absolute right-4 top-3 z-20 flex h-10 w-10 items-center justify-center rounded-full bg-black/30 text-white backdrop-blur transition-colors hover:bg-black/45 focus:outline-none focus:ring-2 focus:ring-ring"
            >
              <X className="h-5 w-5" aria-hidden="true" />
            </button>
          </>
        )}

        <div
          ref={scrollerRef}
          data-slot="overlay-scroller"
          className={cn(
            "min-h-0 overflow-y-auto overscroll-contain px-4 pb-[max(1.5rem,env(safe-area-inset-bottom))] pt-4 lg:px-6",
            /* #88/#397 桌面双栏与竖屏集新版布局：滚动改由层内信息列承担。 */
            (topLayout === "split-media" || topLayout === "variant") &&
              "min-[1100px]:h-full min-[1100px]:overflow-hidden",
          )}
        >
          {stack.map((layer, index) => (
            <div
              key={`${index}:${layer.entry.contentId}`}
              /* 层推入/弹出的缓动用内联样式：模板字符串拼出的 Tailwind 类不会
                 被 JIT 生成，写进类名等于没写（2026-09-06 实测修复）。 */
              style={
                stackMove && index === depth - 1
                  ? { animationTimingFunction: OVERLAY_EASING }
                  : undefined
              }
              className={cn(
                index === depth - 1
                  ? cn(
                      "h-full",
                      stackMove === "push" &&
                        "animate-in fade-in-0 slide-in-from-right-12 duration-[240ms]",
                      stackMove === "pop" &&
                        "animate-in fade-in-0 slide-in-from-left-12 duration-[240ms]",
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
                /* #409 F1：首帧保持/几何冻结只作用于首层（唯一做共享元素
                   入场转场的层；后续 push/pop 走水平滑动动画）。 */
                motionHoldSrc={index === 0 ? motionHoldSrc : null}
                motionSettled={entranceSettled}
              />
            </div>
          ))}
        </div>
      </div>
    </dialog>
    </>
  );
}
