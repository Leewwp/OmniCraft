"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Search } from "lucide-react";
import { OverlayMasonryGrid } from "@/components/content/OverlayMasonryGrid";
import { ContentCardData } from "@/components/content/ContentCard";
import { FilterPills } from "@/components/ui/filter-pills";
import { SortSelect as SharedSortSelect } from "@/components/ui/SortSelect";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { normalizeContentList } from "@/lib/content";

interface ContentResponse {
  contents?: ContentCardData[];
  total?: number;
  type_counts?: Record<string, number>;
}

const MEDIA_TYPES = [
  { value: "all", labelKey: "home.all" },
  { value: "image", labelKey: "home.image" },
  { value: "article", labelKey: "home.text" },
  { value: "video", labelKey: "home.video" },
  { value: "audio", labelKey: "home.audio" },
  { value: "mod", labelKey: "home.mod" },
  { value: "prompt", labelKey: "home.aiPrompt" },
  { value: "sheet_music", labelKey: "home.sheetMusic" },
  { value: "other", labelKey: "home.other" },
];

const SORTS = [
  { value: "newest", labelKey: "content.sortNewRelease" },
  { value: "hot", labelKey: "content.sortHottest" },
  { value: "most_views", labelKey: "content.sortMostViewed" },
  { value: "best_rated", labelKey: "content.sortTopRated" },
];

interface IPShareTabProps {
  ipId: number;
  apiBase: string;
  query: string;
  type: string;
  sort: string;
  onTypeChange: (next: string) => void;
  onSortChange: (next: string) => void;
}

// 内容分享 tab：该 IP 的二创（zone=fanwork），媒体类型药丸（带命中计数）+
// 四排序 + IP 内搜索过滤；筛选状态由 URL query 驱动（#290 单页契约）。
export function IPShareTab({ ipId, apiBase, query, type, sort, onTypeChange, onSortChange }: IPShareTabProps) {
  const t = useTranslations();
  const [contents, setContents] = useState<ContentCardData[]>([]);
  const [typeCounts, setTypeCounts] = useState<Record<string, number>>({});
  const [total, setTotal] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  // 请求序号守卫：快速切换筛选/搜索时丢弃过期响应
  const fetchSeqRef = useRef(0);

  const fetchContents = useCallback(async (nextType: string, nextSort: string, q: string) => {
    const seq = ++fetchSeqRef.current;
    setLoading(true);
    const params = new URLSearchParams({ sort: nextSort, page_size: "24" });
    if (nextType !== "all") params.set("type", nextType);
    if (q) params.set("q", q);
    try {
      const res = await fetch(`${apiBase}/ips/${ipId}/contents?${params.toString()}`, { cache: "no-store" });
      if (!res.ok) throw new Error("FETCH_FAILED");
      const data = (await res.json()) as ContentResponse;
      if (seq !== fetchSeqRef.current) return;
      setContents(normalizeContentList(data.contents));
      setTypeCounts(data.type_counts ?? {});
      setTotal(data.total ?? null);
    } catch {
      if (seq !== fetchSeqRef.current) return;
      setContents([]);
      setTypeCounts({});
      setTotal(null);
    } finally {
      if (seq === fetchSeqRef.current) setLoading(false);
    }
  }, [apiBase, ipId]);

  useEffect(() => {
    void fetchContents(type, sort, query);
  }, [type, sort, query, fetchContents]);

  const pillCount = (value: string): number | undefined => {
    if (total == null) return undefined;
    return value === "all" ? total : typeCounts[value] ?? 0;
  };

  return (
    <section className="space-y-3" aria-label={t('ip.hubTab_share')}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <FilterPills
          ariaLabel={t('search.contentType')}
          options={MEDIA_TYPES.map((m) => ({ value: m.value, label: t(m.labelKey), count: pillCount(m.value) }))}
          value={type}
          onChange={onTypeChange}
          loading={loading}
          className="flex-1"
        />
        <SharedSortSelect
          ariaLabel={t('common.sortLabel')}
          value={sort}
          options={SORTS.map((s) => ({ value: s.value, label: t(s.labelKey) }))}
          onChange={onSortChange}
        />
      </div>

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
      ) : contents.length === 0 ? (
        <EmptyState
          icon={Search}
          title={query ? t('ip.hubNoSearchResult', { q: query }) : t('home.noOriginalContent')}
          description={query ? t('ip.hubNoSearchHint') : t('ip.hubEmptyShareHint')}
        />
      ) : (
        <OverlayMasonryGrid items={contents} source="ip-page" />
      )}
    </section>
  );
}
