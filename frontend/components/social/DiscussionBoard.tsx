"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { silentError } from "@/lib/error-handler";
import { DiscussionCard } from "@/components/social/DiscussionCard";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Plus, Search, ArrowRight, MessageSquareText } from "lucide-react";
import { cn } from "@/lib/utils";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";

interface DiscussionData {
  id: number;
  title: string;
  ip_id?: number;
  author?: { id?: number; username?: string };
  reply_count?: number;
  last_active_at?: string;
  created_at?: string;
}

interface DiscussionBoardProps {
  ipId: number;
  compact?: boolean;
  className?: string;
}

function BoardSkeleton() {
  return (
    <div className="space-y-3">
      {Array.from({ length: 4 }).map((_, i) => (
        <div key={i} className="rounded-md border border-border bg-card p-4">
          <Skeleton className="h-4 w-3/4" />
          <div className="mt-2 flex gap-3">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-3 w-12" />
            <Skeleton className="h-3 w-20" />
          </div>
        </div>
      ))}
    </div>
  );
}

export function DiscussionBoard({ ipId, compact = false, className }: DiscussionBoardProps) {
  const t = useTranslations();
  const { user } = useAuth();
  const [discussions, setDiscussions] = useState<DiscussionData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [busy, setBusy] = useState(false);
  const searchRef = useRef(search);
  searchRef.current = search;

  const load = useCallback(async (searchQuery?: string) => {
    setLoading(true);
    setError("");
    try {
      const q = (searchQuery ?? searchRef.current).trim();
      const path = q ? `/search?q=${encodeURIComponent(q)}` : "";
      const res = await api.get<{ discussions?: DiscussionData[] }>(
        `/api/v1/ips/${ipId}/discussions${path}`,
      );
      setDiscussions(res.discussions ?? []);
    } catch (e) {
      setError(t(getUserFacingErrorKey(e, "common.loadFailed")));
      silentError(e, { component: 'DiscussionBoard', action: 'load' });
    } finally {
      setLoading(false);
    }
  }, [ipId, t]);

  useEffect(() => { load(); }, [load]);

  async function handleSearch() {
    setBusy(true);
    await load();
    setBusy(false);
  }

  const displayedDiscussions = compact ? discussions.slice(0, 5) : discussions;

  if (compact && !loading && discussions.length === 0) {
    return null;
  }

  return (
    <div className={cn("space-y-4", className)}>
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold tracking-tight">{t("discussion.title")}</h2>
          {!compact && (
            <p className="mt-1 text-sm text-muted-foreground">{t("discussion.subtitle")}</p>
          )}
        </div>
        <div className="flex items-center gap-2">
          {!compact && user && (
            <Link href={`/ip/${ipId}/discussions/new`}>
              <Button size="sm">
                <Plus className="mr-1 h-4 w-4" />
                {t("discussion.newPost")}
              </Button>
            </Link>
          )}
          {compact && discussions.length > 0 && (
            <Link
              href={`/ip/${ipId}/discussions`}
              className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-accent transition-colors"
            >
              {t("common.viewAll")}
              <ArrowRight className="h-3 w-3" />
            </Link>
          )}
        </div>
      </div>

      {/* Search bar (full mode only) */}
      {!compact && (
        <div className="flex items-center gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") handleSearch(); }}
              placeholder={t("discussion.searchPlaceholder")}
              className="w-full rounded-md border border-border bg-background py-2 pl-10 pr-4 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
            />
          </div>
          <Button size="sm" variant="outline" onClick={handleSearch} disabled={busy}>
            {t("discussion.search")}
          </Button>
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
          <button
            className="ml-2 underline hover:no-underline"
            onClick={() => load()}
          >
            {t("common.retry")}
          </button>
        </div>
      )}

      {/* Content */}
      {loading ? (
        <BoardSkeleton />
      ) : displayedDiscussions.length === 0 ? (
        <EmptyState
          icon={MessageSquareText}
          title={t("discussion.empty")}
          description={t("discussion.emptyHint")}
          className="px-4 py-10"
          action={
            !compact && user ? (
              <Link href={`/ip/${ipId}/discussions/new`}>
                <Button size="sm" variant="outline">
                  <Plus className="mr-1 h-4 w-4" />
                  {t("discussion.newPost")}
                </Button>
              </Link>
            ) : null
          }
        />
      ) : (
        <div className="space-y-3">
          {displayedDiscussions.map((d) => (
            <DiscussionCard key={d.id} data={d} />
          ))}
        </div>
      )}

      {/* Compact mode: "view full board" link */}
      {compact && discussions.length >= 5 && (
        <Link
          href={`/ip/${ipId}/discussions`}
          className="block rounded-md border border-border bg-card px-4 py-3 text-center text-sm text-muted-foreground hover:text-accent hover:border-accent/20 transition-colors"
        >
          {t("common.viewAll")} ({discussions.length})
        </Link>
      )}
    </div>
  );
}
