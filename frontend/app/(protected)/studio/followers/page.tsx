"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Users, UserPlus, UserMinus } from "lucide-react";
import { api } from "@/lib/api";
import { StatsCard } from "@/components/studio/StatsCard";
import { FollowerTrendChart } from "@/components/studio/FollowerTrendChart";
import { FollowerSourceChart } from "@/components/studio/FollowerSourceChart";
import { DataList } from "@/components/ui/data-list";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/Toast";

export default function StudioFollowersPage() {
  const t = useTranslations();
  const { toast } = useToast();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [stats, setStats] = useState({
    total: 0,
    newThisMonth: 0,
    lostThisMonth: 0,
  });
  const [trend, setTrend] = useState<Array<{ date: string; newFollowers: number; netGrowth: number }>>([]);
  const [sources, setSources] = useState<Array<{ name: string; value: number }>>([]);

  const fetchData = useCallback(async () => {
      setLoading(true);
      setError("");
      try {
        const res = await api.get("/api/v1/users/me/followers/stats?days=30") as Record<string, unknown> | null;
        if (res) {
          setStats({
            total: (res.total as number) ?? 0,
            newThisMonth: (res.new_this_month as number) ?? 0,
            lostThisMonth: (res.lost_this_month as number) ?? 0,
          });
          const daily = res.daily as Array<{ date: string; count?: number; lost?: number }> | undefined;
          if (daily) {
            setTrend(
              daily.map((d) => ({
                date: d.date || "",
                newFollowers: d.count ?? 0,
                netGrowth: (d.count ?? 0) - (d.lost ?? 0),
              }))
            );
          }
          const sourceData = res.sources as Array<{ name: string; value: number }> | undefined;
          if (sourceData) {
            setSources(sourceData);
          }
        }
      } catch {
        const message = t("common.loadFailed");
        setError(message);
        toast("error", message);
      } finally {
        setLoading(false);
      }
  }, [t, toast]);

  useEffect(() => { void fetchData(); }, [fetchData]);

  if (loading) {
    return (
      <div>
        <h1 className="mb-6 text-xl font-bold text-foreground">{t('studio.followers.title')}</h1>
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
      <h1 className="mb-1 text-xl font-bold text-foreground">{t('studio.followers.title')}</h1>
      <p className="mb-6 text-sm text-muted-foreground">{t('studio.followers.subtitle')}</p>

      <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatsCard
          label={t('studio.followers.totalFollowers')}
          value={stats.total.toLocaleString()}
          icon={<Users className="h-5 w-5" />}
        />
        <StatsCard
          label={t('studio.followers.newThisMonth')}
          value={stats.newThisMonth.toLocaleString()}
          change={stats.total > 0 ? Math.round((stats.newThisMonth / stats.total) * 100) : 0}
          icon={<UserPlus className="h-5 w-5" />}
        />
        <StatsCard
          label={t('studio.followers.lostThisMonth')}
          value={stats.lostThisMonth.toLocaleString()}
          change={stats.total > 0 ? -Math.round((stats.lostThisMonth / stats.total) * 100) : 0}
          icon={<UserMinus className="h-5 w-5" />}
        />
      </div>

      <div className="mb-6">
        <FollowerTrendChart data={trend} />
      </div>
      <DataList
        items={sources.length > 0 ? [sources] : []}
        loading={loading}
        error={error}
        onRetry={() => void fetchData()}
        empty={<EmptyState icon={Users} title={t('studio.followers.noSourceData')} />}
        renderItem={(sourceData) => <FollowerSourceChart data={sourceData} />}
      />
    </div>
  );
}
