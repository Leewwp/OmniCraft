"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslations } from "next-intl";
import { ChevronLeft, ChevronRight, ImageOff, Minus, Plus, RotateCcw, X } from "lucide-react";
import { cn } from "@/lib/utils";
import type { MediaGalleryItem } from "@/components/content/MediaGallery";

interface MediaViewerProps {
  className?: string;
  items: MediaGalleryItem[];
  index: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** 媒体集内翻页（上一张/下一张/滑动）时同步索引，不参与内容级切换。 */
  onIndexChange?: (index: number) => void;
}

/** 缩放范围与步进（按钮 ×1.5 / 滚轮 ×1.2，双击与重置按钮归位）。 */
const ZOOM_MIN = 1;
const ZOOM_MAX = 5;
const ZOOM_BUTTON_STEP = 1.5;
const WHEEL_ZOOM_FACTOR = 1.2;
/** 触摸滑动翻页位移阈值（px）。 */
const SWIPE_THRESHOLD = 40;
/** 视为「点击」而非拖拽的最大位移（px），防止手势结束误触发背板关闭。 */
const CLICK_MOVE_THRESHOLD = 10;

function clampIndex(value: number, length: number): number {
  if (!Number.isFinite(value) || length <= 0) return 0;
  return Math.min(Math.max(Math.floor(value), 0), length - 1);
}

function clampPanToStage(
  pan: { x: number; y: number },
  zoom: number,
  stageWidth: number,
  stageHeight: number,
): { x: number; y: number } {
  if (zoom <= 1) return { x: 0, y: 0 };
  // 布局未测量（jsdom/首帧）时不做钳制，由真实布局接管。
  if (stageWidth <= 0 || stageHeight <= 0) return pan;
  const maxX = (stageWidth * (zoom - 1)) / 2 + stageWidth / 4;
  const maxY = (stageHeight * (zoom - 1)) / 2 + stageHeight / 4;
  return {
    x: Math.min(maxX, Math.max(-maxX, pan.x)),
    y: Math.min(maxY, Math.max(-maxY, pan.y)),
  };
}

/**
 * 全屏媒体查看器（#86）：规范化既有「图片预览」语义，可叠加在内容详情
 * 浮层最上层。经 createPortal 挂到 document.body 以脱离浮层 FLIP transform
 * 祖先的约束；再以原生 <dialog> + showModal() 进入浏览器 top layer，保证
 * 真正渲染在详情浮层（同为原生 dialog）之上——普通 z-index 无法压过
 * top layer（ui-spec：fixed inset-0 z-[60] 高于 Overlay）。
 */
export function MediaViewer({
  className,
  items,
  index,
  open,
  onOpenChange,
  onIndexChange,
}: MediaViewerProps) {
  const t = useTranslations();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const stageRef = useRef<HTMLDivElement>(null);
  const [zoom, setZoom] = useState(ZOOM_MIN);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const [failed, setFailed] = useState<Record<number, boolean>>({});
  const zoomRef = useRef(ZOOM_MIN);
  const pointersRef = useRef<Map<number, { x: number; y: number }>>(new Map());
  const pinchRef = useRef<{ distance: number; zoom: number } | null>(null);
  const dragStartRef = useRef<{ x: number; y: number; moved: boolean; onStage: boolean } | null>(null);

  const currentIndex = clampIndex(index, items.length);
  const current = items[currentIndex];
  const isImage = current?.type === "image";
  const showPaging = items.length > 1;

  const updateZoom = useCallback((next: number) => {
    const clamped = Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, next));
    zoomRef.current = clamped;
    setZoom(clamped);
    const stage = stageRef.current;
    setPan((prev) =>
      clampPanToStage(prev, clamped, stage?.clientWidth ?? 0, stage?.clientHeight ?? 0),
    );
  }, []);

  // 进入/关闭生命周期：仅渲染时挂载 dialog，showModal 进入 top layer。
  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog || dialog.open) return;
    dialog.showModal();
  }, [open]);

  // Esc 自包含：capture 阶段 + stopPropagation + preventDefault，不得传播到
  // 下层任何处理者。preventDefault 关键：查看器在 capture 阶段同步关闭后，
  // 浏览器对同一 keydown 的 dialog-cancel 默认动作会落到新的最顶层 dialog
  // （详情浮层），preventDefault 阻止该默认动作，浮层不会被连带关闭。
  useEffect(() => {
    if (!open) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.stopPropagation();
      event.preventDefault();
      onOpenChange(false);
    };
    window.addEventListener("keydown", handleKeyDown, { capture: true });
    return () => window.removeEventListener("keydown", handleKeyDown, { capture: true });
  }, [open, onOpenChange]);

  // 全屏层打开期间锁定背景滚动，关闭时恢复打开前的值（浮层自身锁不受影响）。
  useEffect(() => {
    if (!open) return;
    const htmlOverflow = document.documentElement.style.overflow;
    const bodyOverflow = document.body.style.overflow;
    document.documentElement.style.overflow = "hidden";
    document.body.style.overflow = "hidden";
    return () => {
      document.documentElement.style.overflow = htmlOverflow;
      document.body.style.overflow = bodyOverflow;
    };
  }, [open]);

  // 翻页后复位缩放与平移（视频/图片一致）。
  useEffect(() => {
    updateZoom(ZOOM_MIN);
  }, [index, updateZoom]);

  // 滚轮缩放：React 事件系统对 wheel 以 passive 注册，preventDefault 不生效，
  // 必须用原生非 passive 监听（视频项不缩放、不拦截）。
  useEffect(() => {
    const stage = stageRef.current;
    if (!stage || !open || !isImage) return;
    const handleWheel = (event: WheelEvent) => {
      event.preventDefault();
      const factor = event.deltaY < 0 ? WHEEL_ZOOM_FACTOR : 1 / WHEEL_ZOOM_FACTOR;
      updateZoom(zoomRef.current * factor);
    };
    stage.addEventListener("wheel", handleWheel as EventListener, {
      passive: false,
    } as AddEventListenerOptions);
    return () => stage.removeEventListener("wheel", handleWheel as EventListener);
  }, [open, isImage, updateZoom]);

  const goNext = useCallback(() => {
    if (currentIndex >= items.length - 1) return;
    updateZoom(ZOOM_MIN);
    onIndexChange?.(currentIndex + 1);
  }, [currentIndex, items.length, onIndexChange, updateZoom]);

  const goPrevious = useCallback(() => {
    if (currentIndex <= 0) return;
    updateZoom(ZOOM_MIN);
    onIndexChange?.(currentIndex - 1);
  }, [currentIndex, onIndexChange, updateZoom]);

  function handlePointerDown(event: React.PointerEvent<HTMLDivElement>) {
    event.currentTarget.setPointerCapture?.(event.pointerId);
    pointersRef.current.set(event.pointerId, { x: event.clientX, y: event.clientY });
    if (pointersRef.current.size === 2) {
      const [a, b] = [...pointersRef.current.values()];
      pinchRef.current = { distance: Math.hypot(a.x - b.x, a.y - b.y), zoom: zoomRef.current };
      dragStartRef.current = null;
      return;
    }
    dragStartRef.current = {
      x: event.clientX,
      y: event.clientY,
      moved: false,
      onStage: event.target === event.currentTarget,
    };
  }

  function handlePointerMove(event: React.PointerEvent<HTMLDivElement>) {
    const previous = pointersRef.current.get(event.pointerId);
    if (!previous) return;
    const dx = event.clientX - previous.x;
    const dy = event.clientY - previous.y;
    pointersRef.current.set(event.pointerId, { x: event.clientX, y: event.clientY });

    // 手势优先级：缩放 > 平移 > 翻页。双指捏合只在图片项生效。
    if (pointersRef.current.size === 2 && pinchRef.current) {
      const [a, b] = [...pointersRef.current.values()];
      const distance = Math.hypot(a.x - b.x, a.y - b.y);
      updateZoom(pinchRef.current.zoom * (distance / pinchRef.current.distance));
      return;
    }
    if (!dragStartRef.current) return;
    const totalMove = Math.hypot(
      event.clientX - dragStartRef.current.x,
      event.clientY - dragStartRef.current.y,
    );
    if (totalMove > CLICK_MOVE_THRESHOLD) dragStartRef.current.moved = true;
    if (isImage && zoomRef.current > 1) {
      const stage = stageRef.current;
      setPan((prev) =>
        clampPanToStage(
          { x: prev.x + dx, y: prev.y + dy },
          zoomRef.current,
          stage?.clientWidth ?? 0,
          stage?.clientHeight ?? 0,
        ),
      );
    }
  }

  function handlePointerUp(event: React.PointerEvent<HTMLDivElement>) {
    pointersRef.current.delete(event.pointerId);
    if (pointersRef.current.size > 0) return;
    pinchRef.current = null;
    const start = dragStartRef.current;
    dragStartRef.current = null;
    if (!start) return;
    const dx = event.clientX - start.x;
    const dy = event.clientY - start.y;
    // 背板（点击媒体区外的空白背景）关闭——未发生拖拽的点击才生效，
    // 背板拖拽（缩放态平移起点落在空白区）不误关。
    if (start.onStage) {
      if (!start.moved) onOpenChange(false);
      return;
    }
    // 触摸滑动翻页仅限未缩放态（缩放时滑动优先平移）；鼠标拖拽不翻页。
    if (
      event.pointerType === "touch" &&
      zoomRef.current <= ZOOM_MIN &&
      Math.abs(dx) > SWIPE_THRESHOLD &&
      Math.abs(dx) > Math.abs(dy)
    ) {
      if (dx < 0) goNext();
      else goPrevious();
    }
  }

  function handlePointerCancel(event: React.PointerEvent<HTMLDivElement>) {
    pointersRef.current.delete(event.pointerId);
    if (pointersRef.current.size === 0) pinchRef.current = null;
    dragStartRef.current = null;
  }

  function handleDoubleClick() {
    if (!isImage) return;
    updateZoom(ZOOM_MIN);
  }

  const retry = useCallback(() => {
    const item = items[currentIndex];
    if (!item) return;
    setFailed((prev) => ({ ...prev, [item.id]: false }));
  }, [items, currentIndex]);

  if (!open || items.length === 0) return null;

  const positionLabel = t("media.viewer.position", { current: currentIndex + 1, total: items.length });
  const currentFailed = current ? Boolean(failed[current.id]) : false;

  return createPortal(
    <dialog
      ref={dialogRef}
      aria-label={t("media.viewer.title")}
      className={cn(
        "fixed inset-0 z-[60] m-0 flex h-dvh w-full max-h-none max-w-none items-center justify-center overflow-hidden border-0 bg-black/90 p-0 text-white",
        className,
      )}
      onCancel={(event) => {
        event.preventDefault();
        onOpenChange(false);
      }}
    >
      <div
        ref={stageRef}
        data-slot="viewer-stage"
        className="absolute inset-0 flex select-none items-center justify-center overflow-hidden"
        style={{ touchAction: "none" }}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerCancel}
        onDoubleClick={handleDoubleClick}
      >
        {current?.type === "image" ? (
          currentFailed ? (
            <div className="flex flex-col items-center gap-3 text-white/80">
              <ImageOff className="h-10 w-10" aria-hidden="true" />
              <span className="text-sm">{t("media.viewer.error.loadFailed")}</span>
              <button
                type="button"
                onClick={retry}
                className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md border border-white/30 px-3 text-sm text-white transition-colors hover:bg-white/10 focus:outline-none focus:ring-2 focus:ring-ring"
              >
                {t("media.viewer.error.retry")}
              </button>
            </div>
          ) : (
            <img
              src={current.url}
              alt={t("media.viewer.imageAlt", { current: currentIndex + 1, total: items.length })}
              draggable={false}
              className="max-h-full max-w-full object-contain"
              style={{
                transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
                transformOrigin: "center",
              }}
              onLoad={() => setFailed((prev) => ({ ...prev, [current.id]: false }))}
              onError={() => setFailed((prev) => ({ ...prev, [current.id]: true }))}
            />
          )
        ) : current ? (
          <video
            src={current.url}
            controls
            autoPlay
            poster={current.posterUrl}
            className="max-h-full max-w-full object-contain"
          />
        ) : null}
      </div>

      <div className="absolute inset-x-0 top-0 flex items-center justify-between gap-2 px-3 pt-[max(0.5rem,env(safe-area-inset-top))] lg:px-4">
        <span className="text-sm text-white/80">{positionLabel}</span>
        <button
          type="button"
          onClick={() => onOpenChange(false)}
          aria-label={t("media.viewer.close")}
          className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-white/90 transition-colors hover:bg-white/10 hover:text-white focus:outline-none focus:ring-2 focus:ring-ring"
        >
          <X className="h-5 w-5" aria-hidden="true" />
        </button>
      </div>

      {showPaging && (
        <>
          <button
            type="button"
            onClick={goPrevious}
            disabled={currentIndex === 0}
            aria-label={t("media.viewer.previous")}
            className="absolute left-2 top-1/2 inline-flex min-h-11 min-w-11 -translate-y-1/2 items-center justify-center rounded-md text-white/90 transition-colors hover:bg-white/10 hover:text-white focus:outline-none focus:ring-2 focus:ring-ring disabled:pointer-events-none disabled:opacity-40 lg:left-4"
          >
            <ChevronLeft className="h-6 w-6" aria-hidden="true" />
          </button>
          <button
            type="button"
            onClick={goNext}
            disabled={currentIndex === items.length - 1}
            aria-label={t("media.viewer.next")}
            className="absolute right-2 top-1/2 inline-flex min-h-11 min-w-11 -translate-y-1/2 items-center justify-center rounded-md text-white/90 transition-colors hover:bg-white/10 hover:text-white focus:outline-none focus:ring-2 focus:ring-ring disabled:pointer-events-none disabled:opacity-40 lg:right-4"
          >
            <ChevronRight className="h-6 w-6" aria-hidden="true" />
          </button>
          <div
            role="group"
            aria-label={positionLabel}
            className="absolute bottom-5 left-1/2 flex -translate-x-1/2 items-center gap-2"
          >
            {items.map((item, itemIndex) => (
              <span
                key={item.id}
                aria-hidden="true"
                className={cn(
                  "h-2 w-2 rounded-full transition-colors",
                  itemIndex === currentIndex ? "bg-white" : "bg-white/30",
                )}
              />
            ))}
          </div>
        </>
      )}

      {isImage && (
        <div className="absolute bottom-5 right-3 flex items-center gap-1 lg:right-4">
          <button
            type="button"
            onClick={() => updateZoom(zoomRef.current / ZOOM_BUTTON_STEP)}
            disabled={zoom <= ZOOM_MIN}
            aria-label={t("media.viewer.zoomOut")}
            className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-white/90 transition-colors hover:bg-white/10 hover:text-white focus:outline-none focus:ring-2 focus:ring-ring disabled:pointer-events-none disabled:opacity-40"
          >
            <Minus className="h-5 w-5" aria-hidden="true" />
          </button>
          <button
            type="button"
            onClick={() => updateZoom(ZOOM_MIN)}
            disabled={zoom <= ZOOM_MIN}
            aria-label={t("media.viewer.zoomReset")}
            className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-white/90 transition-colors hover:bg-white/10 hover:text-white focus:outline-none focus:ring-2 focus:ring-ring disabled:pointer-events-none disabled:opacity-40"
          >
            <RotateCcw className="h-5 w-5" aria-hidden="true" />
          </button>
          <button
            type="button"
            onClick={() => updateZoom(zoomRef.current * ZOOM_BUTTON_STEP)}
            disabled={zoom >= ZOOM_MAX}
            aria-label={t("media.viewer.zoomIn")}
            className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-white/90 transition-colors hover:bg-white/10 hover:text-white focus:outline-none focus:ring-2 focus:ring-ring disabled:pointer-events-none disabled:opacity-40"
          >
            <Plus className="h-5 w-5" aria-hidden="true" />
          </button>
        </div>
      )}
    </dialog>,
    document.body,
  );
}
