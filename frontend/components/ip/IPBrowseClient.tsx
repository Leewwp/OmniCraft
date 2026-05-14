"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Search, LayoutGrid, List } from "lucide-react";
import { IPCard } from "@/components/ip/IPCard";
import { Input } from "@/components/ui/input";

interface IPItem {
  id: number;
  name: string;
  category?: string;
  description?: string;
  cover_url?: string;
  content_count?: number;
  trend?: number;
}

interface IPBrowseClientProps {
  apiBase: string;
  initialIPs: IPItem[];
  initialTotal: number;
}

const IP_CATEGORIES = [
  { slug: "", labelKey: "home.allIps" },
  { slug: "game", labelKey: "home.categoryGaming" },
  { slug: "film_tv", labelKey: "home.categoryFilmTv" },
  { slug: "anime", labelKey: "home.animeCategory" },
  { slug: "manga", labelKey: "home.mangaCategory" },
  { slug: "novel", labelKey: "home.novelCategory" },
  { slug: "music", labelKey: "home.audio" },
  { slug: "variety", labelKey: "home.varietyShowCategory" },
  { slug: "short_drama", labelKey: "home.shortDramaCategory" },
  { slug: "vtuber", labelKey: "home.other" },
  { slug: "other", labelKey: "home.other" },
];

const SORT_OPTIONS = [
  { value: "hot", labelKey: "ip.sortHot" },
  { value: "most_contents", labelKey: "ip.sortMostContents" },
  { value: "newest", labelKey: "ip.sortNewest" },
  { value: "name", labelKey: "ip.sortByName" },
];

export function IPBrowseClient({ apiBase, initialIPs, initialTotal }: IPBrowseClientProps) {
  const t = useTranslations();
  const router = useRouter();
  const searchParams = useSearchParams();

  const [ips, setIPs] = useState<IPItem[]>(initialIPs);
  const [total, setTotal] = useState(initialTotal);
  const [category, setCategory] = useState(searchParams.get("category") || "");
  const [sort, setSort] = useState(searchParams.get("sort") || "hot");
  const [search, setSearch] = useState(searchParams.get("q") || "");
  const [searchInput, setSearchInput] = useState(searchParams.get("q") || "");
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const pageSize = 24;

  const fetchIPs = useCallback(async (cat: string, s: string, q: string, p: number) => {
    setLoading(true);
    const params = new URLSearchParams();
    if (cat) params.set("category", cat);
    params.set("sort", s);
    if (q.trim()) params.set("q", q.trim());
    params.set("page", String(p));
    params.set("page_size", String(pageSize));

    try {
      const res = await fetch(`${apiBase}/ips?${params.toString()}`, { cache: "no-store" });
      if (!res.ok) { setIPs([]); setTotal(0); return; }
      const data = await res.json() as { ips?: IPItem[]; total?: number };
      setIPs(data.ips || []);
      setTotal(data.total || 0);
    } catch { setIPs([]); setTotal(0); }
    finally { setLoading(false); }
  }, [apiBase, pageSize]);

  // Update category/sort → refetch
  useEffect(() => {
    const params = new URLSearchParams();
    if (category) params.set("category", category);
    if (sort !== "hot") params.set("sort", sort);
    if (search.trim()) params.set("q", search.trim());
    const qs = params.toString();
    router.replace(qs ? `/ips?${qs}` : "/ips", { scroll: false });
    setPage(1);
    fetchIPs(category, sort, search, 1);
  }, [category, sort, search, fetchIPs, router]);

  function handleSearchSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSearch(searchInput);
  }

  return (
    <div className="mx-auto w-full max-w-[1440px] px-6 py-6">
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-[22px] font-bold tracking-tight text-foreground">{t('ip.title')}</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {t('ip.browseDescription')}
        </p>
        {total > 0 && (
          <p className="mt-2 text-xs text-muted-foreground">
            {t('ip.totalCount', { total })}
          </p>
        )}
      </div>

      {/* Search + Sort row */}
      <div className="mb-4 flex items-center gap-3">
        <form onSubmit={handleSearchSubmit} className="relative flex-1 max-w-sm">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder={t('ip.searchPlaceholder')}
            className="w-full rounded-full border border-transparent bg-muted pl-9 pr-4 py-2 text-sm placeholder:text-muted-foreground/60 focus:border-ring focus:bg-background focus:ring-2 focus:ring-ring/20"
          />
        </form>
        <div className="flex-shrink-0">
          <select
            value={sort}
            onChange={(e) => setSort(e.target.value)}
            className="rounded-md border border-border bg-card px-3 py-2 text-sm text-foreground outline-none focus:ring-1 focus:ring-ring"
          >
            {SORT_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>{t(opt.labelKey)}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Category tabs */}
      <div className="mb-6 flex items-center gap-1 overflow-x-auto pb-1" style={{ scrollbarWidth: "none" }}>
        {IP_CATEGORIES.map((cat) => {
          const active = category === cat.slug;
          return (
            <button
              key={cat.slug || "__all__"}
              type="button"
              onClick={() => setCategory(cat.slug)}
              className={`flex-shrink-0 rounded-full border px-3.5 py-1.5 text-[13px] font-medium transition-colors whitespace-nowrap ${
                active
                  ? "border-border bg-card text-foreground font-semibold"
                  : "border-transparent text-muted-foreground hover:text-foreground hover:bg-muted"
              }`}
            >
              {t(cat.labelKey)}
            </button>
          );
        })}
      </div>

      {/* IP Grid */}
      {loading ? (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
          {Array.from({ length: 12 }).map((_, i) => (
            <div key={i} className="rounded-lg border border-border bg-card overflow-hidden">
              <div className="aspect-[16/10] bg-muted animate-pulse" />
              <div className="p-3 space-y-2">
                <div className="h-4 w-2/3 bg-muted rounded animate-pulse" />
                <div className="h-3 w-1/2 bg-muted rounded animate-pulse" />
              </div>
            </div>
          ))}
        </div>
      ) : ips.length === 0 ? (
        <div className="rounded-xl border border-border bg-card p-16 text-center">
          <Search className="mx-auto mb-4 h-10 w-10 text-muted-foreground/40" />
          <p className="text-sm font-medium text-foreground">{t('ip.notFound')}</p>
          <p className="mt-1 text-sm text-muted-foreground">
            {t('ip.notFoundHint')}
          </p>
          <button
            type="button"
            onClick={() => { setCategory(""); setSearch(""); setSearchInput(""); }}
            className="mt-4 rounded-full bg-[var(--accent-emphasis)] px-6 py-2 text-sm font-medium text-white hover:bg-[var(--accent-hover)] transition-colors"
          >
            {t('ip.clearAllFilters')}
          </button>
        </div>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
            {ips.map((ip) => (
              <IPCard key={ip.id} data={ip} variant="browse" />
            ))}
          </div>

          {/* Load more */}
          {ips.length < total && (
            <div className="mt-8 text-center">
              <button
                type="button"
                onClick={() => {
                  const nextPage = page + 1;
                  setPage(nextPage);
                  fetchIPs(category, sort, search, nextPage);
                }}
                disabled={loading}
                className="rounded-full border border-border bg-card px-8 py-2.5 text-sm font-medium text-foreground hover:border-border/80 transition-colors disabled:opacity-50"
              >
                {loading ? t('common.loading') : t('ip.loadMore')}
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
