"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { DataList } from "@/components/ui/data-list";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import Link from "next/link";
import { Inbox, MessageSquare, ChevronRight } from "lucide-react";

interface FeedbackTicket {
  id: number;
  category: string;
  title: string;
  status: string;
  priority: string;
  created_at: string;
}

export default function FeedbackMinePage() {
  const t = useTranslations();
  const { user } = useAuth();
  const [tickets, setTickets] = useState<FeedbackTicket[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);

  const load = useCallback(async (nextPage = 1, append = false) => {
    if (append) setLoadingMore(true); else setLoading(true);
    setError("");
    setPage(nextPage);
    try {
      const res = (await api.get(`/api/v1/feedback/me?page=${nextPage}&page_size=20`)) as {
        items?: FeedbackTicket[];
        total?: number;
        page_size?: number;
      };
      const incoming = res.items || [];
      setTickets((current) => append ? [...current, ...incoming.filter((item) => !current.some((existing) => existing.id === item.id))] : incoming);
      setTotal(res.total ?? incoming.length);
      setPage(nextPage);
      setHasMore((res.total ?? incoming.length) > nextPage * (res.page_size ?? 20));
    } catch (e) {
      silentError(e, { component: "FeedbackMinePage", action: "load" });
      setError(t(getUserFacingErrorKey(e, "common.loadFailed")));
    } finally {
      setLoadingMore(false);
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { void load(); }, [load]);

  if (!user) {
    return (
      <div className="mx-auto w-full max-w-2xl px-4 py-8 text-center">
        <Inbox className="mx-auto h-12 w-12 text-muted-foreground" />
        <p className="mt-4 text-sm text-muted-foreground">{t("feedback.loginRequired")}</p>
        <Link href="/login" className="mt-2 inline-block text-sm text-primary hover:underline">
          {t("auth.login")}
        </Link>
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-2xl px-4 py-8">
      <div className="mb-6 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Inbox className="h-8 w-8 text-primary" />
          <div>
            <h1 className="text-2xl font-bold tracking-tight">{t("feedback.myTickets")}</h1>
            <p className="text-sm text-muted-foreground">
              {t("feedback.ticketCount", { count: total })}
            </p>
          </div>
        </div>
        <Link
          href="/feedback"
          className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
        >
          <MessageSquare className="h-3.5 w-3.5" />
          {t("feedback.newTicket")}
        </Link>
      </div>

      <DataList
        items={tickets}
        loading={loading}
        error={error}
        onRetry={() => void load(page, page > 1)}
        hasMore={hasMore}
        loadingMore={loadingMore}
        onLoadMore={() => load(page + 1, true)}
        empty={<EmptyState icon={Inbox} title={t("feedback.noTickets")} description={t("feedback.submitFirst")} />}
        loadingState={<div className="space-y-3"><Skeleton className="h-20 w-full rounded-lg" /><Skeleton className="h-20 w-full rounded-lg" /><Skeleton className="h-20 w-full rounded-lg" /></div>}
        getKey={(ticket) => ticket.id}
        renderItem={(ticket) => (
            <Link
              key={ticket.id}
              href={`/feedback/${ticket.id}`}
              className="flex items-center justify-between rounded-lg border border-border bg-card p-3 hover:bg-canvas-subtle transition-colors"
            >
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span
                    className={`inline-block rounded px-1.5 py-0.5 text-[10px] font-medium ${
                      ticket.status === "open"
                        ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                        : ticket.status === "in_progress"
                          ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
                          : ticket.status === "resolved"
                            ? "bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400"
                            : "bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-500"
                    }`}
                  >
                    {t(`feedback.status_${ticket.status}`)}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {t(`feedback.cat_${ticket.category}`)}
                  </span>
                </div>
                <p className="mt-1 truncate text-sm font-medium">{ticket.title}</p>
                <p className="text-[11px] text-muted-foreground">
                  {new Date(ticket.created_at).toLocaleDateString()}
                </p>
              </div>
              <ChevronRight className="h-4 w-4 text-muted-foreground" />
            </Link>
        )}
      />
    </div>
  );
}
