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

export function SidebarWrapper() {
  const { user } = useAuth();
  const t = useTranslations();
  const [trendingSearches, setTrendingSearches] = useState<TrendingSearchItem[]>([]);

  useEffect(() => {
    api.get<{ items?: Array<{ name?: string; id?: string | number; participant_count?: number }> }>("/api/v1/search/trending")
      .then((data) => {
        if (data && Array.isArray(data.items) && data.items.length > 0) {
          const items: TrendingSearchItem[] = data.items.slice(0, 5).map((item, i) => ({
            query: item.name || String(item.id || ""),
            stat: item.participant_count ? String(item.participant_count) : "",
            nameKey: `home.trending${i + 1}`,
          }));
          setTrendingSearches(items);
        }
      })
      .catch(() => {});
  }, []);

  const trendingTopics: TrendingEntry[] = trendingSearches.map((item, i) => ({
    rank: i + 1,
    name: t(item.nameKey),
    stat: item.stat ? `${item.stat} ${t("home.trendingParticipants")}` : "",
    href: `/search?q=${encodeURIComponent(item.query)}`,
  }));

  const sections = [
    {
      label: t("common.manage"),
      items: [
        { icon: <Heart className="h-4 w-4" />, label: t("nav.favorites"), href: user ? "/studio/favorites" : "/login?redirect=/studio/favorites" },
        { icon: <FileText className="h-4 w-4" />, label: t("nav.myOriginal"), href: user ? "/studio/contents" : "/login?redirect=/studio/contents" },
        { icon: <Clock className="h-4 w-4" />, label: t("nav.history"), href: user ? "/history" : "/login?redirect=/history" },
      ] as SidebarItem[],
    },
  ];

  return (
    <Sidebar
      sections={sections}
      trending={trendingTopics.length > 0 ? { title: t("home.trendingTopics"), entries: trendingTopics } : undefined}
    />
  );
}
