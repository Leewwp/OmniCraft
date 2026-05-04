"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import Link from "next/link";

interface Stats {
  total_contents: number;
  total_views: number;
  pending_pr_count: number;
}

export default function DashboardPage() {
  const { user, isLoading } = useAuth();
  const [stats, setStats] = useState<Stats>({ total_contents: 0, total_views: 0, pending_pr_count: 0 });
  const [error, setError] = useState("");

  useEffect(() => {
    if (!user) return;
    void loadStats();
  }, [user]);

  async function loadStats() {
    try {
      const data = await api.get<{ contents?: { id: number }[] }>(`/api/v1/contents?author_id=${user!.id}&page=1&page_size=1`);
      const total = (data as any).total || (data.contents || []).length;
      setStats((prev) => ({ ...prev, total_contents: total }));
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "加载失败");
    }
  }

  if (isLoading) {
    return <div className="mx-auto w-full max-w-4xl px-4 py-6 text-sm text-muted-foreground">加载中...</div>;
  }

  return (
    <div className="mx-auto w-full max-w-4xl space-y-6 px-4 py-6">
      <div className="rounded-md border border-border bg-card p-4 shadow-none">
        <h1 className="text-2xl font-bold tracking-tight">创作者后台</h1>
        <p className="mt-1 text-sm text-muted-foreground">管理你的内容和协同创作</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div className="rounded-md border border-border bg-card p-4 shadow-none">
          <p className="text-xs text-muted-foreground">我的内容</p>
          <p className="text-2xl font-bold">{stats.total_contents}</p>
        </div>
        <div className="rounded-md border border-border bg-card p-4 shadow-none">
          <p className="text-xs text-muted-foreground">总浏览量</p>
          <p className="text-2xl font-bold">{stats.total_views}</p>
        </div>
        <div className="rounded-md border border-border bg-card p-4 shadow-none">
          <p className="text-xs text-muted-foreground">待处理 PR</p>
          <p className="text-2xl font-bold">{stats.pending_pr_count}</p>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Link href="/dashboard/contents" className="rounded-md border border-border bg-card p-4 shadow-none hover:bg-muted/30 transition-colors">
          <h3 className="text-sm font-semibold">我的内容</h3>
          <p className="mt-1 text-xs text-muted-foreground">查看、编辑、下架已发布的内容</p>
        </Link>
        <Link href="/dashboard/pr-requests" className="rounded-md border border-border bg-card p-4 shadow-none hover:bg-muted/30 transition-colors">
          <h3 className="text-sm font-semibold">PR 申请管理</h3>
          <p className="mt-1 text-xs text-muted-foreground">审核协同修改申请</p>
        </Link>
        <Link href="/dashboard/contributors" className="rounded-md border border-border bg-card p-4 shadow-none hover:bg-muted/30 transition-colors">
          <h3 className="text-sm font-semibold">贡献者管理</h3>
          <p className="mt-1 text-xs text-muted-foreground">管理贡献者和拉黑列表</p>
        </Link>
        <Link href="/publish" className="rounded-md border border-border bg-card p-4 shadow-none hover:bg-muted/30 transition-colors">
          <h3 className="text-sm font-semibold">发布新内容</h3>
          <p className="mt-1 text-xs text-muted-foreground">创作并发布新的内容作品</p>
        </Link>
      </div>
    </div>
  );
}
