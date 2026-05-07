"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import Link from "next/link";

interface Stats {
  total_contents: number;
  total_views: number;
  pending_pr_count: number;
  pending_tag_suggestions: number;
}

export default function DashboardPage() {
  const t = useTranslations();
  const { user, isLoading } = useAuth();
  const [stats, setStats] = useState<Stats>({ total_contents: 0, total_views: 0, pending_pr_count: 0, pending_tag_suggestions: 0 });
  const [error, setError] = useState("");

  useEffect(() => {
    if (!user) return;
    void loadStats();
  }, [user]);

  async function loadStats() {
    try {
      interface ContentsPage { contents?: { id: number }[]; total?: number }
      const data = await api.get<ContentsPage>(`/api/v1/contents?author_id=${user!.id}&page=1&page_size=1`);
      const total = data.total ?? (data.contents || []).length;
      setStats((prev) => ({ ...prev, total_contents: total }));
      // Load tag suggestion count in background
      try {
        const sRes = await api.get<{ suggestions?: { id: number }[] }>(
          `/api/v1/dashboard/tag-suggestions?content_id=${user!.id}&all=true`
        );
        setStats((prev) => ({ ...prev, pending_tag_suggestions: (sRes.suggestions || []).length }));
      } catch { /* tag suggestions not available yet */ }
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t('common.loadFailed'));
    }
  }

  if (isLoading) {
    return <div className="mx-auto w-full max-w-4xl px-4 py-6 text-sm text-muted-foreground">{t('common.loading')}</div>;
  }

  return (
    <div className="mx-auto w-full max-w-4xl space-y-6 px-4 py-6">
      <div className="rounded-md border border-border bg-card p-4 shadow-none">
        <h1 className="text-2xl font-bold tracking-tight">{t('dashboard.title')}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t('dashboard.subtitle')}</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div className="rounded-md border border-border bg-card p-4 shadow-none">
          <p className="text-xs text-muted-foreground">{t('dashboard.myContent')}</p>
          <p className="text-2xl font-bold">{stats.total_contents}</p>
        </div>
        <div className="rounded-md border border-border bg-card p-4 shadow-none">
          <p className="text-xs text-muted-foreground">{t('dashboard.totalViews')}</p>
          <p className="text-2xl font-bold">{stats.total_views}</p>
        </div>
        <div className="rounded-md border border-border bg-card p-4 shadow-none">
          <p className="text-xs text-muted-foreground">{t('dashboard.pendingPrs')}</p>
          <p className="text-2xl font-bold">{stats.pending_pr_count}</p>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Link href="/dashboard/contents" className="rounded-md border border-border bg-card p-4 shadow-none hover:bg-muted/30 transition-colors">
          <h3 className="text-sm font-semibold">{t('dashboard.myContent')}</h3>
          <p className="mt-1 text-xs text-muted-foreground">{t('dashboard.manageContent')}</p>
        </Link>
        <Link href="/dashboard/pr-requests" className="rounded-md border border-border bg-card p-4 shadow-none hover:bg-muted/30 transition-colors">
          <h3 className="text-sm font-semibold">{t('dashboard.prManagement')}</h3>
          <p className="mt-1 text-xs text-muted-foreground">{t('dashboard.prDesc')}</p>
        </Link>
        <Link href="/dashboard/contributors" className="rounded-md border border-border bg-card p-4 shadow-none hover:bg-muted/30 transition-colors">
          <h3 className="text-sm font-semibold">{t('dashboard.contributorManagement')}</h3>
          <p className="mt-1 text-xs text-muted-foreground">{t('dashboard.contributorDesc')}</p>
        </Link>
        <Link href="/publish" className="rounded-md border border-border bg-card p-4 shadow-none hover:bg-muted/30 transition-colors">
          <h3 className="text-sm font-semibold">{t('dashboard.publishNew')}</h3>
          <p className="mt-1 text-xs text-muted-foreground">{t('dashboard.publishDesc')}</p>
        </Link>
        <Link href="/dashboard/tag-suggestions" className="rounded-md border border-border bg-card p-4 shadow-none hover:bg-muted/30 transition-colors">
          <h3 className="text-sm font-semibold">{t('dashboard.tagSuggestions')}</h3>
          <p className="mt-1 text-xs text-muted-foreground">{t('dashboard.tagSuggestionsDesc')}</p>
          {stats.pending_tag_suggestions > 0 && (
            <span className="mt-2 inline-block rounded bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-900/30 dark:text-amber-400">
              {t('dashboard.pendingCount', { count: stats.pending_tag_suggestions })}
            </span>
          )}
        </Link>
      </div>
    </div>
  );
}
