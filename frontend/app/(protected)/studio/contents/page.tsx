"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { Eye, Heart, MessageCircle, Edit, Trash2, FileText } from "lucide-react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { DataList } from "@/components/ui/data-list";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";

export default function StudioContentsPage() {
  const t = useTranslations();
  const [contents, setContents] = useState<Array<{
    id: number; title: string; zone: string; content_type: string;
    view_count: number; like_count: number; comment_count: number;
    status: string;
    ban_reason?: string;
  }>>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const contentsRef = useRef(contents);
  contentsRef.current = contents;

  const load = useCallback(async (nextPage = 1, append = false) => {
    if (append) setLoadingMore(true); else setLoading(contentsRef.current.length === 0);
    setError("");
    setPage(nextPage);
    try {
      const res = await api.get(`/api/v1/users/me/contents?page=${nextPage}&page_size=20`) as Record<string, unknown>;
      const rawItems = (res?.contents ?? res?.data) as Array<Record<string, unknown>> | undefined;
      const incoming = (rawItems || []).map((c) => ({
        id: c.id as number, title: c.title as string, zone: c.zone as string,
        content_type: c.content_type as string, view_count: c.view_count as number,
        like_count: c.like_count as number, comment_count: c.comment_count as number,
        status: c.status as string,
        ban_reason: typeof c.ban_reason === "string" ? c.ban_reason : undefined,
      }));
      const meta = res?.meta as Record<string, unknown> | undefined;
      const total = (res?.total as number) ?? (meta?.total as number) ?? incoming.length;
      const pageSize = (res?.page_size as number) ?? (meta?.page_size as number) ?? 20;
      setContents((current) => append
        ? [...current, ...incoming.filter((item) => !current.some((existing) => existing.id === item.id))]
        : incoming);
      setPage(nextPage);
      setHasMore(total > nextPage * pageSize);
    } catch {
      setError(t("common.loadFailed"));
    } finally {
      setLoadingMore(false);
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { void load(); }, [load]);

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-foreground">{t('studio.contents.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t('studio.contents.subtitle')}</p>
        </div>
        <Link href="/studio/publish/original">
          <Button size="sm">{t('studio.contents.publishNew')}</Button>
        </Link>
      </div>

      <DataList
        items={contents}
        loading={loading}
        error={error}
        onRetry={() => void load(page, page > 1)}
        hasMore={hasMore}
        loadingMore={loadingMore}
        onLoadMore={() => load(page + 1, true)}
        empty={
          <EmptyState
            icon={FileText}
            title={t('studio.contents.noContent')}
            action={<Link href="/studio/publish/original"><Button>{t('studio.contents.startCreating')}</Button></Link>}
          />
        }
        loadingState={<div className="space-y-3">{[1, 2, 3].map((i) => <Skeleton key={i} className="h-20 rounded-lg" />)}</div>}
        getKey={(item) => item.id}
        renderItem={(item) => (
            <div
              key={item.id}
              className="flex items-center gap-4 rounded-lg border border-border bg-card p-4"
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <Link
                    href={`/content/${item.id}`}
                    className="text-sm font-medium text-foreground hover:text-primary truncate"
                  >
                    {item.title}
                  </Link>
                  <Badge variant="secondary" className="text-[10px]">{item.zone}</Badge>
                  <Badge variant="outline" className="text-[10px]">{item.content_type}</Badge>
                  {item.status === "banned" && (
                    <Badge className="bg-destructive/10 text-destructive border border-destructive/30 text-[10px]">
                      {t('studio.contents.banned')}
                    </Badge>
                  )}
                </div>
                {item.status === "banned" && item.ban_reason && (
                  <p className="mt-1 text-xs text-destructive">{t('studio.contents.banReason')}: {item.ban_reason}</p>
                )}
                <div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
                  <span className="inline-flex items-center gap-1"><Eye className="h-3 w-3" /> {item.view_count}</span>
                  <span className="inline-flex items-center gap-1"><Heart className="h-3 w-3" /> {item.like_count}</span>
                  <span className="inline-flex items-center gap-1"><MessageCircle className="h-3 w-3" /> {item.comment_count}</span>
                </div>
              </div>
              <div className="flex items-center gap-1">
                <Button variant="ghost" size="icon-sm" title={t('studio.contents.edit')}>
                  <Edit className="h-3.5 w-3.5" />
                </Button>
                <Button variant="ghost" size="icon-sm" title={t('studio.contents.delete')}>
                  <Trash2 className="h-3.5 w-3.5 text-destructive" />
                </Button>
              </div>
            </div>
        )}
      />
    </div>
  );
}
