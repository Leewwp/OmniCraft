"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { MessageSquare, Pin, Plus, Search } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { SortSelect as SharedSortSelect } from "@/components/ui/SortSelect";
import { DiscussionDetailOverlay } from "@/components/ip/hub/DiscussionDetailOverlay";

interface DiscussionRow {
  id: number;
  title: string;
  body?: string;
  is_pinned?: boolean;
  reply_count?: number;
  view_count?: number;
  created_at?: string;
  author?: { id?: number; username?: string; avatar_url?: string };
}

const SORTS = [
  { value: "latest_reply", labelKey: "ip.hubSortLatestReply" },
  { value: "newest_post", labelKey: "ip.hubSortNewestPost" },
  { value: "most_replies", labelKey: "ip.hubSortMostReplies" },
  { value: "hot", labelKey: "ip.hubSortHot" },
];

interface IPDiscussionsTabProps {
  ipId: number;
  apiBase: string;
  query: string;
  sort: string;
  onSortChange: (next: string) => void;
  // ?d=<id>：从「发起讨论」页创建成功跳回时直接打开该帖浮层
  initialDiscussionId?: number | null;
}

// 讨论区 tab：四排序（含热门）+ 置顶优先 + IP 内搜索；帖详情浮层（含回帖）。
// 排序状态由 URL query 驱动（#290 单页契约）。
export function IPDiscussionsTab({ ipId, apiBase, query, sort, onSortChange, initialDiscussionId }: IPDiscussionsTabProps) {
  const t = useTranslations();
  const [discussions, setDiscussions] = useState<DiscussionRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeId, setActiveId] = useState<number | null>(initialDiscussionId ?? null);
  // 请求序号守卫：快速切换排序/搜索时丢弃过期响应
  const fetchSeqRef = useRef(0);

  const fetchDiscussions = useCallback(async (nextSort: string, q: string) => {
    const seq = ++fetchSeqRef.current;
    setLoading(true);
    const params = new URLSearchParams({ sort: nextSort, page_size: "30" });
    if (q) params.set("q", q);
    try {
      const res = await fetch(`${apiBase}/ips/${ipId}/discussions?${params.toString()}`, { cache: "no-store" });
      if (!res.ok) throw new Error("FETCH_FAILED");
      const data = (await res.json()) as { discussions?: DiscussionRow[] };
      if (seq !== fetchSeqRef.current) return;
      setDiscussions(data.discussions ?? []);
    } catch {
      if (seq !== fetchSeqRef.current) return;
      setDiscussions([]);
    } finally {
      if (seq === fetchSeqRef.current) setLoading(false);
    }
  }, [apiBase, ipId]);

  useEffect(() => {
    void fetchDiscussions(sort, query);
  }, [sort, query, fetchDiscussions]);

  return (
    <section className="space-y-3" aria-label={t('ip.hubTab_discussions')}>
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-base font-semibold">{t('ip.hubTab_discussions')}</h2>
        <div className="flex items-center gap-2">
          {/* 发起讨论走既有受保护页（登录由该页 redirect 兜底），沿用旧讨论区入口 */}
          <Link
            href={`/ip/${ipId}/discussions/new`}
            className={buttonVariants({ size: "sm" })}
          >
            <Plus className="h-3.5 w-3.5" aria-hidden="true" />
            {t('discussion.newPost')}
          </Link>
          <SharedSortSelect
            ariaLabel={t('common.sortLabel')}
            value={sort}
            options={SORTS.map((s) => ({ value: s.value, label: t(s.labelKey) }))}
            onChange={onSortChange}
          />
        </div>
      </div>

      {loading ? (
        <div className="space-y-2" aria-busy="true" aria-label={t('common.loading')}>
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="rounded-md border border-border bg-card p-4">
              <Skeleton className="h-5 w-2/3" />
              <Skeleton className="mt-2 h-3 w-1/3" />
            </div>
          ))}
        </div>
      ) : discussions.length === 0 ? (
        <EmptyState
          icon={Search}
          title={query ? t('ip.hubNoSearchResult', { q: query }) : t('ip.hubEmptyDiscussions')}
          description={query ? t('ip.hubNoSearchHint') : t('ip.hubEmptyDiscussionsHint')}
        />
      ) : (
        <div className="space-y-2">
          {discussions.map((d) => (
            <button
              key={d.id}
              type="button"
              onClick={() => setActiveId(d.id)}
              className="block w-full rounded-md border border-border bg-card p-4 text-left transition-colors duration-150 hover:border-accent/20 hover:bg-muted/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <div className="flex items-center gap-2">
                {d.is_pinned && (
                  <span className="inline-flex items-center gap-1 rounded-full border border-accent-emphasis bg-accent-subtle px-2 py-0.5 text-[10px] font-semibold text-accent-emphasis">
                    <Pin className="h-3 w-3" aria-hidden="true" />
                    {t('ip.hubPinned')}
                  </span>
                )}
                <h3 className="min-w-0 flex-1 truncate text-sm font-medium text-foreground">{d.title}</h3>
              </div>
              <div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
                <span>{d.author?.username ?? ""}</span>
                <span className="inline-flex items-center gap-1">
                  <MessageSquare className="h-3 w-3" aria-hidden="true" />
                  {d.reply_count ?? 0}
                </span>
              </div>
            </button>
          ))}
        </div>
      )}

      {activeId != null && (
        <DiscussionDetailOverlay
          discussionId={activeId}
          onClose={() => setActiveId(null)}
        />
      )}
    </section>
  );
}
