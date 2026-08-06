"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { FileText, Loader2 } from "lucide-react";
import { ContentCard, ContentCardData } from "@/components/content/ContentCard";import { EmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  computeShortestColumnLayout,
  MASONRY_GAP,
  masonryColumnCount,
  type MasonryPosition,
} from "@/lib/masonry-layout";

interface MasonryGridProps {
  items: ContentCardData[];
  emptyText?: string;
  className?: string;
  /** 与 ContentCard 双式一致：original 走无边框卡，fanwork 走 1px border 卡。 */
  zone?: "original" | "fanwork";
  /** 浮窗模式回调：提供后卡片点击触发 onOpenDetail 而非跳详情页。 */
  onOpenDetail?: (data: ContentCardData, trigger: HTMLElement) => void;
  /** 底部加载更多状态：sentinel 进入视口时触发 onLoadMore。 */
  isLoadingMore?: boolean;
  hasMore?: boolean;
  loadError?: boolean;
  onLoadMore?: () => void;
  onRetry?: () => void;
}

interface LayoutState {
  positions: MasonryPosition[];
  height: number;
  signature: string;
}

function layoutSignature(positions: MasonryPosition[], height: number): string {
  return `${height}:${positions.map((p) => `${Math.round(p.top)}x${Math.round(p.left)}`).join(",")}`;
}

/**
 * 最短列瀑布流：按 items 顺序逐条放入当前累计高度最短列，DOM/键盘/读屏顺序
 * 保持 items 原顺序；窗口与图片高度变化时无显著动画地重新平衡（瞬时重排，
 * 禁止 CSS columns-* 与列优先填充）。未完成测量前回退到同断点网格，避免
 * 首屏空白与重叠闪烁。
 */
export function MasonryGrid({
  items,
  emptyText,
  className,
  onOpenDetail,
  isLoadingMore = false,
  hasMore = false,
  loadError = false,
  onLoadMore,
  onRetry,
}: MasonryGridProps) {
  const t = useTranslations();
  const containerRef = useRef<HTMLDivElement>(null);
  const itemRefs = useRef<Array<HTMLDivElement | null>>([]);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const [mounted, setMounted] = useState(false);
  const [layout, setLayout] = useState<LayoutState | null>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

  useLayoutEffect(() => {
    if (!mounted) return;
    const container = containerRef.current;
    if (!container || items.length === 0) return;

    function measure() {
      const el = containerRef.current;
      if (!el) return;
      const columns = masonryColumnCount(window.innerWidth);
      const width = (el.clientWidth - (columns - 1) * MASONRY_GAP) / columns;
      if (width <= 0) return;
      const nodes = itemRefs.current.slice(0, items.length);
      for (const node of nodes) {
        if (node) node.style.width = `${width}px`;
      }
      const heights = nodes.map((node) => node?.offsetHeight ?? 0);
      const next = computeShortestColumnLayout(heights, columns, MASONRY_GAP, width);
      const signature = layoutSignature(next.positions, next.height);
      if (signature !== layout?.signature) {
        setLayout({ ...next, signature });
      }
    }

    measure();
    const resizeObserver = typeof ResizeObserver !== "undefined" ? new ResizeObserver(measure) : null;
    resizeObserver?.observe(container);
    for (const node of itemRefs.current.slice(0, items.length)) {
      if (node) resizeObserver?.observe(node);
    }
    window.addEventListener("resize", measure);
    return () => {
      resizeObserver?.disconnect();
      window.removeEventListener("resize", measure);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mounted, items]);

  useEffect(() => {
    if (!hasMore || !onLoadMore || isLoadingMore || loadError) return;
    const sentinel = sentinelRef.current;
    if (!sentinel || typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) onLoadMore();
      },
      { rootMargin: "400px 0px" },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [hasMore, onLoadMore, isLoadingMore, loadError]);

  if (items.length === 0) {
    return (
      <EmptyState
        icon={FileText}
        title={emptyText || t("content.emptyContentMsg")}
        description={t("content.emptyContentHint")}
        className="p-8"
      />
    );
  }

  const hasFullLayout = layout !== null && layout.positions.length === items.length;

  return (
    <div
      ref={containerRef}
      className={cn(
        "w-full",
        hasFullLayout
          ? "relative"
          : "grid grid-cols-2 gap-4 min-[701px]:grid-cols-3 min-[1101px]:grid-cols-4",
        className,
      )}
      style={hasFullLayout ? { height: layout.height } : undefined}
    >
      {items.map((item, index) => {
        const position = hasFullLayout ? layout.positions[index] : null;
        return (
          <div
            key={item.id}
            ref={(node) => {
              itemRefs.current[index] = node;
            }}
            className={cn("min-w-0", position && "absolute left-0 top-0")}
            style={position ? { top: position.top, left: position.left, width: position.width } : undefined}
          >
            <ContentCard data={item} onOpenDetail={onOpenDetail} />
          </div>
        );
      })}

      {onLoadMore && (
        <div
          className="flex min-h-11 flex-col items-center justify-center gap-2"
          aria-live="polite"
        >
          <div ref={sentinelRef} className="h-4" />
          {loadError ? (
            <Button variant="outline" size="sm" onClick={onRetry}>
              {t("common.retry")}
            </Button>
          ) : isLoadingMore ? (
            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" aria-hidden="true" />
          ) : !hasMore ? (
            <p className="text-xs text-muted-foreground">{t("common.endReached")}</p>
          ) : null}
        </div>
      )}
    </div>
  );
}
