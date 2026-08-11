"use client";

import { useCallback, useMemo, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { AlertCircle, Compass, RotateCw } from "lucide-react";
import useSWRInfinite from "swr/infinite";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { ContentCard, type ContentCardData } from "@/components/content/ContentCard";
import { useContentDetailOverlay } from "@/components/content/use-content-detail-overlay";
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

interface RecommendPageData {
  items: ContentCardData[];
  total: number | null;
}

/**
 * /recommend 推荐流：单一"为你推荐"内容流（无分区标签），SSR 首屏 +
 * SWR useSWRInfinite 无限滚动（sort=recommended，page=2,3... 追加）；
 * 卡片点击打开共享 ContentDetailOverlay（source=recommendation），
 * 关闭后恢复页面滚动位置。
 */
export function RecommendFeedClient({
  apiBase,
  initialItems,
  initialTotal,
  initialError,
}: RecommendFeedClientProps) {
  const t = useTranslations();
  const { open: handleOpenDetail, overlayElement } = useContentDetailOverlay({
    source: "recommendation",
  });

  const firstPageRef = useRef<RecommendPageData | null>(null);
  firstPageRef.current = initialError ? null : { items: initialItems, total: initialTotal };
  const firstPageUrl = `${apiBase}/contents?sort=recommended&page=1&page_size=${PAGE_SIZE}`;

  const getKey = useCallback(
    (pageIndex: number) =>
      `${apiBase}/contents?sort=recommended&page=${pageIndex + 1}&page_size=${PAGE_SIZE}`,
    [apiBase],
  );

  const fetcher = useCallback(
    async (url: string): Promise<RecommendPageData> => {
      const cached = firstPageRef.current;
      if (cached && url === firstPageUrl) return cached;
      const res = await fetch(url, { cache: "no-store" });
      if (!res.ok) throw new Error("RECOMMEND_FETCH_FAILED");
      const data = (await res.json()) as { contents?: unknown[]; total?: number };
      return {
        items: normalizeContentList(data.contents),
        total: typeof data.total === "number" ? data.total : null,
      };
    },
    [firstPageUrl],
  );

  const {
    data,
    size,
    setSize,
    error: swrError,
    isValidating,
    mutate,
  } = useSWRInfinite(getKey, fetcher, {
    initialSize: initialError ? 0 : 1,
    fallbackData: firstPageRef.current ? [firstPageRef.current] : [],
    revalidateFirstPage: false,
    revalidateIfStale: false,
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
    shouldRetryOnError: false,
    dedupingInterval: 60000,
  });

  const items = useMemo(() => data?.flatMap((page) => page.items) ?? initialItems, [data, initialItems]);
  const total = data?.[data.length - 1]?.total ?? initialTotal;
  const hasMore = total !== null ? items.length < total : items.length >= PAGE_SIZE;

  const isLoading = isValidating && data === undefined;
  const isLoadingMore = isValidating && size > 1;
  const showInitialError = initialError || (swrError !== undefined && size <= 1);
  const loadError = swrError !== undefined && size > 1;

  const retryInitial = useCallback(() => {
    void setSize((current) => (current === 0 ? 1 : current));
  }, [setSize]);

  const loadMore = useCallback(() => {
    void setSize((current) => current + 1);
  }, [setSize]);

  const retryLoadMore = useCallback(() => {
    void mutate();
  }, [mutate]);

  const openDetail = useCallback(
    (data: ContentCardData, trigger: HTMLElement) => {
      const index = items.findIndex((item) => item.id === data.id);
      handleOpenDetail(
        {
          contentId: data.id,
          zone: data.zone === "original" ? "original" : "fanwork",
          /* #89 连续浏览：把触发上下文列表 + 当前索引传给浮层（移动端上滑切篇）。 */
          contextList: items.map((item) => ({
            id: item.id,
            zone: item.zone === "original" ? "original" : "fanwork",
          })),
          contextIndex: index >= 0 ? index : undefined,
        },
        trigger,
      );
    },
    [handleOpenDetail, items],
  );

  if (isLoading && items.length === 0) {
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

  if (showInitialError && items.length === 0) {
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
        onOpenDetail={openDetail}
        isLoadingMore={isLoadingMore}
        hasMore={hasMore}
        loadError={loadError}
        onLoadMore={loadMore}
        onRetry={retryLoadMore}
      />
      {overlayElement}
    </>
  );
}
