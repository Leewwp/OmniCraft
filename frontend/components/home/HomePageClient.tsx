"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { ChevronRight, Filter } from "lucide-react";
import { IPCard } from "@/components/ip/IPCard";
import { MasonryGrid } from "@/components/content/MasonryGrid";
import { ContentCardData } from "@/components/content/ContentCard";
import { Button } from "@/components/ui/button";
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

interface IPResponse {
  ips: IPItem[];
}

interface ContentResponse {
  contents: ContentCardData[];
}

interface RecentIP {
  id: number;
  name: string;
}

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
  const [timeRange, setTimeRange] = useState("all");

  const contentTypeOptions = useMemo(() => [
    { label: t('home.all'), value: "" },
    { label: t('home.text'), value: "text" },
    { label: t('home.image'), value: "image" },
    { label: t('home.video'), value: "video" },
    { label: t('home.audio'), value: "audio" },
    { label: t('home.other'), value: "mod" },
    { label: t('home.aiPrompt'), value: "prompt" },
    { label: t('home.sheetMusic'), value: "sheet_music" },
    { label: t('home.other'), value: "other" },
  ], [t]);

  const contentSortOptions = useMemo(() => [
    { label: t('home.hottest'), value: "hot" },
    { label: t('home.newest'), value: "newest" },
    { label: t('home.mostViewed'), value: "most_views" },
    { label: t('home.topRated'), value: "best_rated" },
  ], [t]);

  const timeRangeOptions = useMemo(() => [
    { label: t('home.all'), value: "all" },
    { label: t('home.thisWeek'), value: "week" },
    { label: t('home.thisMonth'), value: "month" },
    { label: t('home.thisYear'), value: "year" },
  ], [t]);

  const ipCategories = useMemo(() => {
    const fromData = Array.from(new Set(ips.map((ip) => ip.category).filter(Boolean))) as string[];
    return [ALL_KEY, ...fromData];
  }, [ips]);

  useEffect(() => {
    const raw = window.localStorage.getItem(RECENT_IP_KEY);
    if (!raw) {
      return;
    }
    try {
      const parsed = JSON.parse(raw) as RecentIP[];
      setRecentIPs(parsed.slice(0, 5));
    } catch {
      setRecentIPs([]);
    }
  }, []);

  useEffect(() => {
    const query = new URLSearchParams();
    if (ipCategory) {
      query.set("category", ipCategory);
    }
    query.set("sort", ipSort);

    const run = async () => {
      try {
        const res = await fetch(`${apiBase}/ips?${query.toString()}`, { cache: "no-store" });
        if (!res.ok) {
          return;
        }
        const data = (await res.json()) as IPResponse;
        setIPs(data.ips || []);
      } catch {
        setIPs([]);
      }
    };

    void run();
  }, [apiBase, ipCategory, ipSort]);

  useEffect(() => {
    const query = new URLSearchParams();
    query.set("zone", "fanwork");
    query.set("sort", contentSort);
    query.set("time_range", timeRange);
    if (contentType) {
      query.set("content_type", contentType);
    }

    const run = async () => {
      try {
        const res = await fetch(`${apiBase}/contents?${query.toString()}`, {
          cache: "no-store",
        });
        if (!res.ok) {
          return;
        }
        const data = (await res.json()) as ContentResponse;
        setContents(normalizeContentList(data.contents));
      } catch {
        setContents([]);
      }
    };

    void run();
  }, [apiBase, contentType, contentSort, timeRange]);

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-8 px-4 py-6">
      <section className="rounded-md border border-border bg-card p-4 shadow-none">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-semibold">{t('home.recentIps')}</h2>
          <Filter className="h-4 w-4 text-muted-foreground" />
        </div>

        {recentIPs.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t('home.noRecentIps')}</p>
        ) : (
          <div className="flex gap-2 overflow-x-auto pb-1">
            {recentIPs.map((item) => (
              <Link
                key={item.id}
                href={`/ip/${item.id}`}
                className="inline-flex items-center gap-2 whitespace-nowrap rounded-md border border-border px-3 py-2 text-xs hover:bg-muted"
              >
                {item.name}
                <ChevronRight className="h-3.5 w-3.5" />
              </Link>
            ))}
          </div>
        )}
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <h2 className="text-base font-semibold">{t('home.ipBrowseZone')}</h2>
          <select
            value={ipSort}
            onChange={(e) => setIPSort(e.target.value)}
            className="h-9 rounded-md border border-border bg-background px-3 text-sm"
          >
            <option value="hot">{t('home.hottest')}</option>
            <option value="newest">{t('home.newest')}</option>
            <option value="most_contents">{t('home.mostContent')}</option>
          </select>
        </div>

        <div className="flex flex-wrap gap-2">
          {ipCategories.map((category) => {
            const value = category === ALL_KEY ? "" : category;
            const active = ipCategory === value;
            return (
              <Button
                key={category}
                size="sm"
                variant={active ? "default" : "outline"}
                onClick={() => setIPCategory(value)}
              >
                {category === ALL_KEY ? t('home.all') : category}
              </Button>
            );
          })}
        </div>

        <div className="flex gap-3 overflow-x-auto pb-2">
          {ips.map((ip) => (
            <IPCard key={ip.id} data={ip} />
          ))}
        </div>
      </section>

      <section className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <h2 className="text-base font-semibold">{t('home.fanworkBrowseZone')}</h2>
          <div className="flex flex-wrap items-center gap-2">
            {contentSortOptions.map((option) => (
              <Button
                key={option.value}
                size="sm"
                variant={contentSort === option.value ? "default" : "outline"}
                onClick={() => setContentSort(option.value)}
              >
                {option.label}
              </Button>
            ))}
            <select
              value={timeRange}
              onChange={(e) => setTimeRange(e.target.value)}
              className="h-9 rounded-md border border-border bg-background px-3 text-sm"
            >
              {timeRangeOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          {contentTypeOptions.map((option) => (
            <Button
              key={option.label}
              size="sm"
              variant={contentType === option.value ? "default" : "outline"}
              onClick={() => setContentType(option.value)}
            >
              {option.label}
            </Button>
          ))}
        </div>

        <MasonryGrid items={contents} />
      </section>
    </div>
  );
}
