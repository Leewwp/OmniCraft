"use client";

import { useEffect, useRef, useState } from "react";
import { cn } from "@/lib/utils";
import { OVERLAY_COVER_CROSSFADE_MS } from "@/lib/overlay-motion";

/**
 * 首帧渐进双层次图片（#430，修订 #409 F1「三处同源」契约）：
 *
 * - 保持层（holdSrc = 信息流卡片正在渲染的快变体，必命中缓存）在顶——浮窗
 *   首帧秒出、无 spinner 空窗；入场动效期间它是唯一可见层（零换图契约）。
 * - 规范层（canonicalSrc = w=1080）在底、opacity 0 起异步加载；hold 释放
 *   （入场落定）且规范层解码就绪后，保持层以 180ms 交叉淡出——规范层未就绪
 *   则保持层继续顶着，规范层失败则永久保持（不闪空白）。
 * - 无 holdSrc（无卡片锚点/二次挂载）时仅渲染规范层，行为同旧单层实现。
 *
 * 布局两种形态与旧 <img> 一致：flow（超高图 h-auto 定高在文档流，保持层
 * absolute 覆盖其上）与 fill（两层 absolute inset-0）。
 */
interface HoldCrossfadeImageProps {
  canonicalSrc: string;
  alt: string;
  /** 首帧保持地址（卡片快变体）；null/undefined = 无保持。 */
  holdSrc?: string | null;
  /** 传给两个 <img> 的视觉类（object-fit / cursor 等）。 */
  imgClassName?: string;
  /** 规范层是否在文档流中（超高图）；默认绝对填充。 */
  flowLayout?: boolean;
  /** 可视首帧加载落定/失败回调（保持期由保持层驱动，否则规范层驱动）。 */
  onSettle?: (state: "ready" | "error") => void;
  /** 保持层淡出结束回调（#430 机制级验证锚点）。 */
  onHoldReleased?: () => void;
}

export function HoldCrossfadeImage({
  canonicalSrc,
  alt,
  holdSrc,
  imgClassName,
  flowLayout = false,
  onSettle,
  onHoldReleased,
}: HoldCrossfadeImageProps) {
  /* 记住最近一次非空 holdSrc：释放（null）后保持层仍需渲染到淡出完成。 */
  const lastHoldRef = useRef<string | null>(holdSrc ?? null);
  useEffect(() => {
    if (holdSrc) lastHoldRef.current = holdSrc;
  }, [holdSrc]);

  const [canonicalReady, setCanonicalReady] = useState(false);
  const [holdGone, setHoldGone] = useState(false);

  /* 缓存命中时 React 可能挂上 onLoad 前图片已 complete（不触发 onLoad）——
     ref 回调里补查一次，保底不卡淡出。 */
  const canonicalRef = useRef<HTMLImageElement | null>(null);
  useEffect(() => {
    const el = canonicalRef.current;
    if (el?.complete && el.naturalWidth > 0 && !canonicalReady) setCanonicalReady(true);
  });

  const holding = Boolean(holdSrc);
  const heldUrl = lastHoldRef.current;
  /* 淡出条件：hold 已释放 + 规范层就绪。未释放或规范层未就绪时保持层满不透明。 */
  const fadeOut = !holding && canonicalReady;

  /* 淡出完成门：transitionEnd 为主，定时器兜底——入场 VT 收尾窗口可能把进行中的
     过渡打断为 transitioncancel（不派发 transitionend），保持层将永久滞留。 */
  useEffect(() => {
    if (!fadeOut) return;
    const timer = window.setTimeout(() => setHoldGone(true), OVERLAY_COVER_CROSSFADE_MS + 80);
    return () => window.clearTimeout(timer);
  }, [fadeOut]);

  const settleReady = useRef(false);
  const settleError = useRef(false);
  const handleSettle = (state: "ready" | "error") => {
    if (state === "ready") {
      if (settleReady.current) return;
      settleReady.current = true;
    } else if (settleReady.current || settleError.current) return;
    else settleError.current = true;
    onSettle?.(state);
  };

  const canonicalClass = flowLayout
    ? imgClassName
    : cn("absolute inset-0 h-full w-full", imgClassName);
  const holdClass = cn("absolute inset-0 h-full w-full", imgClassName);

  return (
    <>
      {/* 规范层（底）：动效期间 opacity 0，就绪+释放后承接可视。 */}
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        ref={canonicalRef}
        src={canonicalSrc}
        alt={alt}
        draggable={false}
        className={canonicalClass}
        style={{
          opacity: fadeOut ? 1 : holding ? 0 : 1,
          transition: `opacity ${OVERLAY_COVER_CROSSFADE_MS}ms`,
        }}
        onLoad={() => {
          setCanonicalReady(true);
          if (!heldUrl) handleSettle("ready");
        }}
        onError={() => {
          if (!heldUrl) handleSettle("error");
        }}
      />
      {/* 保持层（顶）：首帧唯一可见层；释放且规范就绪后淡出并卸载。
          data-slot="cover-hold" = 机制级验证锚点（backdrop-sync-verify 扩展）。 */}
      {heldUrl && !holdGone && (
        /* eslint-disable-next-line @next/next/no-img-element */
        <img
          src={heldUrl}
          alt=""
          aria-hidden="true"
          data-slot="cover-hold"
          draggable={false}
          className={holdClass}
          style={{
            opacity: fadeOut ? 0 : 1,
            transition: `opacity ${OVERLAY_COVER_CROSSFADE_MS}ms`,
          }}
          onLoad={() => handleSettle("ready")}
          onError={() => handleSettle("error")}
          onTransitionEnd={(event) => {
            if (event.propertyName !== "opacity" || !fadeOut) return;
            setHoldGone(true);
            onHoldReleased?.();
          }}
        />
      )}
    </>
  );
}
