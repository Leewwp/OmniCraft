"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { DiscussionCard } from "@/components/social/DiscussionCard";
import { Button } from "@/components/ui/button";
import { DataList } from "@/components/ui/data-list";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/Toast";
import { MessageSquare, Plus, Search } from "lucide-react";
import { silentError } from "@/lib/error-handler";

interface DiscussionData {
  id: number;
  title: string;
  ip_id?: number;
  author?: { id?: number; username?: string };
  reply_count?: number;
  last_active_at?: string;
  created_at?: string;
}

export default function DiscussionsPage() {
  const t = useTranslations();
  const params = useParams<{ ipId: string }>();
  const ipId = parseInt(params.ipId, 10);
  const { user } = useAuth();
  const { toast } = useToast();
  const [discussions, setDiscussions] = useState<DiscussionData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const searchRef = useRef(search);
  searchRef.current = search;
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);

  const load = useCallback(async (searchQuery?: string, nextPage = 1, append = false) => {
    if (append) setLoadingMore(true); else setLoading(true);
    setError("");
    setPage(nextPage);
    try {
      const q = (searchQuery ?? searchRef.current).trim();
      const path = q
        ? `/search?q=${encodeURIComponent(q)}&page=${nextPage}&page_size=20`
        : `?page=${nextPage}&page_size=20`;
      const res = await api.get<{ discussions?: DiscussionData[]; total?: number; page_size?: number }>(
        `/api/v1/ips/${ipId}/discussions${path}`,
      );
      const incoming = res.discussions ?? [];
      setDiscussions((current) => append ? [...current, ...incoming.filter((item) => !current.some((existing) => existing.id === item.id))] : incoming);
      setPage(nextPage);
      setHasMore((res.total ?? incoming.length) > nextPage * (res.page_size ?? 20));
    } catch (e) {
      silentError(e, { component: 'DiscussionsPage', action: 'load' });
      const message = t(getUserFacingErrorKey(e, "common.loadFailed"));
      setError(message);
      toast("error", message);
    } finally {
      setLoadingMore(false);
      setLoading(false);
    }
  }, [ipId, t, toast]);

  useEffect(() => { void load(); }, [load]);

  return (
    <div className="mx-auto w-full max-w-full min-[701px]:max-w-[720px] min-[1101px]:max-w-[960px] space-y-4 px-4 py-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("discussion.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("discussion.subtitle")}</p>
        </div>
        {user && (
          <Link href={`/ip/${ipId}/discussions/new`}>
            <Button size="sm"><Plus className="mr-1 h-4 w-4" />{t("discussion.newPost")}</Button>
          </Link>
        )}
      </div>

      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("discussion.searchPlaceholder")}
            className="w-full rounded-md border border-border bg-background py-2 pl-10 pr-4 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
          />
        </div>
        <Button size="sm" variant="outline" onClick={() => load()}>{t("discussion.search")}</Button>
      </div>

      <DataList
        items={discussions}
        loading={loading}
        error={error}
        onRetry={() => void load(undefined, page, page > 1)}
        hasMore={hasMore}
        loadingMore={loadingMore}
        onLoadMore={() => load(undefined, page + 1, true)}
        empty={<EmptyState icon={MessageSquare} title={t("discussion.empty")} />}
        loadingState={<div className="space-y-3"><Skeleton className="h-24 w-full" /><Skeleton className="h-24 w-full" /><Skeleton className="h-24 w-full" /></div>}
        getKey={(discussion) => discussion.id}
        renderItem={(discussion) => <DiscussionCard key={discussion.id} data={discussion} />}
      />
    </div>
  );
}
