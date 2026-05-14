"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import {
  LayoutGrid, Gamepad2, Tv, BookOpen, Globe, Music, Clock, Film,
  Heart, Settings, FileText, ChevronRight,
} from "lucide-react";
import { IPCard } from "@/components/ip/IPCard";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { ContentCardData } from "@/components/content/ContentCard";
import { useAuth } from "@/contexts/AuthContext";
import { Sidebar, type SidebarItem, type TrendingEntry } from "@/components/layout/Sidebar";
import { normalizeContentList } from "@/lib/content";
import { api } from "@/lib/api";

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
}

interface IPResponse { ips: IPItem[] }
interface ContentResponse { contents: ContentCardData[] }
interface RecentIP { id: number; name: string }

const RECENT_IP_KEY = "recent_ips";
const ALL_KEY = "__all__";

export function HomePageClient({ apiBase, initialIPs, initialContents }: HomePageClientProps) {
  const t = useTranslations();
  const { user } = useAuth();
  const [recentIPs, setRecentIPs] = useState<RecentIP[]>([]);
  const [ips, setIPs] = useState<IPItem[]>(initialIPs);
  const [contents, setContents] = useState<ContentCardData[]>(initialContents);
  const [ipCategory, setIPCategory] = useState("");
  const [ipSort, setIPSort] = useState("hot");
  const [contentType, setContentType] = useState("");
  const [contentSort, setContentSort] = useState("hot");
  const [categoryCounts, setCategoryCounts] = useState<Record<string, string>>({});
  const [statsSummary, setStatsSummary] = useState<{ users: number; ips: number; contents: number } | null>(null);

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

  // Load recent IPs
  useEffect(() => {
    try {
      const raw = localStorage.getItem(RECENT_IP_KEY);
      if (raw) setRecentIPs(JSON.parse(raw).slice(0, 6));
    } catch { /* ignore */ }
  }, []);

  // Fetch stats summary
  useEffect(() => {
    api.getStatsSummary()
      .then(d => { if (d?.summary) setStatsSummary(d.summary); })
      .catch((e) => { console.error("[HomePage] stats fetch failed", e); });
  }, []);

  // Fetch category counts
  useEffect(() => {
    fetch(`${apiBase}/ips/stats/category_counts`, { cache: "no-store" })
      .then(r => r.ok ? r.json() as Promise<{ category_counts?: Record<string, string> }> : null)
      .then(d => { if (d?.category_counts) setCategoryCounts(d.category_counts); })
      .catch((e) => { console.error("[HomePage] category counts fetch failed", e); });
  }, [apiBase]);

  // Fetch IPs
  useEffect(() => {
    const q = new URLSearchParams();
    if (ipCategory) q.set("category", ipCategory);
    q.set("sort", ipSort);
    fetch(`${apiBase}/ips?${q.toString()}`, { cache: "no-store" })
      .then(r => r.ok ? r.json() as Promise<IPResponse> : null)
      .then(d => { if (d) setIPs(d.ips || []); })
      .catch((e) => { console.error("[HomePage] IPs fetch failed", e); });
  }, [apiBase, ipCategory, ipSort]);

  // Fetch contents
  useEffect(() => {
    const q = new URLSearchParams();
    q.set("zone", "fanwork");
    q.set("sort", contentSort);
    q.set("time_range", "all");
    if (contentType) q.set("content_type", contentType);
    fetch(`${apiBase}/contents?${q.toString()}`, { cache: "no-store" })
      .then(r => r.ok ? r.json() as Promise<ContentResponse> : null)
      .then(d => { if (d) setContents(normalizeContentList(d.contents)); })
      .catch((e) => { console.error("[HomePage] contents fetch failed", e); });
  }, [apiBase, contentType, contentSort]);

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
        { icon: <Heart className="h-4 w-4" />, label: t('home.myFavorites'), href: user ? "/studio/contents" : "/login?redirect=/studio/contents" },
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
    <div className="mx-auto flex w-full max-w-[1440px] min-h-[calc(100vh-52px)]">
      {/* Sidebar */}
      <Sidebar
        sections={sidebarSections}
        trending={{ title: t('home.trendingIpsThisWeek'), entries: trendingEntries }}
      />

      {/* Main content */}
      <div className="flex-1 min-w-0">
        {/* Zone banner */}
        <div className="px-6 pt-5 pb-3">
          <div className="flex items-baseline gap-3">
            <h1 className="text-[22px] font-bold tracking-tight text-foreground">{t('nav.fanworkZone')}</h1>
            <p className="text-sm text-muted-foreground">{t('home.fanworkZoneSubtitle')}</p>
          </div>
          <div className="mt-3 flex gap-4">
            <span className="flex items-baseline gap-1">
              <span className="text-[15px] font-semibold text-foreground">{statsSummary ? statsSummary.contents.toLocaleString() : "--"}</span>
              <span className="text-xs text-muted-foreground">{t('home.contentCountLabel')}</span>
            </span>
            <span className="flex items-baseline gap-1">
              <span className="text-[15px] font-semibold text-foreground">{statsSummary ? statsSummary.ips.toLocaleString() : "--"}</span>
              <span className="text-xs text-muted-foreground">{t('home.activeIpsLabel')}</span>
            </span>
            <span className="flex items-baseline gap-1">
              <span className="text-[15px] font-semibold text-foreground">{statsSummary ? statsSummary.users.toLocaleString() : "--"}</span>
              <span className="text-xs text-muted-foreground">{t('home.creatorsLabel')}</span>
            </span>
          </div>
        </div>

        {/* Recent IPs */}
        {recentIPs.length > 0 && (
          <div className="px-6 pb-3">
            <div className="mb-2">
              <span className="text-[13px] font-semibold text-muted-foreground">{t('home.recentIps')}</span>
            </div>
            <div className="flex gap-2.5 overflow-x-auto" style={{ scrollbarWidth: 'none' }}>
              {recentIPs.map((ip) => (
                <Link
                  key={ip.id}
                  href={`/ip/${ip.id}`}
                  className="flex-shrink-0 rounded-lg border border-border bg-card px-3 py-2 text-sm font-medium text-foreground transition-all duration-200 hover:border-accent/20 hover:bg-accent-subtle/5 hover:-translate-y-0.5 active:scale-[0.97]"
                >
                  {ip.name}
                </Link>
              ))}
            </div>
          </div>
        )}

        {/* IP horizontal scroll */}
        <div className="px-6 pb-2">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-[13px] font-semibold text-muted-foreground">{t('home.recommendedIps')}</span>
            <Link href="/ips" className="text-xs text-accent-emphasis font-medium">
              {t('home.browseAllIps')}
            </Link>
          </div>
          <div className="flex gap-2.5 overflow-x-auto pb-2" style={{ scrollbarWidth: 'none' }}>
            {ips.slice(0, 8).map((ip) => (
              <IPCard key={ip.id} data={ip} variant="browse" />
            ))}
          </div>
        </div>

        {/* Content toolbar */}
        <div className="sticky top-[52px] z-40 bg-background px-6 py-2.5">
          <div className="flex items-center gap-0">
            <div className="flex flex-1 items-center gap-1 overflow-x-auto" style={{ scrollbarWidth: 'none' }}>
              {contentTypeOptions.map((opt) => {
                const active = contentType === opt.value;
                return (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => setContentType(opt.value)}
                    className={`flex-shrink-0 rounded-md px-3 py-1.5 text-[12.5px] font-medium transition-all duration-150 whitespace-nowrap border select-none active:scale-95 ${
                      active
                        ? "border-border bg-muted text-foreground font-semibold"
                        : "border-transparent text-muted-foreground hover:text-foreground hover:bg-muted/70 cursor-pointer"
                    }`}
                  >
                    {opt.label}
                  </button>
                );
              })}
            </div>
            <div className="ml-3 flex-shrink-0">
              <select
                value={contentSort}
                onChange={(e) => setContentSort(e.target.value)}
                className="rounded-md border border-border bg-card px-2.5 py-1.5 text-xs text-foreground outline-none focus:ring-1 focus:ring-ring"
              >
                <option value="hot">{t('home.hottest')}</option>
                <option value="newest">{t('home.newest')}</option>
                <option value="most_views">{t('home.mostViewed')}</option>
              </select>
            </div>
          </div>
        </div>

        {/* Masonry grid */}
        <div className="px-6 py-4 pb-16">
          <MasonryGrid items={contents} emptyText={t("home.noOriginalContent")} />
        </div>
      </div>
    </div>
  );
}
