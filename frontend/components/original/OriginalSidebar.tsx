"use client";

import { Heart, FileText, Clock } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { Sidebar, type SidebarItem, type TrendingEntry } from "@/components/layout/Sidebar";

const trendingTopics: TrendingEntry[] = [
  { rank: 1, name: "#春日穿搭挑战", stat: "4,823 参与", href: "/search?q=春日穿搭" },
  { rank: 2, name: "#周末厨房日记", stat: "3,216 参与", href: "/search?q=周末厨房" },
  { rank: 3, name: "#桌面改造计划", stat: "2,847 参与", href: "/search?q=桌面改造" },
  { rank: 4, name: "#猫咪日常", stat: "2,103 参与", href: "/search?q=猫咪日常" },
  { rank: 5, name: "#极简生活", stat: "1,876 参与", href: "/search?q=极简生活" },
];

export function SidebarWrapper() {
  const { user } = useAuth();

  const sections = [
    {
      label: "管理",
      items: [
        { icon: <Heart className="h-4 w-4" />, label: "我的喜欢", href: user ? "/studio/contents" : "/login?redirect=/studio/contents" },
        { icon: <FileText className="h-4 w-4" />, label: "我的原创", href: user ? "/studio/contents" : "/login?redirect=/studio/contents" },
        { icon: <Clock className="h-4 w-4" />, label: "浏览历史", href: user ? "/history" : "/login?redirect=/history" },
      ] as SidebarItem[],
    },
  ];

  return (
    <Sidebar
      sections={sections}
      trending={{ title: "热门话题 · 本周", entries: trendingTopics }}
    />
  );
}
