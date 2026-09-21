"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { FileText, Eye, Heart, Users } from "lucide-react";
import { api } from "@/lib/api";
import { StatsCard } from "@/components/studio/StatsCard";
import { PendingTasksCard } from "@/components/studio/PendingTasksCard";
import { FollowerGainTrendChart } from "@/components/studio/FollowerGainTrendChart";
import { DataList } from "@/components/ui/data-list";
import { EmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/Toast";

export default function StudioOverviewPage() {
  const t = useTranslations();
  const { toast } = useToast();
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState({
    totalContents: 0,
    totalViews: 0,
    totalLikes: 0,
    followers: 0,
  });
  const [topContent, setTopContent] = useState<Array<{ id: number; title: string; viewCount: number; zone?: string }>>([]);
  const [topPage, setTopPage] = useState(1);
  const [topHasMore, setTopHasMore] = useState(false);
  const [topLoadingMore, setTopLoadingMore] = useState(false);
  const [topError, setTopError] = useState("");
  const [pendingTasks, setPendingTasks] = useState<Array<{ type: "pr" | "tag"; id: number; title: string }>>([]);
  const [followerTrend, setFollowerTrend] = useState<Array<{ date: string; count: number }>>([]);
  const [trendError, setTrendError] = useState("");
  const topContentRef = useRef(topContent);
  topContentRef.current = topContent;

  const loadContents = useCallback(async (nextPage = 1, append = false) => {
    if (append) setTopLoadingMore(true); else setLoading(topContentRef.current.length === 0);
    setTopError("");
    setTopPage(nextPage);
    try {
      const contentsRes = await api.get(`/api/v1/users/me/contents?page=${nextPage}&page_size=5&sort=popular`) as Record<string, unknown>;
      const data = ((contentsRes?.contents ?? contentsRes?.data) as Array<Record<string, unknown>>) || [];
      const meta = contentsRes?.meta as Record<string, unknown> | undefined;
      const total = (contentsRes?.total as number) ?? (meta?.total as number) ?? data.length;
      const pageSize = (contentsRes?.page_size as number) ?? (meta?.page_size as number) ?? 5;
      // 概览卡的全量合计来自后端 totals 聚合（分页数据只覆盖当前页，
      // reduce 首页 5 条会产出失真数字——SP-25 中-10①）。
      const totals = contentsRes?.totals as { views?: number; likes?: number } | undefined;
      const incoming = data.map((c) => ({
        id: c.id as number,
        title: c.title as string,
        viewCount: (c.view_count as number) || 0,
        zone: c.zone as string,
      }));
      setTopContent((current) => append
        ? [...current, ...incoming.filter((item) => !current.some((existing) => existing.id === item.id))]
        : incoming);
      setTopPage(nextPage);
      setTopHasMore(total > nextPage * pageSize);
      if (!append) {
        setStats((current) => ({
          ...current,
          totalContents: total,
          totalViews: totals?.views ?? current.totalViews,
          totalLikes: totals?.likes ?? current.totalLikes,
        }));
      }
    } catch {
      const message = t("common.loadFailed");
      setTopError(message);
      toast("error", message);
    } finally {
      setTopLoadingMore(false);
      setLoading(false);
    }
  }, [t, toast]);

  // T49 (FIX-22c): 待办卡真实数据——一次请求聚合 open PR + pending 标签建议
  const loadPendingTasks = useCallback(async () => {
    try {
      const res = await api.get("/api/v1/users/me/pending-tasks") as { tasks?: Array<{ type: "pr" | "tag"; id: number; title: string }> } | null;
      setPendingTasks(res?.tasks ?? []);
    } catch {
      // 待办卡为辅助信息，加载失败静默保持空态
      setPendingTasks([]);
    }
  }, []);

  // 粉丝分析端点一次供给两处数据：total → 粉丝卡（中-10②，不再恒 0）；
  // daily[].count → 新增粉丝趋势（中-10④，标题与数据源一致）。
  const loadTrend = useCallback(async () => {
    setTrendError("");
    try {
      const trendRes = await api.get("/api/v1/users/me/followers/stats?days=30") as Record<string, unknown> | null;
      if (trendRes) {
        setStats((current) => ({ ...current, followers: (trendRes.total as number) ?? current.followers }));
      }
      if (trendRes?.daily) {
        setFollowerTrend(
          (trendRes.daily as Array<{ date: string; count?: number }>).map((d) => ({
            date: d.date,
            count: d.count ?? 0,
          }))
        );
      }
    } catch {
      const message = t("common.loadFailed");
      setTrendError(message);
      toast("error", message);
    }
  }, [t, toast]);

  useEffect(() => {
    async function fetchData() {
      await loadContents();
      await loadTrend();
    }
    void fetchData();
    void loadPendingTasks();
  }, [loadContents, loadTrend, loadPendingTasks]);

  /*
   * The ranking endpoint is paginated even though the first view only shows
   * five rows. DataList owns the presentation and next-page affordance while
   * this page keeps the page cursor and fetch semantics.
   */
  function renderRankItem(item: { id: number; title: string; viewCount: number; zone?: string }, index: number) {
    return (
      <Link
        href={item.zone === "original" ? `/original/${item.id}` : `/content/${item.id}`}
        className="flex items-center gap-3 rounded-md p-1.5 -mx-1.5 transition-colors hover:bg-muted"
      >
        <span className="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full bg-muted text-xs font-bold text-muted-foreground">
          {index + 1}
        </span>
        <span className="flex-1 truncate text-sm text-foreground">{item.title}</span>
        <span className="flex items-center gap-1 text-xs text-muted-foreground flex-shrink-0">
          <Eye className="h-3 w-3" />
          {item.viewCount.toLocaleString()}
        </span>
      </Link>
    );
  }

  if (loading) {
    return (
      <div>
        <h1 className="mb-6 text-xl font-bold text-foreground">{t('studio.overview.title')}</h1>
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
      <h1 className="mb-1 text-xl font-bold text-foreground">{t('studio.overview.title')}</h1>
      <p className="mb-6 text-sm text-muted-foreground">{t('studio.overview.subtitle')}</p>

      {/* Stats cards — change 徽标已移除（中-10③）：没有真实环比数据源，
          硬编码的 +12%/+8% 是假增长率；接入真实环比时再恢复。 */}
      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatsCard
          label={t('studio.overview.totalContents')}
          value={stats.totalContents}
          icon={<FileText className="h-5 w-5" />}
        />
        <StatsCard
          label={t('studio.overview.totalViews')}
          value={stats.totalViews.toLocaleString()}
          icon={<Eye className="h-5 w-5" />}
        />
        <StatsCard
          label={t('studio.overview.totalLikes')}
          value={stats.totalLikes.toLocaleString()}
          icon={<Heart className="h-5 w-5" />}
        />
        <StatsCard
          label={t('studio.overview.followers')}
          value={stats.followers}
          icon={<Users className="h-5 w-5" />}
        />
      </div>

      {/* 新增粉丝趋势（数据源 = 粉丝分析端点 daily.count，标题一致） */}
      <div className="mb-6">
        {trendError ? (
          <div className="rounded-lg border border-destructive/40 bg-destructive/5 p-5" role="alert">
            <p className="text-sm text-destructive">{trendError}</p>
            <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => void loadTrend()}>
              {t("common.retry")}
            </Button>
          </div>
        ) : <FollowerGainTrendChart data={followerTrend} />}
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <div className="rounded-lg border border-border bg-card p-5">
          <h3 className="mb-4 text-sm font-semibold text-foreground">{t('studio.overview.topContent')}</h3>
          <DataList
            items={topContent}
            loading={loading}
            error={topError}
            onRetry={() => void loadContents(topPage, topPage > 1)}
            hasMore={topHasMore}
            loadingMore={topLoadingMore}
            onLoadMore={() => loadContents(topPage + 1, true)}
            empty={<EmptyState title={t('studio.overview.noData')} action={<Link href="/studio/publish/original"><Button size="sm">{t('studio.contents.startCreating')}</Button></Link>} />}
            loadingState={<div className="space-y-3">{[1, 2, 3].map((i) => <Skeleton key={i} className="h-8 w-full" />)}</div>}
            getKey={(item) => item.id}
            renderItem={renderRankItem}
          />
        </div>
        <PendingTasksCard items={pendingTasks} />
      </div>
    </div>
  );
}
