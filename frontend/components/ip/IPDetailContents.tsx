"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { OverlayMasonryGrid } from "@/components/content/OverlayMasonryGrid";
import { ContentCardData } from "@/components/content/ContentCard";
import { FilterPills } from "@/components/ui/filter-pills";
import { Skeleton } from "@/components/ui/skeleton";
import { ipCategoryOptions } from "@/components/ip/ipCategory";
import { normalizeContentList } from "@/lib/content";

interface IPDetailContentsProps {
  ipId: number;
  apiBase: string;
  initialContents: ContentCardData[];
  initialCategory?: string;
  initialSort?: string;
}

interface ContentResponse {
  contents?: ContentCardData[];
}

// IP 详情类目内容就地切换（ui-spec §/ip/[ipId] 类目内容就地切换）：
// FilterPills 点击仅刷新下方列表并 router.replace 同步 URL，不滚动不跳页。
export function IPDetailContents({ ipId, apiBase, initialContents, initialCategory, initialSort }: IPDetailContentsProps) {
  const t = useTranslations();
  const router = useRouter();
  const [category, setCategory] = useState(initialCategory || "all");
  const [contents, setContents] = useState<ContentCardData[]>(initialContents);
  const [loading, setLoading] = useState(false);
  // Signature guard: skip the first effect run (and StrictMode re-runs of it);
  // only real category changes past the initial URL state trigger sync.
  const lastApplied = useRef<string>(category);
  const sort = initialSort || "hot";

  const fetchContents = useCallback(async (nextCategory: string) => {
    setLoading(true);
    const params = new URLSearchParams({
      ip_id: String(ipId),
      zone: "fanwork",
      sort,
      time_range: "all",
      page_size: "24",
    });
    if (nextCategory !== "all") {
      params.set("content_type", nextCategory);
    }
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
  }, [apiBase, ipId, sort]);

  useEffect(() => {
    if (category === lastApplied.current) {
      return;
    }
    lastApplied.current = category;
    const qs = new URLSearchParams();
    if (category !== "all") qs.set("category", category);
    if (sort !== "hot") qs.set("sort", sort);
    const query = qs.toString();
    router.replace(query ? `/ip/${ipId}?${query}` : `/ip/${ipId}`, { scroll: false });
    void fetchContents(category);
  }, [category, router, ipId, sort, fetchContents]);

  return (
    <section className="space-y-3 rounded-md border border-border bg-card p-4">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-base font-semibold">{t('content.allContent')}</h2>
      </div>
      <FilterPills
        ariaLabel={t('ip.contentCategories')}
        options={ipCategoryOptions.map((item) => ({ value: item.key, label: t(item.label) }))}
        value={category}
        onChange={setCategory}
      />
      {loading ? (
        <div className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4" aria-busy="true" aria-label={t('common.loading')}>
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="overflow-hidden rounded-lg border border-border bg-card">
              <Skeleton className="aspect-[16/10] rounded-none" />
              <div className="space-y-2 p-3">
                <Skeleton className="h-4 w-2/3" />
                <Skeleton className="h-3 w-1/2" />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <OverlayMasonryGrid items={contents} source="ip-page" />
      )}
    </section>
  );
}
