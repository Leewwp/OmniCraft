"use client";

import { useCallback, useEffect, useId, useRef, useState, type CSSProperties, type RefObject } from "react";
import { useTranslations } from "next-intl";
import { ArrowLeft, X } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  OVERLAY_CARD_COVER_SLOT,
  OVERLAY_COVER_SLOT,
  OVERLAY_MOTION,
  OVERLAY_STAGE_SLOT,
  computeShellMorph,
  measureSourceRect,
  nextFrame,
  readElementRect,
  rectHasArea,
  reducedMotionEnabled,
  selectMotionPath,
  shellMorphOriginToCss,
  shellMorphToCss,
  type OverlayRect,
} from "@/lib/overlay-motion";
import {
  ContentDetailOverlayLayer,
  type OverlayEntry,
  type OverlaySource,
} from "./ContentDetailOverlayLayer";
import {
  MAX_OVERLAY_STACK_DEPTH,
  abortOverlayHistorySession,
  beginOverlayHistorySession,
  endOverlayHistorySession,
  pushOverlayHistoryLayer,
  replaceOverlayHistoryTop,
  requestOverlayHistoryBack,
  requestOverlayHistoryExit,
  subscribeOverlayHistory,
  type OverlayHistoryRecord,
} from "@/lib/overlay-history";
import { useAuthGate } from "@/components/auth/AuthGateProvider";
import { useOverlayViewport } from "@/components/content/use-overlay-viewport";
import { OVERLAY_GEOMETRY_TRANSITION } from "@/lib/overlay-geometry";

const OVERLAY_EASING = "cubic-bezier(0.22,0.61,0.36,1)";
/** 入场转场等待层数据的保险时限：超时按降级路径淡入，避免不可见卡死。 */
const ENTRANCE_SAFETY_MS = 2000;

/** D3 #860：history 记录 → 浮层条目（forward 重建层/换篇对齐用）。 */
function entryFromRecord(record: OverlayHistoryRecord): OverlayEntry {
  return {
    contentId: record.contentId,
    zone: record.zone,
    source: record.source,
    contextList: record.contextList,
    contextIndex: record.contextIndex,
  };
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
  /* D1 #858：统一版式壳层由 twoColumn（≥960 视口，C2 #921 useOverlayViewport）
     决定——桌面 = 统一双栏壳（无 header 工具栏、返回/关闭悬浮半透明圆钮、
     滚动归属层内 layer-scroller），从 loading 首帧生效；<960 保持移动单列 +
     header 工具栏。浮层为纯客户端挂载无水合路径，uSES 首帧直接走 getSnapshot
     （同步读 matchMedia，与旧惰性初始化语义等价）；SSR/测试缺 matchMedia
     按移动壳层处理。 */
  const { twoColumn } = useOverlayViewport();
  /* #409 F1 单一时间轴配套：入场转场落定标记（驱动首帧保持解除）+ 首帧保持
     src（打开瞬间卡片封面实际渲染的地址）。 */
  const [entranceSettled, setEntranceSettled] = useState(false);
  const [motionHoldSrc, setMotionHoldSrc] = useState<string | null>(null);
  /* E1 #953 内容推导壳宽：顶层活动层媒体链就绪后上报的 dialog 宽（媒体列 +
     固定右列，经 CSS 变量喂给 min-[960px]:w-[var(--overlay-shell-width,…)]；
     未上报前回退 fit-content（媒体列 px + 右列 440 的内容宽，loading/错误态
     不留右侧空带）。跨内容/档位切换时值变化 → 240ms 宽度过渡。 */
  const [shellWidthPx, setShellWidthPx] = useState<number | null>(null);
  const handleShellWidthChange = useCallback((px: number) => {
    setShellWidthPx((prev) => (prev === px ? prev : px));
  }, []);
  /* E2 #954 关窗回源编排：exitResetToken 递增 = 顶层舞台瞬时回源（slide 0 +
     档位/宽度无过渡，供 morphBack 在稳定几何上测量）；shellWidthSnap 关窗期
     关闭 dialog 宽度过渡（宽度跳变被收缩运动掩盖，不再走 240ms 档）。 */
  const [exitResetToken, setExitResetToken] = useState(0);
  const [shellWidthSnap, setShellWidthSnap] = useState(false);

  const stackRef = useRef<OverlayLayerState[]>(stack);
  const popFocusRef = useRef<HTMLElement | null>(popFocus);
  const closingRef = useRef(false);
  const closeTimerRef = useRef<number | null>(null);
  const restoreRef = useRef<{ trigger: HTMLElement | null; windowY: number } | null>(null);
  const lastOpenRef = useRef(false);
  const topKeyRef = useRef<string | null>(null);
  const onOpenChangeRef = useRef(onOpenChange);
  const twoColumnRef = useRef(twoColumn);
  /* D3 #860：当前浮层 history 会话 id（卸载兜底 abort 用）。 */
  const historySessionRef = useRef<string | null>(null);

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
    twoColumnRef.current = twoColumn;
  }, [twoColumn]);

  /* 转场状态（#67 原型 §5 契约）：run token 拦截过期回调；入场只跑一次。 */
  const motionRunRef = useRef(0);
  const motionTimerRef = useRef<number | null>(null);
  const safetyTimerRef = useRef<number | null>(null);
  const entranceDoneRef = useRef(false);
  const sourceRectRef = useRef<OverlayRect | null>(null);
  const sourceAnchorRef = useRef<HTMLElement | null>(null);

  /* 当前唯一滚动容器：D1 #858 统一版式（桌面 ≥960 全状态双栏）时取顶层可见层
     的 layer-scroller（跳过 display:none 的底层），移动回到 overlay-scroller。 */
  const resolveActiveScroller = useCallback((): HTMLElement | null => {
    const scroller = scrollerRef.current;
    if (!scroller) return null;
    if (!twoColumnRef.current) return scroller;
    const candidates = scroller.querySelectorAll<HTMLElement>('[data-slot="layer-scroller"]');
    for (let i = candidates.length - 1; i >= 0; i -= 1) {
      if (candidates[i].offsetParent !== null) return candidates[i];
    }
    return scroller;
  }, []);

  /* 打开/关闭契约：open 翻转时初始化首层（保存触发元素、页面滚动与 source 几何），
     关闭时走 finalizeClose（幂等）。source 测量必须先于任何布局变更（滚动锁、
     overlay 插入），见原型 §4.3。
     #398 C3：打开瞬间锁定触发卡片的 hover 缩放（globals.css 以
     [data-overlay-motion-lock] 归位），VT 旧快照捕获时 <img> 处于未变换位姿；
     finalizeClose 移除锁。
     D3 #860：打开 = 开启 history 会话并 pushState 到首层规范路径
     （fanwork=/content/<id>、original=/original/<id>，spec §9.4 转移表）；
     StrictMode dev 双跑由模块的复活窗口吸收（不双写记录）。 */
  useEffect(() => {
    /* D3 #860：`!lastOpenRef.current` = 正常打开；`historySessionRef.current === null`
       = StrictMode dev 卸载→重挂（挂载清理 abort 掉了 history 会话）——复用模块
       的复活窗口重新 begin（同签名 + 当前记录仍是本会话 depth-1 时复活同一
       会话，不双写历史记录）。 */
    if (open && (!lastOpenRef.current || historySessionRef.current === null)) {
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
      /* E1 #953：新开浮窗壳宽清零回退 fit-content，顶层链就绪后按内容推导上报
         （起跑前壳层 opacity 0，宽度跳变不可见）。
         E2 #954：关窗编排状态复位——在途关窗定时器作废（快速关-开不误杀新
         会话）、壳 morph 残留 transform/origin 清零、宽度过渡恢复常轨。 */
      setShellWidthPx(null);
      if (closeTimerRef.current !== null) {
        window.clearTimeout(closeTimerRef.current);
        closeTimerRef.current = null;
      }
      setShellWidthSnap(false);
      setExitResetToken(0);
      const shellReset = shellRef.current;
      if (shellReset) {
        shellReset.style.transition = "none";
        shellReset.style.transform = "none";
        shellReset.style.transformOrigin = "";
        shellReset.style.removeProperty("will-change");
      }
      trigger?.setAttribute("data-overlay-motion-lock", "");
      restoreRef.current = { trigger, windowY: window.scrollY };
      setStack([
        { entry: { contentId, zone, source, contextList, contextIndex }, trigger, scrollTop: 0, title: null },
      ]);
      historySessionRef.current = beginOverlayHistorySession({
        contentId,
        zone,
        source,
        contextList,
        contextIndex,
      });
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
      /* D3 #860：未经关闭动效的卸载（宿主页真实跳转）也要终结 history 会话，
         死记录交给模块的规范宿主页接管；finalizeClose 已终结时为幂等 no-op。 */
      if (historySessionRef.current) {
        abortOverlayHistorySession(historySessionRef.current);
        historySessionRef.current = null;
      }
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

  /* 压栈（浮层内关联内容）。D3 #860：React 栈与 history 记录同进同退——
     五层封顶时模块拒绝写第六条记录，React 栈同样不再压入。 */
  const pushLayer = useCallback((entry: OverlayEntry, trigger: HTMLElement | null) => {
    if (closingRef.current || stackRef.current.length === 0) return;
    if (stackRef.current.length >= MAX_OVERLAY_STACK_DEPTH) return;
    if (!pushOverlayHistoryLayer(entry)) return;
    const current = stackRef.current;
    const activeScroller = resolveActiveScroller();
    const next = current.map((layer, index) =>
      index === current.length - 1 ? { ...layer, scrollTop: activeScroller?.scrollTop ?? 0 } : layer,
    );
    next.push({ entry, trigger, scrollTop: 0, title: null });
    setStackMove("push");
    setStack(next);
  }, [resolveActiveScroller]);

  /* #89 连续浏览：原地替换顶层为上下文列表下一篇（不压栈）。D3 #860：换篇
     同步 replaceState——顶层记录与 URL 原地改写为新内容规范路径，不新增历史。 */
  const switchTopLayer = useCallback((nextEntry: OverlayEntry) => {
    if (closingRef.current || stackRef.current.length === 0) return;
    replaceOverlayHistoryTop(nextEntry);
    setStack((prev) =>
      prev.map((layer, index) =>
        index === prev.length - 1
          ? { ...layer, entry: nextEntry, scrollTop: 0, title: null }
          : layer,
      ),
    );
  }, []);

  /* ---------- 开合转场（E2 #954：整容器封面 morph / morphBack 编排） ---------- */

  const getTopCover = useCallback((): HTMLElement | null => {
    const scroller = scrollerRef.current;
    if (!scroller) return null;
    const covers = scroller.querySelectorAll<HTMLElement>(`[data-slot="${OVERLAY_COVER_SLOT}"]`);
    /* C2 #921 单挂载后桌面变体态行内媒体区不再渲染（唯一锚点 = 左栏媒体列）；
       逐锚点跳过 display:none 的隐藏实例仍是 data-slot 缝的防御语义——任何
       隐藏 detail-cover 锚点的回归都被兜住。 */
    for (let i = covers.length - 1; i >= 0; i -= 1) {
      if (covers[i].offsetParent !== null || covers[i].getClientRects().length > 0) {
        return covers[i];
      }
    }
    return null;
  }, []);

  /* E2 #954 morph 锚点 = 媒体舞台（桌面统一版式 variant-media-pane，E1 后贴
     壳左缘 → origin ≈ 0 0 即舞台左上角，右列随行）；移动单列/兜底回退封面锚点。 */
  const getStageAnchor = useCallback((): HTMLElement | null => {
    const scroller = scrollerRef.current;
    if (scroller) {
      const panes = scroller.querySelectorAll<HTMLElement>(`[data-slot="${OVERLAY_STAGE_SLOT}"]`);
      for (let i = panes.length - 1; i >= 0; i -= 1) {
        if (panes[i].offsetParent !== null || panes[i].getClientRects().length > 0) {
          return panes[i];
        }
      }
    }
    return getTopCover();
  }, [getTopCover]);

  /* 中断处理：run token 递增使所有在途回调过期；清过渡、清定时器、清壳层
     morph 残留（opacity 由各动效路径显式设定，不在此清——入场前的 0 不能被
     中断洗成可见）。 */
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
      shell.style.transformOrigin = "";
      shell.style.removeProperty("will-change");
    }
  }, []);

  /* #409 F1 入场落定：单一时间轴动画全部结束后标记——解除首帧保持
     （媒体链不同文件时此刻才切换）与媒体列几何冻结。 */
  const markEntranceSettled = useCallback((token: number) => {
    if (token !== motionRunRef.current) return;
    setEntranceSettled(true);
  }, []);

  /* 不可定位降级：居中 scale(0.96) + 淡化（开/关 400ms ease，与 morph 同口径；
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
          `opacity ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.morphEasing}, ` +
          `transform ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.morphEasing}`;
        shell.style.opacity = "1";
        shell.style.transform = "none";
        const backdrop = backdropRef.current;
        if (backdrop) {
          backdrop.style.transition = `opacity ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.morphEasing}`;
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

  /* E2 #954 整容器 morph 开：起点帧 = 壳瞬移到卡片位（等比 scale = 封面宽 ÷
     舞台宽，origin 锚舞台左上角，transition none + 整壳置不透明——frame 0
     实心壳恰好落在来源卡片正上方、舞台与封面像素重合，出场闪变被卡片本体
     掩盖）→ 双 rAF 确保绘制 → transform 收敛到 identity，400ms ease（小红书
     实测），右列随行。超高图首项走 fallback（C2：卡片 400px 上限 contain 与
     舞台 contain 取景无法统一）。 */
  const runMorphOpen = useCallback(
    (token: number, shell: HTMLElement) => {
      const sourceRect = sourceRectRef.current;
      const anchor = getStageAnchor();
      const stageRect = anchor ? readElementRect(anchor) : null;
      if (
        !sourceRect ||
        !anchor ||
        !stageRect ||
        !rectHasArea(stageRect) ||
        anchor.dataset.ultraTall === "true"
      ) {
        runFallbackOpen(token, shell);
        return;
      }
      const morph = computeShellMorph(sourceRect, stageRect, readElementRect(shell));
      shell.style.transition = "none";
      shell.style.transformOrigin = shellMorphOriginToCss(morph);
      shell.style.transform = shellMorphToCss(morph);
      shell.style.opacity = "1";
      void nextFrame().then(() => {
        if (token !== motionRunRef.current) return;
        shell.style.willChange = "transform";
        shell.style.transition = `transform ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.morphEasing}`;
        shell.style.transform = "none";
        const backdrop = backdropRef.current;
        if (backdrop) {
          backdrop.style.transition = `opacity ${OVERLAY_MOTION.openDuration}ms ${OVERLAY_MOTION.morphEasing}`;
          backdrop.style.opacity = "1";
        }
        motionTimerRef.current = window.setTimeout(() => {
          if (token !== motionRunRef.current) return;
          shell.style.transition = "";
          shell.style.transform = "";
          shell.style.transformOrigin = "";
          shell.style.removeProperty("will-change");
          markEntranceSettled(token);
        }, OVERLAY_MOTION.openDuration + 40);
      });
    },
    [getStageAnchor, markEntranceSettled, runFallbackOpen],
  );

  /* 入场：reducedMotion() ? fade : (!sourceRect || 舞台不可测 ? fallback : morph)。
     由顶层 onMotionReady 触发（层数据落定 + 几何实测即 motionGate 满足），每次
     打开只跑一次。E2 #954 媒体就绪门退场：不再等封面 decode（封面由 holdSrc/
     预加载链路自然呈现，就绪交换交由 HoldCrossfadeImage），入场转场在数据+
     几何就绪即起跑；ENTRANCE_SAFETY_MS 保险兜底数据悬挂。 */
  const runEntranceMotion = useCallback(() => {
    if (entranceDoneRef.current) return;
    entranceDoneRef.current = true;
    cancelActiveMotion();
    const token = motionRunRef.current;
    const shell = shellRef.current;
    if (!shell) return;

    const path = selectMotionPath(reducedMotionEnabled(), sourceRectRef.current);
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
    runMorphOpen(token, shell);
  }, [cancelActiveMotion, markEntranceSettled, runFallbackOpen, runMorphOpen]);

  const handleMotionReady = useCallback(() => {
    if (entranceDoneRef.current || closingRef.current) return;
    runEntranceMotion();
  }, [runEntranceMotion]);

  /* 完全退出后的恢复契约：还原触发入口、页面滚动位置与焦点。
     D3 #860：终结 history 会话（× 途径 popstate 已终结时幂等）。 */
  const finalizeClose = useCallback(() => {
    if (historySessionRef.current) {
      endOverlayHistorySession();
      historySessionRef.current = null;
    }
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

  /* E2 #954 退场（morphBack 编排，镜像开窗的完整编排）：关闭时重新测量
     source（用户可能已滚动/虚拟化卸载，原型 §4.4）+ 舞台（已在关窗开始时
     瞬时回源）。可测 → morphBack：identity → 卡片位一次到位（独立反向，
     不复用开窗 FLIP——先瞬移再放大回来会造成二次闪现）+ 整壳 transform 与
     opacity 同一 transition 400ms ease（溶解式落地，落地即透明、隐藏帧零
     可见变化）+ 右列/舞台 chrome 300ms 同步淡出（globals.css data-closing）
     + 遮罩同程淡出；不可测 → 居中缩淡；reduced-motion 100ms 纯 opacity。 */
  const runCloseMotion = useCallback(() => {
    if (!closingRef.current) return;
    const shell = shellRef.current;
    if (!shell) {
      closeTimerRef.current = window.setTimeout(finalizeClose, 0);
      return;
    }
    const backdrop = backdropRef.current;
    const finish = (delay: number) => {
      /* 关窗定时器总是收尾（重开路径在打开瞬间已 clearTimeout 本定时器，
         不存在误杀新会话的窗口）。 */
      closeTimerRef.current = window.setTimeout(finalizeClose, delay);
    };
    const runFallbackClose = () => {
      shell.style.willChange = "opacity, transform";
      shell.style.transition =
        `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.morphEasing}, ` +
        `transform ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.morphEasing}`;
      shell.style.opacity = "0";
      shell.style.transform = `scale(${OVERLAY_MOTION.fallbackScale})`;
      if (backdrop) {
        backdrop.style.transition = `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.morphEasing}`;
        backdrop.style.opacity = "0";
      }
      finish(OVERLAY_MOTION.closeHideDelayMs);
    };

    const sourceRect = measureSourceRect(restoreRef.current?.trigger ?? null);
    const path = selectMotionPath(reducedMotionEnabled(), sourceRect);
    if (path === "fade") {
      shell.style.transition = `opacity ${OVERLAY_MOTION.reducedDuration}ms ease-out`;
      shell.style.opacity = "0";
      if (backdrop) {
        backdrop.style.transition = `opacity ${OVERLAY_MOTION.reducedDuration}ms ease-out`;
        backdrop.style.opacity = "0";
      }
      finish(OVERLAY_MOTION.reducedDuration);
      return;
    }
    if (path === "fallback") {
      runFallbackClose();
      return;
    }
    const anchor = getStageAnchor();
    const stageRect = anchor ? readElementRect(anchor) : null;
    /* 超高图当前项同样退化为居中缩淡（与开路径同因，见 runMorphOpen）。 */
    if (
      !sourceRect ||
      !anchor ||
      !stageRect ||
      !rectHasArea(stageRect) ||
      anchor.dataset.ultraTall === "true"
    ) {
      runFallbackClose();
      return;
    }
    const morph = computeShellMorph(sourceRect, stageRect, readElementRect(shell));
    shell.style.willChange = "transform, opacity";
    shell.style.transformOrigin = shellMorphOriginToCss(morph);
    shell.style.transition =
      `transform ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.morphEasing}, ` +
      `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.morphEasing}`;
    shell.style.transform = shellMorphToCss(morph);
    shell.style.opacity = "0";
    if (backdrop) {
      backdrop.style.transition = `opacity ${OVERLAY_MOTION.closeDuration}ms ${OVERLAY_MOTION.morphEasing}`;
      backdrop.style.opacity = "0";
    }
    finish(OVERLAY_MOTION.closeHideDelayMs);
  }, [finalizeClose, getStageAnchor]);

  /* 退出整个浮层（X / 背板 / history exit）：关窗开始第 0ms 即解除滚动锁与
     让位补偿（原型规范第 10 条——复位被收缩运动掩盖，等动画结束才解锁会被
     感知为页面闪跳）+ 顶层舞台瞬时回源（exitResetToken，morphBack 测量稳定），
     关闭动效由 closing 效应在回源重置提交后再过两帧起跑。
     D3 #860：× 一次 history.go 退回会话来源记录（back 在途时先记全退意图，
     由模块在 popstate 补齐剩余步数，Esc→× 连击不越过来源）。 */
  const beginExit = useCallback(() => {
    if (closingRef.current || stackRef.current.length === 0) return;
    closingRef.current = true;
    setClosing(true);
    cancelActiveMotion();
    document.documentElement.style.overflow = "";
    document.body.style.overflow = "";
    document.body.style.paddingRight = "";
    setShellWidthSnap(true);
    setExitResetToken((token) => token + 1);
  }, [cancelActiveMotion]);

  /* 关窗编排时序：closing 提交（舞台回源重置已随之提交）→ 两帧后几何稳定 →
     runCloseMotion 测量并起跑 morphBack；隐藏（finalizeClose）贴动画结束
     （≈415ms），落地前整壳已透明。 */
  useEffect(() => {
    if (!closing) return;
    let cancelled = false;
    const outer = window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => {
        if (!cancelled) runCloseMotion();
      });
    });
    return () => {
      cancelled = true;
      window.cancelAnimationFrame(outer);
    };
  }, [closing, runCloseMotion]);

  const handleExit = useCallback(() => {
    if (closingRef.current || stackRef.current.length === 0) return;
    beginExit();
    requestOverlayHistoryExit();
  }, [beginExit]);

  /* 返回按钮 / Esc（D3 #860）：统一经 history.back 与浏览器返回归并到同一个
     popstate——栈中退一层、栈底退回来源记录并关闭，路径全部由模块的
     popstate 对齐逻辑裁决（支持 go(-n)、快速连按、动画重入）。 */
  const handleBack = useCallback(() => {
    if (closingRef.current || stackRef.current.length === 0) return;
    requestOverlayHistoryBack();
  }, []);

  /* D3 #860：popstate 唯一归并点（lib/overlay-history 控制器分发）。
     sync = 按目标记录深度绝对对齐 React 栈（返回逐层/多步、forward 重建层、
     换篇后的内容一致性）；exit = 落回来源记录，执行关闭动效收尾。 */
  useEffect(() => {
    return subscribeOverlayHistory((event) => {
      if (event.type === "exit") {
        if (closingRef.current || stackRef.current.length === 0) return;
        beginExit();
        return;
      }
      if (closingRef.current || stackRef.current.length === 0) return;
      const current = stackRef.current;
      const records = event.records;
      const targetDepth = records.length;
      if (targetDepth === current.length) {
        /* 深度不变但顶层记录换过内容（连续换篇后 forward/back 命中）：对齐。 */
        const topRecord = records[targetDepth - 1];
        if (topRecord && topRecord.contentId !== current[current.length - 1].entry.contentId) {
          setStack((prev) =>
            prev.map((layer, index) =>
              index === prev.length - 1
                ? { ...layer, entry: entryFromRecord(topRecord), scrollTop: 0, title: null }
                : layer,
            ),
          );
        }
        return;
      }
      if (targetDepth < current.length) {
        const popped = current[current.length - 1];
        setPopFocus(popped.trigger);
        setStackMove("pop");
        setStack(current.slice(0, targetDepth));
        return;
      }
      /* forward 落在更深的会话记录：下层仍存活时按记录重建缺失层（组件重挂
         后自行拉数据，地址与可见内容一致）。 */
      const rebuilt = records.slice(current.length).map((record) => ({
        entry: entryFromRecord(record),
        trigger: null,
        scrollTop: 0,
        title: null,
      }));
      setStackMove("push");
      setStack([...current, ...rebuilt]);
    });
  }, [beginExit]);

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
           旧快照里不残留白板面板。
           D1 #858：面板断点与内部统一双栏断点对齐（960–1023 也收居中面板，
           修「全屏面板 + 内部已双栏」的几何缺口）。
           E1 #953：桌面宽 = 内容推导（顶层层上报的 --overlay-shell-width：
           媒体列 + 固定右列 440，上限 min(1320, 视口 − 2rem)）；未上报前
           fit-content（媒体列 px + 右列的内容宽），不再固定 1120。移动
           <960 不读该变量（全屏单列路径不受影响）。 */
        "content-detail-overlay fixed inset-0 m-0 h-dvh w-full max-h-none max-w-none overflow-hidden border-0 bg-transparent p-0 text-foreground",
        "min-[960px]:m-auto min-[960px]:h-[min(92dvh,900px)] min-[960px]:w-[var(--overlay-shell-width,fit-content)] min-[960px]:max-w-[calc(100%-2rem)]",
      )}
      style={
        {
          ...(shellWidthPx != null ? { "--overlay-shell-width": `${shellWidthPx}px` } : null),
          /* 档位切换/跨内容的宽度过渡与媒体列同程（SP-12 缓动口径）；移动端
             w-full 恒定值不触发。E2 #954：关窗回源帧关闭过渡（shellWidthSnap，
             宽度瞬时对齐防 morphBack 测量漂移），其余路径照常 240ms。 */
          transition: shellWidthSnap ? "none" : OVERLAY_GEOMETRY_TRANSITION,
        } as CSSProperties
      }
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
        data-closing={closing ? "" : undefined}
        className={cn(
          "relative grid h-full w-full overflow-hidden bg-card min-[960px]:rounded-lg min-[960px]:border min-[960px]:border-border min-[960px]:shadow-[var(--elevation-3)]",
          twoColumn ? "grid-rows-[minmax(0,1fr)]" : "grid-rows-[auto_minmax(0,1fr)]",
          closing && "pointer-events-none",
        )}
      >
        {/* D1 #858 统一版式壳层：桌面 ≥960 整个不渲染 header（grid 单行），
            返回/关闭由壳层悬浮圆钮承担（返回钮 hover 显示「返回到：XXX」）；
            sr-only 标题保留 dialog 无障碍名称（aria-labelledby）、初始焦点锚点
            与多层栈返回文案三职。移动 <960 保持 header 工具栏。 */}
        {!twoColumn && (
          <header className="flex items-center gap-2 border-b border-border bg-card px-3 pb-2 pt-[max(0.5rem,env(safe-area-inset-top))] lg:px-4 lg:pb-2.5">
            <button
              type="button"
              onClick={handleBack}
              aria-label={returnLabel}
              className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:outline-none focus-visible:ring-1 focus:ring-ring"
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
              className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:outline-none focus-visible:ring-1 focus:ring-ring"
            >
              <X className="h-4 w-4" aria-hidden="true" />
            </button>
          </header>
        )}
        {twoColumn && (
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
                data-slot="overlay-back-button"
                onClick={handleBack}
                aria-label={backTooltip}
                className="flex h-10 w-10 items-center justify-center rounded-full bg-black/35 text-white backdrop-blur transition-colors hover:bg-black/50 focus:outline-none focus-visible:ring-1 focus:ring-ring"
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
              data-slot="overlay-close-button"
              onClick={handleExit}
              aria-label={t("contentDetailOverlay.close")}
              title={t("contentDetailOverlay.close")}
              className="absolute right-4 top-3 z-20 flex h-10 w-10 items-center justify-center rounded-full bg-black/30 text-white backdrop-blur transition-colors hover:bg-black/45 focus:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            >
              <X className="h-5 w-5" aria-hidden="true" />
            </button>
          </>
        )}

        <div
          ref={scrollerRef}
          data-slot="overlay-scroller"
          className={cn(
            "min-h-0 overflow-y-auto overscroll-contain px-4 pb-[max(1.5rem,env(safe-area-inset-bottom))] pt-4",
            /* D1 #858 统一版式：桌面全状态滚动归属层内 layer-scroller，外壳容器
               零内边距、不滚动——统一版式直贴面板，无需负 margin 抵消（旧的
               overlay-scroller px/pt/pb 与 variant root -mx/-mb/-mt 相互抵消的
               结构退役）。 */
            twoColumn && "h-full overflow-hidden p-0",
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
                onPush={pushLayer}
                onSwitchNext={switchTopLayer}
                onTitleChange={handleTitleChange(index)}
                onMotionReady={handleMotionReady}
                /* #409 F1：首帧保持只作用于首层（唯一做共享元素入场转场的层；
                   后续 push/pop 走水平滑动动画）。E1 #953：壳宽跟随顶层活动层
                   的内容推导规格（几何上报在布局侧以 active 门控）。 */
                motionHoldSrc={index === 0 ? motionHoldSrc : null}
                onShellWidthChange={handleShellWidthChange}
                /* E2 #954：关窗回源令牌只发给顶层（隐藏层回源无意义）。 */
                exitResetToken={index === depth - 1 ? exitResetToken : 0}
                /* D2 #859：键盘翻页/媒体挂起矩阵只作用于顶层活动层
                   （关闭中的层即将卸载，同样不算活动）。 */
                active={index === depth - 1 && !closing}
              />
            </div>
          ))}
        </div>
      </div>
    </dialog>
    </>
  );
}
