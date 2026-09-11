"use client";

import { useCallback, useMemo, useRef } from "react";
import useSWRInfinite from "swr/infinite";
import type { ContentCardData } from "@/components/content/ContentCard";
import { normalizeContentList } from "@/lib/content";

/**
 * 三瀑布流（推荐 / 二创首页 / 原创）共享的无限滚动接线（#410 F2）：
 * SSR 首屏作为 useSWRInfinite 的 fallbackData + page=2,3... 追加 + total 判尽。
 * 页大小统一 12（2026-09-07 用户裁决：首屏 = 每页 = 12 条）。
 *
 * 筛选切换语义由调用方承担：以筛选签名为 React key 重挂本 hook 所在的
 * feed 段（重置回第 1 页；SSR 首屏数据仅在签名与初始态一致时作为
 * initialPage 传入，否则首屏走客户端加载 + 骨架）。
 */

/** 首屏 = 每页条数（全局裁决 #2，已确认）。 */
export const FEED_PAGE_SIZE = 12;

export interface ContentFeedFilters {
  zone?: "original" | "fanwork";
  contentType?: string;
  category?: string;
  sort: string;
  timeRange?: string;
}

export interface ContentFeedPage {
  items: ContentCardData[];
  total: number | null;
}

/** 构造列表页 URL（与 SSR 首屏请求同参形，含 page/page_size）。 */
export function contentFeedUrl(
  apiBase: string,
  filters: ContentFeedFilters,
  page: number,
  pageSize: number = FEED_PAGE_SIZE,
): string {
  const params = new URLSearchParams({
    sort: filters.sort,
    time_range: filters.timeRange ?? "all",
    page: String(page),
    page_size: String(pageSize),
  });
  if (filters.zone) params.set("zone", filters.zone);
  if (filters.contentType) params.set("content_type", filters.contentType);
  if (filters.category) params.set("category", filters.category);
  return `${apiBase}/contents?${params.toString()}`;
}

interface ContentFeedResponse {
  contents?: unknown[];
  total?: number;
}

export function useContentInfiniteFeed(options: {
  apiBase: string;
  filters: ContentFeedFilters;
  /** SSR 首屏（须与 filters 同签名；null/undefined = 首屏客户端加载）。 */
  initialPage?: ContentFeedPage | null;
  /** SSR 首屏请求失败标记（initialPage 为 null 且首屏需重试入口）。 */
  initialError?: boolean;
}) {
  const { apiBase, filters, initialPage = null, initialError = false } = options;

  const firstPageRef = useRef<ContentFeedPage | null>(initialPage);
  const firstPageUrl = contentFeedUrl(apiBase, filters, 1);

  const getKey = useCallback(
    (pageIndex: number) => contentFeedUrl(apiBase, filters, pageIndex + 1),
    // 筛选变化由调用方以 key 重挂本 hook 承担，此处依赖只随 apiBase 变化。
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [apiBase, firstPageUrl],
  );

  const fetcher = useCallback(
    async (url: string): Promise<ContentFeedPage> => {
      const cached = firstPageRef.current;
      if (cached && url === firstPageUrl) return cached;
      const res = await fetch(url, { cache: "no-store" });
      if (!res.ok) throw new Error("CONTENT_FEED_FETCH_FAILED");
      const data = (await res.json()) as ContentFeedResponse;
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
    /* fallbackData 语义分野（踩坑记录）：
       - SSR 首屏 → [initialPage]（不再回源第 1 页）；
       - SSR 失败（initialError）→ []（「无数据但已定」，错误态立即可渲染，
         不进加载骨架，重试经 setSize(0→1)）；
       - 筛选切换后的重挂（无 SSR 页）→ undefined（SWR 视为未加载，
         立即发第 1 页请求并进入加载骨架）。 */
    fallbackData: initialError ? [] : initialPage ? [initialPage] : undefined,
    revalidateFirstPage: false,
    revalidateIfStale: false,
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
    shouldRetryOnError: false,
    dedupingInterval: 60000,
  });

  const items = useMemo(
    () => data?.flatMap((page) => page.items) ?? [],
    [data],
  );
  const total = data?.[data.length - 1]?.total ?? initialPage?.total ?? null;
  const hasMore = total !== null ? items.length < total : items.length >= FEED_PAGE_SIZE;

  const isLoading = isValidating && data === undefined && !initialError;
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

  return {
    items,
    total,
    hasMore,
    isLoading,
    isLoadingMore,
    showInitialError,
    loadError,
    loadMore,
    retryInitial,
    retryLoadMore,
  };
}
