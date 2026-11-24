"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import { Scale, ThumbsDown, ThumbsUp, ChevronDown, ChevronUp } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { DataList } from "@/components/ui/data-list";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import VerdictDetail from "@/components/judge/VerdictDetail";

/* #1022：我的投票历史行契约（GET /api/v1/judge/votes/mine）。不做内容
 * 预览——审核中内容仅持资格判官可加载、banned 内容不透出，本页一律不嵌。 */
interface MyVoteRow {
  case_id: number;
  target_type: string;
  my_vote: string;
  reason: string | null;
  created_at: string;
  case_status: string;
  vote_approve: number;
  vote_reject: number;
  min_votes: number;
  closed_at: string | null;
  upvotes: number;
  downvotes: number;
}

const PAGE_SIZE = 20;

export default function JudgeHistoryPage() {
  const t = useTranslations();
  const locale = useLocale();
  const { user } = useAuth();

  const [rows, setRows] = useState<MyVoteRow[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [expandedCaseId, setExpandedCaseId] = useState<number | null>(null);

  const load = useCallback(async (nextPage = 1, append = false) => {
    if (append) setLoadingMore(true); else setLoading(true);
    setError("");
    try {
      const res = await api.get<{ items?: MyVoteRow[]; total?: number; page_size?: number }>(
        `/api/v1/judge/votes/mine?page=${nextPage}&page_size=${PAGE_SIZE}`
      );
      const incoming = res.items || [];
      setRows((current) =>
        append
          ? [...current, ...incoming.filter((item) => !current.some((existing) => existing.case_id === item.case_id))]
          : incoming
      );
      setTotal(res.total ?? incoming.length);
      setPage(nextPage);
      setHasMore((res.total ?? incoming.length) > nextPage * (res.page_size ?? PAGE_SIZE));
    } catch (e) {
      silentError(e, { component: "JudgeHistoryPage", action: "load" });
      setError(t(getUserFacingErrorKey(e, "judge.history.loadFailed")));
    } finally {
      setLoadingMore(false);
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    if (user) {
      void load();
    }
  }, [user, load]);

  const typeLabelKey: Record<string, string> = {
    article: "judge.article",
    image: "judge.image",
    video: "judge.video",
    audio: "judge.audio",
    prompt: "judge.prompt",
    sheet_music: "judge.sheetMusic",
    template: "judge.template",
    other: "judge.other",
    content: "judge.reviewCard.typeContent",
    comment: "judge.reviewCard.typeComment",
  };

  function renderRow(row: MyVoteRow) {
    const typeLabel = typeLabelKey[row.target_type]
      ? t(typeLabelKey[row.target_type])
      : row.target_type;
    const totalVotes = row.vote_approve + row.vote_reject;
    // 终态判定与 VerdictDetail 同口径：closed_ 前缀 + 60% 不违规阈值。
    const isClosed = row.case_status.startsWith("closed_");
    const isNotViolation = isClosed && row.vote_approve / Math.max(totalVotes, 1) >= 0.6;
    const verdictLabel = isNotViolation
      ? t("judge.reviewCard.approve")
      : t("judge.reviewCard.reject");
    const matchesVerdict = row.my_vote === (isNotViolation ? "approve" : "reject");
    const expanded = expandedCaseId === row.case_id;
    const hasReason = Boolean(row.reason && row.reason.trim().length > 0);

    return (
      <div className="rounded-lg border border-border bg-card p-4">
        <div className="flex flex-wrap items-center gap-2">
          <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
            {typeLabel}
          </span>
          <span className="text-xs text-muted-foreground">{t("judge.history.myVote")}</span>
          <span
            className={`rounded px-1.5 py-0.5 text-[11px] font-medium ${
              row.my_vote === "approve"
                ? "bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                : "bg-destructive/10 text-destructive"
            }`}
          >
            {row.my_vote === "approve" ? t("judge.reviewCard.approve") : t("judge.reviewCard.reject")}
          </span>
          <span className="ml-auto text-[11px] text-muted-foreground">
            {new Date(row.created_at).toLocaleString(locale === "en" ? "en-US" : "zh-CN")}
          </span>
        </div>

        {hasReason && (
          <div className="mt-2 flex items-start justify-between gap-3">
            <p className="min-w-0 flex-1 text-sm text-foreground">
              <span className="mr-1 text-xs text-muted-foreground">{t("judge.history.reasonLabel")}</span>
              {row.reason}
            </p>
            <div className="flex shrink-0 items-center gap-3 text-xs text-muted-foreground">
              <span className="inline-flex items-center gap-1" aria-label={t("judge.history.reasonUpvotes", { count: row.upvotes })}>
                <ThumbsUp className="size-3.5" aria-hidden="true" />
                <span>{row.upvotes}</span>
              </span>
              <span className="inline-flex items-center gap-1" aria-label={t("judge.history.reasonDownvotes", { count: row.downvotes })}>
                <ThumbsDown className="size-3.5" aria-hidden="true" />
                <span>{row.downvotes}</span>
              </span>
            </div>
          </div>
        )}

        <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
          {isClosed ? (
            <span className="flex flex-wrap items-center gap-2 text-xs">
              <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                {t("judge.history.closed")}
              </span>
              <span className="font-medium text-foreground">
                {t("judge.history.verdictResult", { result: verdictLabel })}
              </span>
              <span className={matchesVerdict ? "text-emerald-600" : "text-destructive"}>
                {matchesVerdict ? t("judge.history.matchVerdict") : t("judge.history.oppositeVerdict")}
              </span>
            </span>
          ) : (
            <span className="rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
              {t("judge.history.inProgress", { total: totalVotes, min: row.min_votes })}
            </span>
          )}
          <Button
            type="button"
            variant="ghost"
            size="sm"
            aria-expanded={expanded}
            onClick={() => setExpandedCaseId(expanded ? null : row.case_id)}
          >
            {expanded ? (
              <ChevronUp className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
            ) : (
              <ChevronDown className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
            )}
            {expanded ? t("judge.history.collapseDetail") : t("judge.history.expandDetail")}
          </Button>
        </div>

        {expanded && (
          <div className="mt-3">
            <VerdictDetail caseId={row.case_id} />
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-2xl px-4 py-8">
      <div className="mb-6 flex items-center gap-3">
        <Scale className="h-8 w-8 text-primary" aria-hidden="true" />
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("judge.history.title")}</h1>
          <p className="text-sm text-muted-foreground">
            {t("judge.history.count", { count: total })}
          </p>
        </div>
      </div>

      <DataList
        items={rows}
        loading={loading}
        error={error}
        onRetry={() => void load(page, page > 1)}
        hasMore={hasMore}
        loadingMore={loadingMore}
        onLoadMore={() => load(page + 1, true)}
        empty={
          <EmptyState
            icon={Scale}
            title={t("judge.history.empty")}
            description={t("judge.history.emptyHint")}
            action={
              <Link
                href="/judge/queue"
                className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
              >
                {t("judge.history.goQueue")}
              </Link>
            }
          />
        }
        loadingState={
          <div className="space-y-3">
            <Skeleton className="h-24 w-full rounded-lg" />
            <Skeleton className="h-24 w-full rounded-lg" />
            <Skeleton className="h-24 w-full rounded-lg" />
          </div>
        }
        getKey={(row) => row.case_id}
        renderItem={(row) => renderRow(row)}
      />
    </div>
  );
}
