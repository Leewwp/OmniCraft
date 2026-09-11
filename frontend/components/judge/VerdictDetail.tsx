"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import { api, ApiRequestError } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { AlertCircle, Scale, ThumbsDown, ThumbsUp } from "lucide-react";

interface CaseData {
  id: number;
  target_type: string;
  target_id: number;
  status: string;
  vote_approve: number;
  vote_reject: number;
  min_votes: number;
  created_at: string;
  closed_at?: string;
}

interface VoteWithMeta {
  id: number;
  judge_id: number;
  judge_name: string;
  vote: string;
  reason: string;
  created_at: string;
  upvotes: number;
  downvotes: number;
  user_vote_type?: string;
}

interface VerdictDetailProps {
  caseId: number;
}

export default function VerdictDetail({ caseId }: VerdictDetailProps) {
  const t = useTranslations();
  const locale = useLocale();
  const [caseData, setCaseData] = useState<CaseData | null>(null);
  const [votes, setVotes] = useState<VoteWithMeta[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  // T38：理由投票的轻提示（自赞/无资格等），不阻塞 verdict 主体渲染。
  const [voteNotice, setVoteNotice] = useState("");

  const loadVerdict = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api.get<{ case: CaseData; votes: VoteWithMeta[] }>(
        `/api/v1/judge/cases/${caseId}/verdict`
      );
      setCaseData(data.case);
      setVotes(data.votes || []);
    } catch (e) {
      setError(t(getUserFacingErrorKey(e, "judge.verdict.loadFailed")));
      silentError(e, { component: 'VerdictDetail', action: 'loadVerdict' });
    } finally {
      setLoading(false);
    }
  }, [caseId, t]);

  useEffect(() => {
    void loadVerdict();
  }, [loadVerdict]);

  async function voteReason(voteId: number, voteType: "up" | "down") {
    setVoteNotice("");
    try {
      await api.post(`/api/v1/judge/reasons/${voteId}/vote`, { vote_type: voteType });
      setVotes((prev) =>
        prev.map((v) => {
          if (v.id !== voteId) return v;
          const oldType = v.user_vote_type;
          let up = v.upvotes;
          let down = v.downvotes;
          if (oldType === "up") up--;
          if (oldType === "down") down--;
          if (voteType === "up") up++;
          if (voteType === "down") down++;
          return { ...v, upvotes: Math.max(0, up), downvotes: Math.max(0, down), user_vote_type: voteType };
        })
      );
    } catch (e) {
      // T38：守卫类失败给明确提示，其余沿用静默降级。
      if (e instanceof ApiRequestError && (e.code === "REASON_SELF_VOTE" || e.code === "JUDGE_QUALIFICATION_REQUIRED")) {
        setVoteNotice(t(getUserFacingErrorKey(e)));
      } else if (!(e instanceof ApiRequestError && e.status === 409)) {
        setError(t(getUserFacingErrorKey(e)));
        silentError(e, { component: 'VerdictDetail', action: 'voteReason' });
      }
    }
  }

  if (loading) {
    return (
      <div className="min-h-56 space-y-4 rounded-md border border-border bg-card p-4" aria-busy="true" aria-live="polite">
        <span className="sr-only" role="status">{t('common.loading')}</span>
        <Skeleton className="h-5 w-36" />
        <Skeleton className="h-3 w-full" />
        <Skeleton className="h-2 w-full" />
        <Skeleton className="h-20 w-full" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-md border border-destructive/50 bg-destructive/5 p-4" role="alert" aria-live="polite">
        <AlertCircle className="size-5 text-destructive" aria-hidden="true" />
        <p className="text-sm text-destructive">{error}</p>
        <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => void loadVerdict()}>{t('common.retry')}</Button>
      </div>
    );
  }

  if (!caseData) {
    return (
      <EmptyState
        icon={Scale}
        title={t('common.noData')}
        description={t('judge.verdict.title')}
      />
    );
  }

  const totalVotes = caseData.vote_approve + caseData.vote_reject;
  const approvePct = totalVotes > 0 ? Math.round((caseData.vote_approve / totalVotes) * 100) : 0;
  const rejectPct = totalVotes > 0 ? Math.round((caseData.vote_reject / totalVotes) * 100) : 0;
  // T39（FIX-03）：结案终态是 closed_approve/closed_reject 前缀——
  // 旧词表（=== "closed"）恒假导致结案横幅永不显示。
  const isClosed = caseData.status.startsWith("closed_");
  const isNotViolation = isClosed && caseData.vote_approve / Math.max(totalVotes, 1) >= 0.6;
  const verdictLabel = isNotViolation ? t('judge.reviewCard.approve') : t('judge.reviewCard.reject');

  const reasonsWithText = votes.filter((v) => v.reason && v.reason.trim().length > 0);
  const sortedReasons = [...reasonsWithText].sort((a, b) => b.upvotes - b.downvotes - (a.upvotes - a.downvotes));

  return (
    <div className="space-y-4 rounded-md border border-border bg-card p-4 ">
      <h3 className="text-sm font-semibold">{t('judge.verdict.title')}</h3>

      <div className="space-y-2">
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>{t('judge.verdict.voteDistribution')}</span>
          <span>
            {t('judge.verdict.votes', {
              totalVotes,
              minVotes: caseData.min_votes,
              closed: isClosed ? t('judge.verdict.closed') : "",
            })}
          </span>
        </div>
        <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
          <div className="flex h-full">
            <div
              className="h-full bg-emerald-500 transition-all duration-300"
              style={{ width: `${approvePct}%` }}
            />
            <div
              className="h-full bg-destructive transition-all duration-300"
              style={{ width: `${rejectPct}%` }}
            />
          </div>
        </div>
        <div className="flex justify-between text-xs">
          <span className="text-emerald-600">{t('judge.verdict.approveCount', { count: caseData.vote_approve })}</span>
          <span className="text-destructive">{t('judge.verdict.rejectCount', { count: caseData.vote_reject })}</span>
        </div>
      </div>

      {isClosed && (
        <div className={`rounded-md px-3 py-2 text-sm font-medium ${
          isNotViolation
            ? "bg-emerald-50 text-emerald-700"
            : "bg-destructive/10 text-destructive"
        }`}>
          {t('judge.verdict.result', { result: verdictLabel })}
        </div>
      )}

      {sortedReasons.length > 0 ? (
        <div className="space-y-3">
          <p className="text-xs font-medium text-muted-foreground">
            {t('judge.verdict.judgeReasons')}
          </p>
          {voteNotice && (
            <p className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive" role="status">
              {voteNotice}
            </p>
          )}
          {sortedReasons.map((v) => (
            <div
              key={v.id}
              className="rounded-md border border-border bg-muted/20 p-3 "
            >
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <span className="font-medium">{v.judge_name || t('common.userLabel', { id: v.judge_id })}</span>
                <span
                  className={`rounded px-1.5 py-0.5 text-[11px] ${
                    v.vote === "approve"
                      ? "bg-emerald-50 text-emerald-700"
                      : "bg-destructive/10 text-destructive"
                  }`}
                >
                  {v.vote === "approve" ? t('judge.reviewCard.approve') : t('judge.reviewCard.reject')}
                </span>
                <span className="ml-auto">{new Date(v.created_at).toLocaleString(locale === "en" ? "en-US" : "zh-CN")}</span>
              </div>
              <p className="mt-1 text-sm text-foreground">{v.reason}</p>
              <div className="mt-2 flex items-center gap-3">
                <button
                  type="button"
                  onClick={() => void voteReason(v.id, "up")}
                  aria-label={t('social.like')}
                  className={`inline-flex items-center gap-1 text-xs transition-colors cursor-pointer ${
                    v.user_vote_type === "up"
                      ? "text-emerald-600"
                      : "text-muted-foreground hover:text-emerald-600"
                  }`}
                >
                  <ThumbsUp className="size-3.5" aria-hidden="true" />
                  <span>{v.upvotes}</span>
                </button>
                <button
                  type="button"
                  onClick={() => void voteReason(v.id, "down")}
                  aria-label={t('social.dislike')}
                  className={`inline-flex items-center gap-1 text-xs transition-colors cursor-pointer ${
                    v.user_vote_type === "down"
                      ? "text-destructive"
                      : "text-muted-foreground hover:text-destructive"
                  }`}
                >
                  <ThumbsDown className="size-3.5" aria-hidden="true" />
                  <span>{v.downvotes}</span>
                </button>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <EmptyState icon={Scale} title={t('judge.verdict.noReasons')} />
      )}
    </div>
  );
}
