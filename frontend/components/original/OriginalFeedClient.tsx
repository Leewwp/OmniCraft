"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { OverlayMasonryGrid } from "@/components/content/OverlayMasonryGrid";
import { ContentCardData } from "@/components/content/ContentCard";
import {
  useContentInfiniteFeed,
  type ContentFeedPage,
} from "@/components/content/use-content-infinite-feed";
import { SkeletonCard } from "@/components/ui/skeleton";
import { FilterPills } from "@/components/ui/filter-pills";
import { SortSelect as SharedSortSelect } from "@/components/ui/SortSelect";
import { resolveDefaultSort } from "@/lib/search-filters";

interface CategoryTab {
  slug: string;
  i18n: string;
  name_i18n?: Record<string, string>;
}

interface OriginalFeedClientProps {
  apiBase: string;
  categories: CategoryTab[];
  initialContents: ContentCardData[];
  /** SSR 首屏 total（与 initialContents 同签名）。 */
  initialTotal: number | null;
  initialCategory: string;
  initialSort: string;
}

const SORT_OPTIONS_KEYS = [
  { value: "recommended", labelKey: "home.categoryRecommended" },
  { value: "hot", labelKey: "content.sortHottest" },
  { value: "newest", labelKey: "content.sortNewRelease" },
  { value: "most_views", labelKey: "content.sortMostViewed" },
];

/** 原创区内容流段（#410 F2）：useContentInfiniteFeed + 骨架/错误/终态。 */
function OriginalFeedSection({
  apiBase,
  category,
  sort,
  initialPage,
  emptyText,
  loadFailedText,
  retryText,
}: {
  apiBase: string;
  category: string;
  sort: string;
  initialPage: ContentFeedPage | null;
  emptyText: string;
  loadFailedText: string;
  retryText: string;
}) {
  const feed = useContentInfiniteFeed({
    apiBase,
    filters: { zone: "original", category: category || undefined, sort },
    initialPage,
  });
  const { items, hasMore, isLoading, isLoadingMore, showInitialError, loadError } = feed;

  if (isLoading && items.length === 0) {
    return (
      <div aria-busy="true" className="grid grid-cols-2 gap-4 min-[701px]:grid-cols-3 min-[1101px]:grid-cols-4">
        <SkeletonCard count={12} zone="original" />
      </div>
    );
  }

  if (showInitialError && items.length === 0) {
    return (
      <div className="flex flex-col items-center gap-3 rounded-md border border-border-default bg-card p-8 text-center text-sm text-muted-foreground">
        <span>{loadFailedText}</span>
        <button
          type="button"
          onClick={() => feed.retryInitial()}
          className="rounded-md border border-border-default px-3 py-1.5 text-xs font-medium text-foreground transition-colors hover:bg-canvas-subtle focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {retryText}
        </button>
      </div>
    );
  }

  return (
    <OverlayMasonryGrid
      items={items}
      emptyText={emptyText}
      source="zone-page"
      isLoadingMore={isLoadingMore}
      hasMore={hasMore}
      loadError={loadError}
      onLoadMore={feed.loadMore}
      onRetry={feed.retryLoadMore}
    />
  );
}

// 原创区筛选就地化（SP-12 U-03）：类目药丸与排序点击仅客户端刷新列表并
// router.replace 同步 URL，不滚动不跳页；SSR 首屏由服务端供给。
// #410 F2：筛选签名重挂 feed 段 = 重置回第 1 页 + 无限滚动（页大小 12）。
export function OriginalFeedClient({
  apiBase,
  categories,
  initialContents,
  initialTotal,
  initialCategory,
  initialSort,
}: OriginalFeedClientProps) {
  const t = useTranslations();
  const router = useRouter();
  const [category, setCategory] = useState(initialCategory);
  const [sort, setSort] = useState(initialSort || "recommended");
  const initialSignature = `${initialCategory}|${resolveDefaultSort({ category: initialCategory, sort: initialSort })}`;
  const isInitialSignature =
    category === initialCategory &&
    resolveDefaultSort({ category, sort }) === initialSignature.split("|")[1];
  // Signature guard: skip the first effect run (and StrictMode re-runs of it);
  // only real filter changes past the initial URL state trigger sync.
  const lastApplied = useRef(initialSignature);

  useEffect(() => {
    const effectiveSort = resolveDefaultSort({ category, sort });
    const signature = `${category}|${effectiveSort}`;
    if (signature === lastApplied.current) {
      return;
    }
    lastApplied.current = signature;
    const qs = new URLSearchParams();
    if (category) qs.set("category", category);
    if (effectiveSort !== "recommended") qs.set("sort", effectiveSort);
    const query = qs.toString();
    router.replace(query ? `/original?${query}` : "/original", { scroll: false });
  }, [category, sort, router]);

  return (
    <>
      {/* Category pills + sort — unified sticky row */}
      <div className="sticky top-[52px] z-40 border-b border-border-default bg-canvas-default px-4 py-2.5 md:px-6">
        <div className="flex items-center gap-0">
          <FilterPills
            ariaLabel={t("content.originalZone")}
            className="flex-1"
            options={categories.map((cat) => ({
              value: cat.slug,
              label: cat.i18n ? t(cat.i18n) : cat.name_i18n?.zh || cat.name_i18n?.en || cat.slug,
            }))}
            value={category}
            onChange={setCategory}
          />
          <div className="ml-3 flex-shrink-0">
            <SharedSortSelect
              ariaLabel={t('common.sortLabel')}
              value={resolveDefaultSort({ category, sort })}
              options={SORT_OPTIONS_KEYS.map((opt) => ({ value: opt.value, label: t(opt.labelKey) }))}
              onChange={setSort}
            />
          </div>
        </div>
      </div>

      {/* Content masonry */}
      <div className="px-4 pt-4 pb-16 md:px-6">
        <OriginalFeedSection
          key={`${category}|${resolveDefaultSort({ category, sort })}`}
          apiBase={apiBase}
          category={category}
          sort={resolveDefaultSort({ category, sort })}
          initialPage={isInitialSignature ? { items: initialContents, total: initialTotal } : null}
          emptyText={t("home.noOriginalContent")}
          loadFailedText={t("home.contentLoadFailed")}
          retryText={t("common.retry")}
        />
      </div>
    </>
  );
}
