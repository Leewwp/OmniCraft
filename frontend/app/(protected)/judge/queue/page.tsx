"use client";

import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
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
  const { user, isLoading: authLoading } = useAuth();
  const router = useRouter();

  const [cases, setCases] = useState<JudgeCaseData[]>([]);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [votedCaseId, setVotedCaseId] = useState<number | null>(null);
  const [queueTotal, setQueueTotal] = useState(0);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login");
    }
  }, [user, authLoading, router]);

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
      setError(e instanceof ApiRequestError ? e.message : "加载队列失败");
    } finally {
      setLoading(false);
    }
  }, [user]);

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
      setError(e instanceof ApiRequestError ? e.message : "投票失败");
    } finally {
      setSubmitting(false);
    }
  }

  function goNextCase() {
    setVotedCaseId(null);
    setCurrentIndex((i) => i + 1);
  }

  if (authLoading) {
    return (
      <div className="mx-auto w-full max-w-2xl px-4 py-6 text-sm text-muted-foreground">
        加载中...
      </div>
    );
  }

  if (!user) return null;

  const isReputationBlocked = user.reputation < 3;

  return (
    <div className="mx-auto w-full max-w-2xl space-y-4 px-4 py-6">
      <div className="flex items-center justify-between rounded-md border border-border bg-card p-4 shadow-none">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">待审内容队列</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            参与众裁，维护社区内容质量
          </p>
        </div>
        <Button size="sm" variant="outline" onClick={() => router.push("/judge/exam")}>
          资质考核
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {isReputationBlocked && (
        <div className="rounded-md border border-destructive/50 bg-destructive/5 p-4 shadow-none">
          <p className="text-sm text-destructive">
            信誉分不足（当前 {user.reputation} 分，需要 ≥ 3 分），无法参与众裁投票。
          </p>
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center rounded-md border border-border bg-card p-12 shadow-none">
          <span className="text-sm text-muted-foreground">加载队列中...</span>
        </div>
      ) : cases.length === 0 ? (
        <div className="rounded-md border border-border bg-card p-12 text-center shadow-none">
          <p className="text-sm text-muted-foreground">
            暂无待审内容。请先通过资质考核获取判官资格，或等待新的举报内容进入队列。
          </p>
          <Button size="sm" className="mt-4" variant="outline" onClick={() => router.push("/judge/exam")}>
            参加考核
          </Button>
        </div>
      ) : currentIndex >= cases.length ? (
        <div className="rounded-md border border-border bg-card p-12 text-center shadow-none">
          <p className="text-sm text-muted-foreground">
            当前队列已全部审核完毕
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            共 {queueTotal} 个待审案例，你已完成当前页面的投票
          </p>
          <Button size="sm" className="mt-4" variant="outline" onClick={() => void loadQueue()}>
            刷新队列
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
              <div className="rounded-md border border-emerald-500/30 bg-emerald-50 p-3 text-center shadow-none">
                <p className="text-sm font-medium text-emerald-700">
                  投票成功
                </p>
                <Button size="sm" variant="outline" className="mt-2" onClick={goNextCase}>
                  下一个案例
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
