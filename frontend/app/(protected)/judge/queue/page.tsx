"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth, interactionDenialKey } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { useRouter } from "next/navigation";
import ReviewCard from "@/components/judge/ReviewCard";
import VerdictDetail from "@/components/judge/VerdictDetail";

interface JudgeCaseData {
  id: number;
  target_type: string;
  target_id: number;
  status: string;
  vote_approve: number;
  vote_reject: number;
  min_votes: number;
  created_at: string;
}

export default function JudgeQueuePage() {
  const t = useTranslations();
  const { user, capabilities } = useAuth();
  const router = useRouter();

  const [cases, setCases] = useState<JudgeCaseData[]>([]);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [votedCaseId, setVotedCaseId] = useState<number | null>(null);
  const [queueTotal, setQueueTotal] = useState(0);
  const [page, setPage] = useState(1);

  const PAGE_SIZE = 20;

  const loadQueue = useCallback(async () => {
    if (!user) return;
    setLoading(true);
    setError("");
    try {
      const data = await api.get<{ cases: JudgeCaseData[]; total: number }>(
        `/api/v1/judge/queue?page=1&page_size=${PAGE_SIZE}`
      );
      setCases(data.cases || []);
      setQueueTotal(data.total || 0);
      setPage(1);
      setCurrentIndex(0);
      setVotedCaseId(null);
    } catch (e) {
      silentError(e, { component: 'JudgeQueuePage', action: 'loadQueue' });
      setError(t(getUserFacingErrorKey(e, "judge.loadQueueFailed")));
    } finally {
      setLoading(false);
    }
  }, [user, t]);

  const hasMore = cases.length < queueTotal;

  const loadMore = useCallback(async () => {
    if (!user || loadingMore || !hasMore) return;
    setLoadingMore(true);
    setError("");
    try {
      const next = page + 1;
      const data = await api.get<{ cases: JudgeCaseData[]; total: number }>(
        `/api/v1/judge/queue?page=${next}&page_size=${PAGE_SIZE}`
      );
      setCases((prev) => [...prev, ...(data.cases || [])]);
      setQueueTotal(data.total || 0);
      setPage(next);
    } catch (e) {
      silentError(e, { component: 'JudgeQueuePage', action: 'loadMore' });
      setError(t(getUserFacingErrorKey(e, "judge.loadQueueFailed")));
    } finally {
      setLoadingMore(false);
    }
  }, [user, loadingMore, hasMore, page, t]);

  useEffect(() => {
    if (user) {
      void loadQueue();
    }
  }, [user, loadQueue]);

  async function handleVote(caseId: number, vote: string, reason: string) {
    setSubmitting(true);
    setError("");
    try {
      await api.post("/api/v1/judge/vote", {
        case_id: caseId,
        vote,
        reason: reason || undefined,
      });
      setVotedCaseId(caseId);
    } catch (e) {
      silentError(e, { component: 'JudgeQueuePage', action: 'handleVote' });
      setError(t(getUserFacingErrorKey(e, "judge.voteFailed")));
    } finally {
      setSubmitting(false);
    }
  }

  function goNextCase() {
    setVotedCaseId(null);
    setCurrentIndex((i) => i + 1);
  }

  const isInteractionBlocked = !capabilities.can_interact;
  const denialKey = interactionDenialKey(capabilities.interaction_denial_reason);

  return (
    <div className="mx-auto w-full max-w-2xl space-y-4 px-4 py-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 ">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('judge.queueTitle')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t('judge.queueSubtitle')}
          </p>
        </div>
        <Button size="sm" variant="outline" onClick={() => router.push("/judge/exam")}>
          {t('judge.qualificationExam')}
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {user && isInteractionBlocked && (
        <div className="rounded-md border border-destructive/50 bg-destructive/5 p-4 ">
          <p className="text-sm text-destructive">
            {t(denialKey)}
          </p>
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center rounded-md border border-border bg-card p-12 ">
          <span className="text-sm text-muted-foreground">{t('judge.loadingQueue')}</span>
        </div>
      ) : cases.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center ">
          <p className="text-sm text-muted-foreground">
            {t('judge.noPendingContent')}
          </p>
          <Button size="sm" className="mt-4" variant="outline" onClick={() => router.push("/judge/exam")}>
            {t('judge.takeExam')}
          </Button>
        </div>
      ) : currentIndex >= cases.length ? (
        hasMore ? (
          <div className="rounded-md border border-border bg-card p-12 text-center ">
            <p className="text-sm text-muted-foreground">
              {t('judge.queueSummary', { total: queueTotal })}
            </p>
            <Button
              size="sm"
              className="mt-4"
              variant="outline"
              disabled={loadingMore}
              onClick={() => void loadMore()}
            >
              {loadingMore ? t('judge.loadingQueue') : t('judge.loadMore')}
            </Button>
          </div>
        ) : (
          <div className="rounded-md border border-border bg-card p-12 text-center ">
            <p className="text-sm text-muted-foreground">
              {t('judge.queueCompleted')}
            </p>
            <p className="mt-1 text-xs text-muted-foreground">
              {t('judge.queueSummary', { total: queueTotal })}
            </p>
            <Button size="sm" className="mt-4" variant="outline" onClick={() => void loadQueue()}>
              {t('judge.refreshQueue')}
            </Button>
          </div>
        )
      ) : (
        <div className="space-y-4">
          <ReviewCard
            judgeCase={cases[currentIndex]}
            disabled={isInteractionBlocked}
            submitting={submitting}
            onVote={(caseId, vote, reason) => void handleVote(caseId, vote, reason)}
          />

          {hasMore && currentIndex >= cases.length - 3 && (
            <div className="text-center">
              <Button
                size="sm"
                variant="ghost"
                disabled={loadingMore}
                onClick={() => void loadMore()}
              >
                {loadingMore ? t('judge.loadingQueue') : t('judge.loadMore')}
              </Button>
            </div>
          )}

          {votedCaseId && (
            <>
              <div className="rounded-md border border-emerald-500/30 bg-emerald-50 p-3 text-center ">
                <p className="text-sm font-medium text-emerald-700">
                  {t('judge.voteSuccess')}
                </p>
                <Button size="sm" variant="outline" className="mt-2" onClick={goNextCase}>
                  {t('judge.nextCase')}
                </Button>
              </div>
              <VerdictDetail caseId={votedCaseId} />
            </>
          )}
        </div>
      )}
    </div>
  );
}
