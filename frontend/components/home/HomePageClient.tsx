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
import { Sidebar, type SidebarItem, type TrendingEntry } from "@/components/layout/Sidebar";
import { normalizeContentList } from "@/lib/content";

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
  const [recentIPs, setRecentIPs] = useState<RecentIP[]>([]);
  const [ips, setIPs] = useState<IPItem[]>(initialIPs);
  const [contents, setContents] = useState<ContentCardData[]>(initialContents);
  const [ipCategory, setIPCategory] = useState("");
  const [ipSort, setIPSort] = useState("hot");
  const [contentType, setContentType] = useState("");
  const [contentSort, setContentSort] = useState("hot");

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

  // Fetch IPs
  useEffect(() => {
    const q = new URLSearchParams();
    if (ipCategory) q.set("category", ipCategory);
    q.set("sort", ipSort);
    fetch(`${apiBase}/ips?${q.toString()}`, { cache: "no-store" })
      .then(r => r.ok ? r.json() as Promise<IPResponse> : null)
      .then(d => { if (d) setIPs(d.ips || []); })
      .catch(() => {});
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
      .catch(() => {});
  }, [apiBase, contentType, contentSort]);

  // Sidebar sections
  const sidebarSections = [
    {
      label: "IP 分类",
      items: [
        { icon: <LayoutGrid className="h-4 w-4" />, label: "全部 IP", active: ipCategory === "", onClick: () => setIPCategory("") },
        { icon: <Gamepad2 className="h-4 w-4" />, label: "游戏", active: ipCategory === "game", onClick: () => setIPCategory("game") },
        { icon: <Tv className="h-4 w-4" />, label: "影视", active: ipCategory === "film_tv", onClick: () => setIPCategory("film_tv") },
        { icon: <BookOpen className="h-4 w-4" />, label: "动画", active: ipCategory === "anime", onClick: () => setIPCategory("anime") },
        { icon: <Globe className="h-4 w-4" />, label: "漫画", active: ipCategory === "manga", onClick: () => setIPCategory("manga") },
        { icon: <Music className="h-4 w-4" />, label: "小说", active: ipCategory === "novel", onClick: () => setIPCategory("novel") },
      ] as SidebarItem[],
    },
    {
      label: "管理",
      items: [
        { icon: <Heart className="h-4 w-4" />, label: "我的收藏", href: "/studio/contents" },
        { icon: <FileText className="h-4 w-4" />, label: "我的创作", href: "/studio/contents" },
        { icon: <Clock className="h-4 w-4" />, label: "浏览历史", href: "/history" },
      ] as SidebarItem[],
    },
  ];

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
        trending={{ title: "热门 IP · 本周", entries: trendingEntries }}
      />

      {/* Main content */}
      <div className="flex-1 min-w-0">
        {/* Zone banner */}
        <div className="px-6 pt-5 pb-3">
          <div className="flex items-baseline gap-3">
            <h1 className="text-[22px] font-bold tracking-tight text-foreground">二创区</h1>
            <p className="text-sm text-muted-foreground">基于 IP 的协同创作与内容发现</p>
          </div>
          <div className="mt-3 flex gap-4">
            <span className="flex items-baseline gap-1">
              <span className="text-[15px] font-semibold text-foreground">58,293</span>
              <span className="text-xs text-muted-foreground">内容</span>
            </span>
            <span className="flex items-baseline gap-1">
              <span className="text-[15px] font-semibold text-foreground">186</span>
              <span className="text-xs text-muted-foreground">活跃 IP</span>
            </span>
            <span className="flex items-baseline gap-1">
              <span className="text-[15px] font-semibold text-foreground">8,412</span>
              <span className="text-xs text-muted-foreground">创作者</span>
            </span>
          </div>
        </div>

        {/* Recent IPs */}
        {recentIPs.length > 0 && (
          <div className="px-6 pb-3">
            <div className="mb-2">
              <span className="text-[13px] font-semibold text-muted-foreground">最近访问IP</span>
            </div>
            <div className="flex gap-2.5 overflow-x-auto" style={{ scrollbarWidth: 'none' }}>
              {recentIPs.map((ip) => (
                <Link
                  key={ip.id}
                  href={`/ip/${ip.id}`}
                  className="flex-shrink-0 rounded-lg border border-border bg-card px-3 py-2 text-sm font-medium text-foreground hover:border-border/80 transition-colors"
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
            <span className="text-[13px] font-semibold text-muted-foreground">推荐 IP</span>
            <Link href="/search" className="text-xs text-accent-emphasis font-medium">
              浏览全部 IP →
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
                    className={`flex-shrink-0 rounded-md px-3 py-1.5 text-[12.5px] font-medium transition-colors whitespace-nowrap border ${
                      active
                        ? "border-border bg-muted text-foreground font-semibold"
                        : "border-transparent text-muted-foreground hover:text-foreground hover:bg-muted"
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
