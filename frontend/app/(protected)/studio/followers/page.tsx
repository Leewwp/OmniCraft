"use client";

import { useEffect, useState } from "react";
import { Users, UserPlus, UserMinus } from "lucide-react";
import { api } from "@/lib/api";
import { StatsCard } from "@/components/studio/StatsCard";
import { Skeleton } from "@/components/ui/skeleton";

export default function StudioFollowersPage() {
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState({
    total: 0,
    newThisMonth: 0,
    lostThisMonth: 0,
  });

  useEffect(() => {
    async function fetchData() {
      try {
        const res = await api.get("/api/v1/users/me/followers/stats?days=30") as Record<string, unknown> | null;
        if (res) {
          setStats({
            total: (res.total as number) ?? 0,
            newThisMonth: (res.new_this_month as number) ?? 0,
            lostThisMonth: (res.lost_this_month as number) ?? 0,
          });
        }
      } catch {
        // Default stats
      } finally {
        setLoading(false);
      }
    }
    fetchData();
  }, []);

  if (loading) {
    return (
      <div>
        <h1 className="mb-6 text-xl font-bold text-foreground">粉丝分析</h1>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-28 rounded-lg" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div>
      <h1 className="mb-1 text-xl font-bold text-foreground">粉丝分析</h1>
      <p className="mb-6 text-sm text-muted-foreground">了解你的粉丝增长趋势</p>

      <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatsCard
          label="总粉丝数"
          value={stats.total.toLocaleString()}
          icon={<Users className="h-5 w-5" />}
        />
        <StatsCard
          label="本月新增"
          value={stats.newThisMonth.toLocaleString()}
          change={stats.total > 0 ? Math.round((stats.newThisMonth / stats.total) * 100) : 0}
          icon={<UserPlus className="h-5 w-5" />}
        />
        <StatsCard
          label="本月流失"
          value={stats.lostThisMonth.toLocaleString()}
          change={stats.total > 0 ? -Math.round((stats.lostThisMonth / stats.total) * 100) : 0}
          icon={<UserMinus className="h-5 w-5" />}
        />
      </div>

      <div className="rounded-lg border border-border bg-card p-8 text-center">
        <p className="text-sm text-muted-foreground">
          粉丝趋势图表将在数据充足时显示 <span className="text-amber-500">(P1)</span>
        </p>
      </div>
    </div>
  );
}
