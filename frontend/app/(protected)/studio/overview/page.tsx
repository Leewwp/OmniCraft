"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { FileText, Eye, Heart, Users } from "lucide-react";
import { api } from "@/lib/api";
import { StatsCard } from "@/components/studio/StatsCard";
import { ContentRankList } from "@/components/studio/ContentRankList";
import { PendingTasksCard } from "@/components/studio/PendingTasksCard";
import { ViewsTrendChart } from "@/components/studio/ViewsTrendChart";
import { Skeleton } from "@/components/ui/skeleton";

export default function StudioOverviewPage() {
  const t = useTranslations();
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState({
    totalContents: 0,
    totalViews: 0,
    totalLikes: 0,
    followers: 0,
  });
  const [topContent, setTopContent] = useState<Array<{ id: number; title: string; viewCount: number; zone?: string }>>([]);
  const [pendingTasks, setPendingTasks] = useState<Array<{ type: "pr" | "tag"; id: number; title: string }>>([]);
  const [viewsTrend, setViewsTrend] = useState<Array<{ date: string; views: number }>>([]);

  useEffect(() => {
    async function fetchData() {
      try {
        const contentsRes = await api.get("/api/v1/my/contents?limit=5&sort=popular") as Record<string, unknown>;
        const data = (contentsRes?.data as Array<Record<string, unknown>>) || [];
        const meta = contentsRes?.meta as Record<string, unknown> | undefined;
        setStats({
          totalContents: (meta?.total as number) ?? 0,
          totalViews: data.reduce((a, c) => a + ((c.view_count as number) || 0), 0),
          totalLikes: data.reduce((a, c) => a + ((c.like_count as number) || 0), 0),
          followers: 0,
        });
        setTopContent(
          data.slice(0, 5).map((c) => ({
            id: c.id as number,
            title: c.title as string,
            viewCount: (c.view_count as number) || 0,
            zone: c.zone as string,
          }))
        );
      } catch {
        // Use default stats on error
      }

      // Fetch views trend
      try {
        const trendRes = await api.get("/api/v1/users/me/followers/stats?days=30") as Record<string, unknown> | null;
        if (trendRes?.daily) {
          setViewsTrend(
            (trendRes.daily as Array<{ date: string; views?: number; count?: number }>).map((d) => ({
              date: d.date,
              views: d.views ?? d.count ?? 0,
            }))
          );
        }
      } catch {
        // chart stays empty
      }

      setLoading(false);
    }
    fetchData();
  }, []);

  if (loading) {
    return (
      <div>
        <h1 className="mb-6 text-xl font-bold text-foreground">数据概览</h1>
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-28 rounded-lg" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">数据概览</h1>
      <p className="mb-6 text-sm text-muted-foreground">查看你的创作数据总览</p>

      {/* Stats cards */}
      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatsCard
          label="总内容数"
          value={stats.totalContents}
          icon={<FileText className="h-5 w-5" />}
        />
        <StatsCard
          label="总访问量"
          value={stats.totalViews.toLocaleString()}
          change={12}
          icon={<Eye className="h-5 w-5" />}
        />
        <StatsCard
          label="总获赞"
          value={stats.totalLikes.toLocaleString()}
          change={8}
          icon={<Heart className="h-5 w-5" />}
        />
        <StatsCard
          label="粉丝数"
          value={stats.followers}
          icon={<Users className="h-5 w-5" />}
        />
      </div>

      {/* Views trend chart */}
      <div className="mb-6">
        <ViewsTrendChart data={viewsTrend} />
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <ContentRankList items={topContent} />
        <PendingTasksCard items={pendingTasks} />
      </div>
    </div>
  );
}
