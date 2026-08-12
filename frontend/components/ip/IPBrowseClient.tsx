"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { AlertCircle, Check, Search, SearchX } from "lucide-react";
import { IPCard } from "@/components/ip/IPCard";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { SortSelect } from "@/components/ui/SortSelect";

interface IPItem {
  id: number;
  name: string;
  category?: string;
  description?: string;
  cover_url?: string;
  content_count?: number;
  trend?: number;
}

function isIPItem(value: unknown): value is IPItem {
  if (!value || typeof value !== "object") return false;
  const item = value as Record<string, unknown>;
  return typeof item.id === "number" && typeof item.name === "string";
}

function parseIPListResponse(value: unknown): { ips: IPItem[]; total: number } | null {
  if (!value || typeof value !== "object") return null;
  const response = value as Record<string, unknown>;
  if (!Array.isArray(response.ips) || !response.ips.every(isIPItem)) return null;
  if (typeof response.total !== "number" || response.total < 0) return null;
  return { ips: response.ips, total: response.total };
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
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState(false);
  const [page, setPage] = useState(1);
  const pageSize = 24;

  const fetchIPs = useCallback(async (cat: string, s: string, q: string, p: number, append = false) => {
    if (append) setLoadingMore(true);
    else setLoading(true);
    setError(false);
    const params = new URLSearchParams();
    if (cat) params.set("category", cat);
    params.set("sort", s);
    if (q.trim()) params.set("q", q.trim());
    params.set("page", String(p));
    params.set("page_size", String(pageSize));

    try {
      const res = await fetch(`${apiBase}/ips?${params.toString()}`, { cache: "no-store" });
      if (!res.ok) throw new Error("IP_FETCH_FAILED");
      const data = parseIPListResponse(await res.json());
      if (!data) throw new Error("IP_RESPONSE_INVALID");
      const incoming = data.ips;
      setIPs((current) => append ? [...current, ...incoming] : incoming);
      setTotal(data.total);
      return true;
    } catch {
      if (!append) {
        setIPs([]);
        setTotal(0);
      }
      setError(true);
      return false;
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
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

  async function handleLoadMore() {
    const nextPage = page + 1;
    if (await fetchIPs(category, sort, search, nextPage, true)) {
      setPage(nextPage);
    }
  }

  return (
    <div className="mx-auto w-full max-w-[1440px] px-4 py-6 md:px-6 md:py-8">
      {/* Header */}
      <div className="mb-6 max-w-[720px]">
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">{t('ip.title')}</h1>
          {total > 0 && (
            <span className="rounded-full border border-primary/30 bg-accent-subtle px-2 py-0.5 text-xs font-semibold tabular-nums text-accent-emphasis">
              {t('ip.totalCount', { total })}
            </span>
          )}
        </div>
        <p className="mt-1 text-sm text-muted-foreground">
          {t('ip.browseDescription')}
        </p>
      </div>

      {/* Search + Sort row */}
      <div className="mb-4 grid max-w-[560px] grid-cols-[minmax(0,1fr)_auto] gap-2">
        <form role="search" onSubmit={handleSearchSubmit} className="relative min-w-0">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={searchInput}
            type="search"
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder={t('ip.searchPlaceholder')}
            aria-label={t('ip.searchPlaceholder')}
            className="min-h-11 w-full rounded-full border border-border bg-muted pl-9 pr-4 text-sm placeholder:text-muted-foreground/60 focus:bg-background"
          />
        </form>
        <div className="shrink-0">
          <SortSelect
            ariaLabel={t('ip.sortLabel')}
            value={sort}
            options={SORT_OPTIONS.map((opt) => ({ value: opt.value, label: t(opt.labelKey) }))}
            onChange={setSort}
            className="min-h-11"
          />
        </div>
      </div>

      {/* Category tabs */}
      <nav aria-label={t('home.ipClassification')} className="mb-6 flex items-center gap-1 overflow-x-auto pb-1" style={{ scrollbarWidth: "none" }}>
        {IP_CATEGORIES.map((cat) => {
          const active = category === cat.slug;
          return (
            <button
              key={cat.slug || "__all__"}
              type="button"
              onClick={() => setCategory(cat.slug)}
              aria-pressed={active}
              className={`inline-flex min-h-11 flex-shrink-0 items-center gap-1 rounded-full border px-3 text-xs font-medium transition-colors whitespace-nowrap focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                active
                  ? "border-accent-emphasis bg-accent-subtle text-accent-emphasis font-semibold"
                  : "border-transparent text-muted-foreground hover:bg-muted hover:text-foreground"
              }`}
            >
              {active && <Check className="h-3.5 w-3.5" aria-hidden="true" />}
              {t(cat.labelKey)}
            </button>
          );
        })}
      </nav>

      {/* IP Grid */}
      {loading ? (
        <div aria-label={t('common.loading')} aria-busy="true" className="grid grid-cols-2 gap-3 min-[701px]:grid-cols-[repeat(auto-fit,minmax(168px,1fr))] min-[701px]:gap-4 min-[1101px]:grid-cols-[repeat(auto-fill,minmax(192px,1fr))]">
          {Array.from({ length: 12 }).map((_, i) => (
            <div key={i} className="overflow-hidden rounded-lg border border-border bg-card shadow-[var(--elevation-1)]">
              <Skeleton className="aspect-[16/10] rounded-none" />
              <div className="p-3 space-y-2">
                <Skeleton className="h-4 w-2/3" />
                <Skeleton className="h-3 w-1/2" />
              </div>
            </div>
          ))}
        </div>
      ) : error && ips.length === 0 ? (
        <EmptyState
          icon={AlertCircle}
          title={t('common.loadFailed')}
          description={t('common.loadFailed')}
          action={<Button variant="outline" onClick={() => void fetchIPs(category, sort, search, 1)}>{t('common.retry')}</Button>}
        />
      ) : ips.length === 0 ? (
        <EmptyState
          icon={SearchX}
          title={t('ip.notFound')}
          description={t('ip.notFoundHint')}
          action={<Button onClick={() => { setCategory(""); setSearch(""); setSearchInput(""); }}>{t('ip.clearAllFilters')}</Button>}
        />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 min-[701px]:grid-cols-[repeat(auto-fit,minmax(168px,1fr))] min-[701px]:gap-4 min-[1101px]:grid-cols-[repeat(auto-fill,minmax(192px,1fr))]">
            {ips.map((ip) => (
              <IPCard key={ip.id} data={ip} variant="browse" />
            ))}
          </div>

          {/* Load more */}
          <div className="mt-8 flex min-h-11 flex-col items-center justify-center gap-2" aria-live="polite">
            {ips.length < total ? (
              <Button
                variant="outline"
                className="min-h-11 rounded-full px-8"
                onClick={() => void handleLoadMore()}
                disabled={loadingMore}
              >
                {loadingMore ? t('common.loading') : error ? t('common.retry') : t('ip.loadMore')}
              </Button>
            ) : total > pageSize ? (
              <p className="text-xs text-muted-foreground">{t('ip.totalCount', { total })}</p>
            ) : null}
          </div>
        </>
      )}
    </div>
  );
}
