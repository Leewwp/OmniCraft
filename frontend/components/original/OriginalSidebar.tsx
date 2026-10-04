"use client";

import { useEffect, useState } from "react";
import { Heart, FileText, Clock } from "lucide-react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { Sidebar, type SidebarItem, type TrendingEntry } from "@/components/layout/Sidebar";
import { api } from "@/lib/api";
import { getContentHref } from "@/lib/content";

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
    /* #781（SP-26-B）：带 zone=original 过滤——trending 是全站混区热榜，
       不过滤时榜单混入二创，且原创行用 /content/ 前缀落 404（分区隔离）。 */
    api.get<{ trending?: Array<{ text?: string; score?: number; content_id?: number }> }>("/api/v1/search/trending?zone=original")
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
    /* #781：榜单请求恒带 zone=original，行内链接经 helper 分流到原创详情
       路由（/content/[id] 对 zone=original 有意 notFound）。 */
    href: getContentHref(item.contentId, "original"),
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
