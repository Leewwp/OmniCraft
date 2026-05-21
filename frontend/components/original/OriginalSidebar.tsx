"use client";

import { Heart, FileText, Clock } from "lucide-react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { Sidebar, type SidebarItem, type TrendingEntry } from "@/components/layout/Sidebar";

const TRENDING_PLACEHOLDERS = [
  { rank: 1, nameKey: "home.trending1", stat: "4,823", query: "春日穿搭" },
  { rank: 2, nameKey: "home.trending2", stat: "3,216", query: "周末厨房" },
  { rank: 3, nameKey: "home.trending3", stat: "2,847", query: "桌面改造" },
  { rank: 4, nameKey: "home.trending4", stat: "2,103", query: "猫咪日常" },
  { rank: 5, nameKey: "home.trending5", stat: "1,876", query: "极简生活" },
];

export function SidebarWrapper() {
  const { user } = useAuth();
  const t = useTranslations();

  const trendingTopics: TrendingEntry[] = TRENDING_PLACEHOLDERS.map(item => ({
    rank: item.rank,
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
