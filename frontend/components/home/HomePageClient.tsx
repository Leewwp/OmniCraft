"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import {
  LayoutGrid, Gamepad2, Tv, BookOpen, Globe, Music, Clock, Film,
  Heart, Settings, FileText, ChevronRight,
} from "lucide-react";
import { IPCard } from "@/components/ip/IPCard";
import { OverlayMasonryGrid } from "@/components/content/OverlayMasonryGrid";
import { ContentCardData } from "@/components/content/ContentCard";
import { useAuth } from "@/contexts/AuthContext";
import { Sidebar, type SidebarItem, type TrendingEntry } from "@/components/layout/Sidebar";
import { SortSelect } from "@/components/ui/SortSelect";
import { FilterPills } from "@/components/ui/filter-pills";
import { SkeletonCard } from "@/components/ui/skeleton";
import { useContentInfiniteFeed, type ContentFeedPage } from "@/components/content/use-content-infinite-feed";
import { api } from "@/lib/api";
import { loadRecentIps, type RecentIPItem } from "@/lib/ip-visit-history";

interface IPItem {
  id: number;
  name: string;
  category?: string;
  description?: string;
}

interface HomePageClientProps {
  apiBase: string;
  initialIPs: IPItem[];
  initialContents: ContentCardData[];
  /** SSR 首屏 total（判尽用；与 initialContents 同签名 = 默认筛选态）。 */
  initialContentTotal: number | null;
}

interface IPResponse { ips: IPItem[] }

const ALL_KEY = "__all__";

export function HomePageClient({ apiBase, initialIPs, initialContents, initialContentTotal }: HomePageClientProps) {
  const t = useTranslations();
  const { user, ipHistoryVersion } = useAuth();
  const [recentIPs, setRecentIPs] = useState<RecentIPItem[]>([]);
  const [ips, setIPs] = useState<IPItem[]>(initialIPs);
  const [ipCategory, setIPCategory] = useState("");
  const [ipSort, setIPSort] = useState("hot");
  const [contentType, setContentType] = useState("");
  const [contentSort, setContentSort] = useState("hot");
  const [categoryCounts, setCategoryCounts] = useState<Record<string, string>>({});
  const [statsSummary, setStatsSummary] = useState<{ users: number; ips: number; contents: number } | null>(null);
  const [ipError, setIpError] = useState(false);
  const [ipCountsError, setIpCountsError] = useState(false);

  const contentTypeOptions = useMemo(() => [
    { label: t('home.all'), value: "" },
    { label: t('home.text'), value: "text" },
    { label: t('home.image'), value: "image" },
    { label: t('home.video'), value: "video" },
    { label: t('home.audio'), value: "audio" },
    { label: t('home.mod'), value: "mod" },
    { label: t('home.aiPrompt'), value: "prompt" },
    { label: t('home.sheetMusic'), value: "sheet_music" },
    { label: t('home.other'), value: "other" },
  ], [t]);

  const ipCategories = useMemo(() => {
    return [ALL_KEY, ...Array.from(new Set(ips.map((ip) => ip.category).filter(Boolean)))] as string[];
  }, [ips]);

  // Load recent IPs: anonymous history comes from local storage, signed-in
  // history from the account source; a completed login merge re-reads the list.
  useEffect(() => {
    let cancelled = false;
    loadRecentIps().then((items) => {
      if (!cancelled) setRecentIPs(items);
    });
    return () => {
      cancelled = true;
    };
  }, [user, ipHistoryVersion]);

  // Fetch stats summary
  useEffect(() => {
    /* #411 F3：二创区头部 = 分区统计（内容数只计二创、创作者 = 区内去重作者数）。 */
    api.getStatsSummary("fanwork")
      .then(d => { if (d?.summary) setStatsSummary(d.summary); })
      .catch(() => {});
  }, []);

  useEffect(() => {
    fetch(`${apiBase}/ips/stats/category_counts`, { cache: "no-store" })
      .then(r => r.ok ? r.json() as Promise<{ category_counts?: Record<string, string> }> : Promise.reject())
      .then(d => {
        if (d?.category_counts) {
          setCategoryCounts(d.category_counts);
          setIpCountsError(false);
        }
      })
      .catch(() => {
        setIpCountsError(true);
      });
  }, [apiBase]);

  useEffect(() => {
    const q = new URLSearchParams();
    if (ipCategory) q.set("category", ipCategory);
    q.set("sort", ipSort);
    fetch(`${apiBase}/ips?${q.toString()}`, { cache: "no-store" })
      .then(r => r.ok ? r.json() as Promise<IPResponse> : Promise.reject())
      .then(d => {
        setIPs(d.ips || []);
        setIpError(false);
      })
      .catch(() => {
        setIpError(true);
      });
  }, [apiBase, ipCategory, ipSort]);

  // Sidebar sections
  const formatCount = (v: string | undefined) => v ? parseInt(v, 10).toLocaleString() : "0";
  const sidebarSections = useMemo(() => [
    {
      label: t('home.ipClassification'),
      items: [
        { icon: <LayoutGrid className="h-4 w-4" />, label: t('home.allIps'), count: formatCount(Object.values(categoryCounts).reduce((a: number, v: string) => a + parseInt(v, 10), 0).toString()), active: ipCategory === "", onClick: () => setIPCategory("") },
        { icon: <Gamepad2 className="h-4 w-4" />, label: t('home.categoryGaming'), count: formatCount(categoryCounts.game), active: ipCategory === "game", onClick: () => setIPCategory("game") },
        { icon: <Tv className="h-4 w-4" />, label: t('home.categoryFilmTv'), count: formatCount(categoryCounts.film_tv), active: ipCategory === "film_tv", onClick: () => setIPCategory("film_tv") },
        { icon: <BookOpen className="h-4 w-4" />, label: t('home.animeCategory'), count: formatCount(categoryCounts.anime), active: ipCategory === "anime", onClick: () => setIPCategory("anime") },
        { icon: <Globe className="h-4 w-4" />, label: t('home.mangaCategory'), count: formatCount(categoryCounts.manga), active: ipCategory === "manga", onClick: () => setIPCategory("manga") },
        { icon: <Music className="h-4 w-4" />, label: t('home.novelCategory'), count: formatCount(categoryCounts.novel), active: ipCategory === "novel", onClick: () => setIPCategory("novel") },
        { icon: <Film className="h-4 w-4" />, label: t('home.varietyShowCategory'), count: formatCount(categoryCounts.variety), active: ipCategory === "variety", onClick: () => setIPCategory("variety") },
        { icon: <Tv className="h-4 w-4" />, label: t('home.shortDramaCategory'), count: formatCount(categoryCounts.short_drama), active: ipCategory === "short_drama", onClick: () => setIPCategory("short_drama") },
      ] as SidebarItem[],
    },
    {
      label: t('home.management'),
      items: [
        { icon: <Heart className="h-4 w-4" />, label: t('home.myFavorites'), href: user ? "/studio/favorites" : "/login?redirect=/studio/favorites" },
        { icon: <FileText className="h-4 w-4" />, label: t('home.myCreations'), href: user ? "/studio/contents" : "/login?redirect=/studio/contents" },
        { icon: <Clock className="h-4 w-4" />, label: t('nav.history'), href: user ? "/history" : "/login?redirect=/history" },
      ] as SidebarItem[],
    },
  ], [t, ipCategory, user, categoryCounts]);

  // Trending IPs (from top IPs)
  const trendingEntries: TrendingEntry[] = ips.slice(0, 6).map((ip, i) => ({
    rank: i + 1,
    avatar: <span>{ip.name.slice(0, 2)}</span>,
    name: ip.name,
    stat: `${ip.description || ""}`,
    href: `/ip/${ip.id}`,
  }));

  return (
    <div className="mx-auto flex w-full max-w-[1280px] min-h-[calc(100vh-52px)]">
      {/* Sidebar */}
      <Sidebar
        className="hidden md:block"
        sections={sidebarSections}
        trending={{ title: t('home.trendingIpsThisWeek'), entries: trendingEntries }}
      />

      {/* Main content */}
      <div data-testid="home-main-content" className="min-w-0 flex-1">
        {/* Zone banner */}
        <div className="px-4 pt-5 pb-3 md:px-6">
          <div className="flex items-baseline gap-3">
            <h1 className="text-xl font-semibold tracking-tight text-foreground">{t('nav.fanworkZone')}</h1>
            <p className="text-sm text-muted-foreground">{t('home.fanworkZoneSubtitle')}</p>
          </div>
          <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1">
            <span className="flex items-baseline gap-1">
              <span className="text-sm font-semibold text-foreground">{statsSummary ? statsSummary.contents.toLocaleString() : "--"}</span>
              <span className="text-xs text-muted-foreground">{t('home.contentCountLabel')}</span>
            </span>
            <span className="flex items-baseline gap-1">
              <span className="text-sm font-semibold text-foreground">{statsSummary ? statsSummary.ips.toLocaleString() : "--"}</span>
              <span className="text-xs text-muted-foreground">{t('home.activeIpsLabel')}</span>
            </span>
            <span className="flex items-baseline gap-1">
              <span className="text-sm font-semibold text-foreground">{statsSummary ? statsSummary.users.toLocaleString() : "--"}</span>
              <span className="text-xs text-muted-foreground">{t('home.creatorsLabel')}</span>
            </span>
          </div>
        </div>

        {/* Recent IPs */}
        {recentIPs.length > 0 && (
          <div className="px-4 pb-3 md:px-6">
            <div className="mb-2">
              <span className="text-sm font-semibold text-muted-foreground">{t('home.recentIps')}</span>
            </div>
            <div className="flex gap-2.5 overflow-x-auto" style={{ scrollbarWidth: 'none' }}>
              {recentIPs.map((ip) => (
                <Link
                  key={ip.id}
                  href={`/ip/${ip.id}`}
                  className="flex-shrink-0 rounded-lg border border-border bg-card px-3 py-2 text-sm font-medium text-foreground transition-colors duration-200 hover:border-accent/20 hover:bg-accent-subtle/5 active:bg-accent-subtle/10"
                >
                  {ip.name}
                </Link>
              ))}
            </div>
          </div>
        )}

        {/* IP horizontal scroll */}
        <div className="px-4 pb-2 md:px-6">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-sm font-semibold text-muted-foreground">{t('home.recommendedIps')}</span>
            <Link href="/ips" className="text-xs text-accent-emphasis font-medium">
              {t('home.browseAllIps')}
            </Link>
          </div>
          <div className="flex gap-2.5 overflow-x-auto pb-2" style={{ scrollbarWidth: 'none' }}>
            {ips.slice(0, 8).map((ip) => (
              <IPCard key={ip.id} data={ip} variant="browse" className="w-48 flex-none" />
            ))}
          </div>
          {(ipError || (ipCountsError && ips.length === 0)) && (
            <div className="rounded-md border border-border bg-card px-3 py-2 text-xs text-muted-foreground">
              {t("home.ipLoadFailed")}
            </div>
          )}
        </div>

        {/* Content toolbar */}
        <div className="sticky top-[52px] z-40 bg-background px-4 py-2.5 md:px-6">
          <div className="flex min-w-0 items-center gap-2">
            {/* #414 O1a：本地筛选按钮收敛为共享 FilterPills（本页原形态即矮药丸基准） */}
            <FilterPills
              ariaLabel={t('home.contentFilterLabel')}
              options={contentTypeOptions}
              value={contentType}
              onChange={setContentType}
              className="min-w-0 flex-1"
            />
            <div className="shrink-0">
              <SortSelect
                ariaLabel={t('common.sortLabel')}
                value={contentSort}
                options={[
                  { value: "hot", label: t('home.hottest') },
                  { value: "newest", label: t('home.newest') },
                  { value: "most_views", label: t('home.mostViewed') },
                ]}
                onChange={setContentSort}
              />
            </div>
          </div>
        </div>

        {/* Masonry grid（#410 F2：筛选签名重挂 feed 段 = 重置回第 1 页） */}
        <div className="px-4 py-4 pb-16 md:px-6">
          <FanworkFeedSection
            key={`${contentType}|${contentSort}`}
            apiBase={apiBase}
            contentType={contentType}
            contentSort={contentSort}
            initialPage={
              contentType === "" && contentSort === "hot"
                ? { items: initialContents, total: initialContentTotal }
                : null
            }
            emptyText={t("home.noOriginalContent")}
            loadFailedText={t("home.contentLoadFailed")}
            retryText={t("common.retry")}
          />
        </div>
      </div>
    </div>
  );
}

/** 二创内容流段（#410 F2）：useContentInfiniteFeed + 骨架/错误/终态。 */
function FanworkFeedSection({
  apiBase,
  contentType,
  contentSort,
  initialPage,
  emptyText,
  loadFailedText,
  retryText,
}: {
  apiBase: string;
  contentType: string;
  contentSort: string;
  initialPage: ContentFeedPage | null;
  emptyText: string;
  loadFailedText: string;
  retryText: string;
}) {
  const feed = useContentInfiniteFeed({
    apiBase,
    filters: { zone: "fanwork", contentType: contentType || undefined, sort: contentSort },
    initialPage,
  });
  const { items, hasMore, isLoading, isLoadingMore, showInitialError, loadError } = feed;

  if (isLoading && items.length === 0) {
    return (
      <div aria-busy="true" className="grid grid-cols-2 gap-4 min-[701px]:grid-cols-3 min-[1101px]:grid-cols-4">
        <SkeletonCard count={12} zone="fanwork" />
      </div>
    );
  }

  if (showInitialError && items.length === 0) {
    return (
      <div className="flex flex-col items-center gap-3 rounded-md border border-border bg-card p-8 text-center text-sm text-muted-foreground">
        <span>{loadFailedText}</span>
        <button
          type="button"
          onClick={() => feed.retryInitial()}
          className="rounded-md border border-border px-3 py-1.5 text-xs font-medium text-foreground transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
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
