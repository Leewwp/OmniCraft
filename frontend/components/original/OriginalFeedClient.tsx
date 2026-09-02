"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { OverlayMasonryGrid } from "@/components/content/OverlayMasonryGrid";
import { ContentCardData } from "@/components/content/ContentCard";
import { FilterPills } from "@/components/ui/filter-pills";
import { SortSelect as SharedSortSelect } from "@/components/ui/SortSelect";
import { normalizeContentList } from "@/lib/content";
import { resolveDefaultSort } from "@/lib/search-filters";

interface CategoryTab {
  slug: string;
  i18n: string;
  name_i18n?: Record<string, string>;
}

interface ContentResponse {
  contents?: unknown[];
}

interface OriginalFeedClientProps {
  apiBase: string;
  categories: CategoryTab[];
  initialContents: ContentCardData[];
  initialCategory: string;
  initialSort: string;
}

const SORT_OPTIONS_KEYS = [
  { value: "recommended", labelKey: "home.categoryRecommended" },
  { value: "hot", labelKey: "content.sortHottest" },
  { value: "newest", labelKey: "content.sortNewRelease" },
  { value: "most_views", labelKey: "content.sortMostViewed" },
];

// 原创区筛选就地化（SP-12 U-03）：类目药丸与排序点击仅客户端刷新
// 列表并 router.replace 同步 URL，不滚动不跳页；SSR 首屏由服务端供给。
export function OriginalFeedClient({ apiBase, categories, initialContents, initialCategory, initialSort }: OriginalFeedClientProps) {
  const t = useTranslations();
  const router = useRouter();
  const [category, setCategory] = useState(initialCategory);
  const [sort, setSort] = useState(initialSort || "recommended");
  const [contents, setContents] = useState<ContentCardData[]>(initialContents);
  const [loading, setLoading] = useState(false);
  // Signature guard: skip the first effect run (and StrictMode re-runs of it);
  // only real filter changes past the initial URL state trigger sync.
  const lastApplied = useRef<string>(`${initialCategory}|${resolveDefaultSort({ category: initialCategory, sort: initialSort })}`);

  const fetchContents = useCallback(async (nextCategory: string, nextSort: string) => {
    setLoading(true);
    const effectiveSort = resolveDefaultSort({ category: nextCategory, sort: nextSort });
    const params = new URLSearchParams({ zone: "original", sort: effectiveSort, time_range: "all", page_size: "24" });
    if (nextCategory) params.set("category", nextCategory);
    try {
      const res = await fetch(`${apiBase}/contents?${params.toString()}`, { cache: "no-store" });
      if (!res.ok) throw new Error("FETCH_FAILED");
      const data = (await res.json()) as ContentResponse;
      setContents(normalizeContentList(data.contents));
    } catch {
      setContents([]);
    } finally {
      setLoading(false);
    }
  }, [apiBase]);

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
    void fetchContents(category, sort);
  }, [category, sort, router, fetchContents]);

  return (
    <>
      {/* Category pills + sort — unified sticky row */}
      <div className="sticky top-[52px] z-40 border-b border-border-default bg-canvas-default px-4 py-2.5 md:px-6">
        <div className="flex items-center gap-0">
          <FilterPills
            ariaLabel={t("content.originalZone")}
            className="flex-1"
            loading={loading}
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
        <OverlayMasonryGrid items={contents} emptyText={t("home.noOriginalContent")} source="zone-page" />
      </div>
    </>
  );
}
