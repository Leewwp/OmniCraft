"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";

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

interface ReviewCardProps {
  judgeCase: JudgeCaseData;
  disabled: boolean;
  submitting: boolean;
  onVote: (caseId: number, vote: "approve" | "reject", reason: string) => void;
}

export default function ReviewCard({ judgeCase, disabled, submitting, onVote }: ReviewCardProps) {
  const [showConfirm, setShowConfirm] = useState(false);
  const [pendingVote, setPendingVote] = useState<"approve" | "reject" | null>(null);
  const [reason, setReason] = useState("");

  const totalVotes = judgeCase.vote_approve + judgeCase.vote_reject;
  const approvePct = totalVotes > 0 ? Math.round((judgeCase.vote_approve / totalVotes) * 100) : 0;
  const rejectPct = totalVotes > 0 ? Math.round((judgeCase.vote_reject / totalVotes) * 100) : 0;

  function triggerVote(vote: "approve" | "reject") {
    setPendingVote(vote);
    setShowConfirm(true);
  }

  function confirmVote() {
    if (pendingVote) {
      onVote(judgeCase.id, pendingVote, reason);
      setShowConfirm(false);
      setPendingVote(null);
      setReason("");
    }
  }

  function cancelVote() {
    setShowConfirm(false);
    setPendingVote(null);
  }

  const targetTypeLabel: Record<string, string> = {
    content: "内容",
    comment: "评论",
  };

  return (
    <div className="space-y-4 rounded-md border border-border bg-card p-4 shadow-none">
      <div className="flex items-center justify-between">
        <span className="text-xs text-muted-foreground">
          {targetTypeLabel[judgeCase.target_type] || judgeCase.target_type} #{judgeCase.target_id}
        </span>
        <span className="text-xs text-muted-foreground">
          {new Date(judgeCase.created_at).toLocaleString("zh-CN")}
        </span>
      </div>

      <div className="rounded-md border border-border bg-muted/30 p-3">
        <p className="text-sm text-muted-foreground">
          该内容因被举报进入众裁审核，请根据社区规范判断是否违规。
        </p>
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>当前投票进度</span>
          <span>
            {totalVotes} / {judgeCase.min_votes} 票
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
          <span className="text-emerald-600">不违规 {judgeCase.vote_approve}</span>
          <span className="text-destructive">违规 {judgeCase.vote_reject}</span>
        </div>
      </div>

      {!showConfirm && (
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            className="flex-1 border-emerald-500/30 text-emerald-600 hover:bg-emerald-50"
            disabled={disabled || submitting}
            onClick={() => triggerVote("approve")}
          >
            不违规
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="flex-1 border-destructive/30 text-destructive hover:bg-destructive/10"
            disabled={disabled || submitting}
            onClick={() => triggerVote("reject")}
          >
            违规
          </Button>
        </div>
      )}

      {showConfirm && (
        <div className="space-y-3 rounded-md border border-border bg-muted/20 p-3">
          <p className="text-sm font-medium">
            确认投「{pendingVote === "approve" ? "不违规" : "违规"}」
          </p>
          <textarea
            placeholder="可选：填写判定理由"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={2}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
          />
          <div className="flex gap-2">
            <Button size="sm" disabled={submitting} onClick={confirmVote}>
              {submitting ? "提交中..." : "确认"}
            </Button>
            <Button size="sm" variant="outline" onClick={cancelVote}>
              取消
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
