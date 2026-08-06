"use client";

import { useCallback, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { AlertCircle, Compass, RotateCw } from "lucide-react";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { ContentCard, type ContentCardData } from "@/components/content/ContentCard";
import { ContentDetailOverlay } from "@/components/content/ContentDetailOverlay";
import { EmptyState } from "@/components/ui/empty-state";
import { buttonVariants } from "@/components/ui/button";
import { SkeletonCard } from "@/components/ui/skeleton";
import { normalizeContentList } from "@/lib/content";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 24;

interface RecommendFeedClientProps {
  apiBase: string;
  initialItems: ContentCardData[];
  initialTotal: number | null;
  initialError: boolean;
}

interface OverlayEntry {
  contentId: number;
  zone: "original" | "fanwork";
}

/**
 * /recommend 推荐流：单一"为你推荐"内容流（无分区标签），SSR 首屏 + 客户端
 * IntersectionObserver 无限滚动（sort=recommended）；卡片点击打开共享
 * ContentDetailOverlay（source=recommendation），关闭后恢复页面滚动位置。
 */
export function RecommendFeedClient({
  apiBase,
  initialItems,
  initialTotal,
  initialError,
}: RecommendFeedClientProps) {
  const t = useTranslations();
  const [items, setItems] = useState<ContentCardData[]>(initialItems);
  const [total, setTotal] = useState<number | null>(initialTotal);
  const [page, setPage] = useState(1);
  const [initialLoading, setInitialLoading] = useState(false);
  const [error, setError] = useState(initialError);
  const [loadingMore, setLoadingMore] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [overlayEntry, setOverlayEntry] = useState<OverlayEntry | null>(null);
  const overlayTriggerRef = useRef<HTMLElement | null>(null);

  const hasMore = total !== null ? items.length < total : items.length >= PAGE_SIZE;

  const fetchPage = useCallback(
    async (targetPage: number): Promise<{ items: ContentCardData[]; total: number | null } | null> => {
      const res = await fetch(
        `${apiBase}/contents?sort=recommended&page=${targetPage}&page_size=${PAGE_SIZE}`,
        { cache: "no-store" },
      );
      if (!res.ok) return null;
      const data = (await res.json()) as { contents?: unknown[]; total?: number };
      return {
        items: normalizeContentList(data.contents),
        total: typeof data.total === "number" ? data.total : null,
      };
    },
    [apiBase],
  );

  const retryInitial = useCallback(async () => {
    setInitialLoading(true);
    setError(false);
    try {
      const result = await fetchPage(1);
      if (!result) throw new Error("RECOMMEND_FETCH_FAILED");
      setItems(result.items);
      setTotal(result.total);
      setPage(1);
    } catch {
      setError(true);
    } finally {
      setInitialLoading(false);
    }
  }, [fetchPage]);

  const loadMore = useCallback(async () => {
    if (loadingMore) return;
    setLoadingMore(true);
    setLoadError(false);
    try {
      const result = await fetchPage(page + 1);
      if (!result) throw new Error("RECOMMEND_LOAD_MORE_FAILED");
      setItems((current) => [...current, ...result.items]);
      setTotal(result.total);
      setPage((current) => current + 1);
    } catch {
      setLoadError(true);
    } finally {
      setLoadingMore(false);
    }
  }, [fetchPage, loadingMore, page]);

  const handleOpenDetail = useCallback((data: ContentCardData, trigger: HTMLElement) => {
    overlayTriggerRef.current = trigger;
    setOverlayEntry({
      contentId: data.id,
      zone: data.zone === "original" ? "original" : "fanwork",
    });
  }, []);

  if (initialLoading && items.length === 0) {
    return (
      <div
        aria-label={t("recommend.loadingLabel")}
        aria-busy="true"
        className="grid grid-cols-2 gap-4 min-[701px]:grid-cols-3 min-[1101px]:grid-cols-4"
      >
        <SkeletonCard count={12} zone="fanwork" />
      </div>
    );
  }

  if (error && items.length === 0) {
    return (
      <EmptyState
        icon={AlertCircle}
        title={t("recommend.errorTitle")}
        description={t("recommend.errorDescription")}
        action={
          <button type="button" onClick={() => void retryInitial()} className={cn(buttonVariants({ variant: "outline" }))}>
            <RotateCw className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
            {t("recommend.retryAction")}
          </button>
        }
      />
    );
  }

  if (items.length === 0) {
    return (
      <EmptyState
        icon={Compass}
        title={t("recommend.emptyTitle")}
        description={t("recommend.emptyDescription")}
        action={
          <Link href="/original" className={cn(buttonVariants())}>
            {t("recommend.emptyAction")}
          </Link>
        }
      />
    );
  }

  return (
    <>
      <MasonryGrid
        items={items}
        onOpenDetail={handleOpenDetail}
        isLoadingMore={loadingMore}
        hasMore={hasMore}
        loadError={loadError}
        onLoadMore={() => void loadMore()}
        onRetry={() => void loadMore()}
      />

      {overlayEntry && (
        <ContentDetailOverlay
          key={`${overlayEntry.zone}:${overlayEntry.contentId}`}
          contentId={overlayEntry.contentId}
          zone={overlayEntry.zone}
          source="recommendation"
          open
          onOpenChange={(open) => {
            if (!open) setOverlayEntry(null);
          }}
          returnFocusRef={overlayTriggerRef}
        />
      )}
    </>
  );
}
