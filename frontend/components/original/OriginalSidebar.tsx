"use client";

import { useEffect, useState } from "react";
import { Heart, FileText, Clock } from "lucide-react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { Sidebar, type SidebarItem, type TrendingEntry } from "@/components/layout/Sidebar";
import { api } from "@/lib/api";

interface TrendingContentItem {
  title: string;
  score: number;
  contentId: number;
}

export function SidebarWrapper() {
  const { user } = useAuth();
  const t = useTranslations();
  const [trendingContents, setTrendingContents] = useState<TrendingContentItem[]>([]);

  useEffect(() => {
    api.get<{ trending?: Array<{ text?: string; score?: number; content_id?: number }> }>("/api/v1/search/trending")
      .then((data) => {
        if (!data || !Array.isArray(data.trending)) {
          return;
        }
        const items: TrendingContentItem[] = data.trending
          .filter((item) => item.text && item.content_id)
          .slice(0, 5)
          .map((item) => ({
            title: item.text ?? "",
            score: item.score ?? 0,
            contentId: item.content_id ?? 0,
          }));
        setTrendingContents(items);
      })
      .catch(() => {});
  }, []);

  const trendingEntries: TrendingEntry[] = trendingContents.map((item, i) => ({
    rank: i + 1,
    name: item.title,
    stat: item.score > 0 ? `${t("home.trendingHeat")} ${item.score}` : "",
    href: `/content/${item.contentId}`,
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
      // T24（FIX-40① 防御性可选）：与 home 同型——窄视口隐藏侧栏让内容全宽
      //（F-082 Phase 6 复测 overflow=0 未能复现，此为同构防御，Safari 手工复测备注留档）。
      className="hidden md:block"
      sections={sections}
      trending={trendingEntries.length > 0 ? { title: t("home.trendingContents"), entries: trendingEntries } : undefined}
    />
  );
}
