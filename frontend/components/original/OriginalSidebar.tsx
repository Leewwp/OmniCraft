"use client";

import { useEffect, useState } from "react";
import { Heart, FileText, Clock } from "lucide-react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { Sidebar, type SidebarItem, type TrendingEntry } from "@/components/layout/Sidebar";
import { api } from "@/lib/api";

interface TrendingSearchItem {
  query: string;
  stat: string;
  nameKey: string;
}

const FALLBACK_SEARCHES: TrendingSearchItem[] = [
  { nameKey: "home.trending1", stat: "4,823", query: "春日穿搭" },
  { nameKey: "home.trending2", stat: "3,216", query: "周末厨房" },
  { nameKey: "home.trending3", stat: "2,847", query: "桌面改造" },
  { nameKey: "home.trending4", stat: "2,103", query: "猫咪日常" },
  { nameKey: "home.trending5", stat: "1,876", query: "极简生活" },
];

export function SidebarWrapper() {
  const { user } = useAuth();
  const t = useTranslations();
  const [trendingSearches, setTrendingSearches] = useState<TrendingSearchItem[]>([]);

  useEffect(() => {
    api.get<{ items?: Array<{ name?: string; id?: string | number }> }>("/api/v1/search/trending")
      .then((data) => {
        if (data && Array.isArray(data.items) && data.items.length > 0) {
          const items: TrendingSearchItem[] = data.items.slice(0, 5).map((item, i) => ({
            query: item.name || String(item.id || ""),
            stat: String(Math.floor(Math.random() * 4000 + 1000)),
            nameKey: `home.trending${i + 1}`,
          }));
          setTrendingSearches(items);
        }
      })
      .catch(() => {});
  }, []);

  const activeSearches = trendingSearches.length > 0 ? trendingSearches : FALLBACK_SEARCHES;

  const trendingTopics: TrendingEntry[] = activeSearches.map((item, i) => ({
    rank: i + 1,
    name: t(item.nameKey),
    stat: `${item.stat} ${t("home.trendingParticipants")}`,
    href: `/search?q=${encodeURIComponent(item.query)}`,
  }));

  const sections = [
    {
      label: t("common.manage"),
      items: [
        { icon: <Heart className="h-4 w-4" />, label: t("nav.favorites"), href: user ? "/studio/contents" : "/login?redirect=/studio/contents" },
        { icon: <FileText className="h-4 w-4" />, label: t("nav.myOriginal"), href: user ? "/studio/contents" : "/login?redirect=/studio/contents" },
        { icon: <Clock className="h-4 w-4" />, label: t("nav.history"), href: user ? "/history" : "/login?redirect=/history" },
      ] as SidebarItem[],
    },
  ];

  return (
    <Sidebar
      sections={sections}
      trending={{ title: t("home.trendingTopics"), entries: trendingTopics }}
    />
  );
}
