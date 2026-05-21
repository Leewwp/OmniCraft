"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
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
  const { user } = useAuth();
  const router = useRouter();

  const [cases, setCases] = useState<JudgeCaseData[]>([]);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [votedCaseId, setVotedCaseId] = useState<number | null>(null);
  const [queueTotal, setQueueTotal] = useState(0);

  const loadQueue = useCallback(async () => {
    if (!user) return;
    setLoading(true);
    setError("");
    try {
      const data = await api.get<{ cases: JudgeCaseData[]; total: number }>(
        "/api/v1/judge/queue?page=1&page_size=20"
      );
      setCases(data.cases || []);
      setQueueTotal(data.total || 0);
      setCurrentIndex(0);
      setVotedCaseId(null);
    } catch (e) {
      silentError(e, { component: 'JudgeQueuePage', action: 'loadQueue' });
      setError(e instanceof ApiRequestError ? e.message : t('judge.loadQueueFailed'));
    } finally {
      setLoading(false);
    }
  }, [user, t]);

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
      setError(e instanceof ApiRequestError ? e.message : t('judge.voteFailed'));
    } finally {
      setSubmitting(false);
    }
  }

  function goNextCase() {
    setVotedCaseId(null);
    setCurrentIndex((i) => i + 1);
  }

  const isReputationBlocked = user!.reputation < 3;

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

      {isReputationBlocked && (
        <div className="rounded-md border border-destructive/50 bg-destructive/5 p-4 ">
          <p className="text-sm text-destructive">
            {t('judge.lowReputation', { reputation: user!.reputation })}
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
      ) : (
        <div className="space-y-4">
          <ReviewCard
            judgeCase={cases[currentIndex]}
            disabled={isReputationBlocked}
            submitting={submitting}
            onVote={(caseId, vote, reason) => void handleVote(caseId, vote, reason)}
          />

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
