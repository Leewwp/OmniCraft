"use client";

import { useCallback } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { AlertCircle, Compass, RotateCw } from "lucide-react";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import type { ContentCardData } from "@/components/content/ContentCard";
import { useContentDetailOverlay } from "@/components/content/use-content-detail-overlay";
import { useContentInfiniteFeed } from "@/components/content/use-content-infinite-feed";
import { EmptyState } from "@/components/ui/empty-state";
import { buttonVariants } from "@/components/ui/button";
import { SkeletonCard } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

interface RecommendFeedClientProps {
  apiBase: string;
  initialItems: ContentCardData[];
  initialTotal: number | null;
  initialError: boolean;
}

/**
 * /recommend 推荐流：单一"为你推荐"内容流（无分区标签），SSR 首屏 +
 * 无限滚动（sort=recommended，page=2,3... 追加，页大小 12 = 全局裁决）；
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

  const feed = useContentInfiniteFeed({
    apiBase,
    filters: { sort: "recommended" },
    initialPage: initialError ? null : { items: initialItems, total: initialTotal },
    initialError,
  });
  const { items, hasMore, isLoading, isLoadingMore, showInitialError, loadError } = feed;

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
          <button type="button" onClick={() => feed.retryInitial()} className={cn(buttonVariants({ variant: "outline" }))}>
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
        onLoadMore={feed.loadMore}
        onRetry={feed.retryLoadMore}
      />
      {overlayElement}
    </>
  );
}
