"use client";

import { useState, useEffect, useCallback } from "react";
import { useTranslations } from "next-intl";
import { ThumbsUp, ThumbsDown, Flag, Heart } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { useAuth, interactionDenialKey } from "@/contexts/AuthContext";
import { useAuthGate } from "@/components/auth/AuthGateProvider";
import { api, ApiRequestError } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { cn } from "@/lib/utils";

interface ReactionBarProps {
  contentId: number;
  initialLikes?: number;
  initialDislikes?: number;
  className?: string;
}

interface ReactionSnapshot {
  counts?: { like?: number; dislike?: number };
  viewer_reaction?: "like" | "dislike" | null;
}

export function ReactionBar({
  contentId,
  initialLikes = 0,
  initialDislikes = 0,
  className,
}: ReactionBarProps) {
  const t = useTranslations();
  const { toast } = useToast();
  const { user, capabilities } = useAuth();
  const { requireAuth } = useAuthGate();
  const [myReaction, setMyReaction] = useState<"like" | "dislike" | null>(null);
  const [likeCount, setLikeCount] = useState(initialLikes);
  const [dislikeCount, setDislikeCount] = useState(initialDislikes);
  const [busy, setBusy] = useState(false);
  const [reported, setReported] = useState(false);
  const [reportOpen, setReportOpen] = useState(false);
  // SP-17/T2 (#491)：未登录可点击（开登录浮窗自动续做）；能力拒绝仍禁用+原因。
  // 注意匿名视角 capabilities 恒 fail-closed（can_interact=false），必须带上
  // !!user 才能把「未登录」从「能力拒绝」中分离出来（FollowButton 同口径）。
  const interactionBlocked = !!user && !capabilities.can_interact;
  const disabled = interactionBlocked || busy;
  const denialKey = interactionDenialKey(capabilities.interaction_denial_reason);

  const applySnapshot = useCallback((data: ReactionSnapshot) => {
    if (data.counts) {
      setLikeCount(data.counts.like ?? 0);
      setDislikeCount(data.counts.dislike ?? 0);
    }
    if (data.viewer_reaction !== undefined) {
      setMyReaction(data.viewer_reaction ?? null);
    }
  }, []);

  useEffect(() => {
    if (!user) return;
    void fetchMyReaction();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user, contentId]);

  async function fetchMyReaction() {
    try {
      const data = await api.get<ReactionSnapshot>(`/api/v1/social/reactions?target_type=content&target_id=${contentId}`);
      applySnapshot(data);
    } catch (e) { silentError(e, { component: 'ReactionBar', action: 'fetchMyReaction' }); }
  }

  // SP-17/T2：门与动作分离——performReaction 不自查登录态，登录成功后由
  // 门层重放（闭包内的 user/busy 快照均不得作为执行前提）。
  const performReaction = useCallback(
    async (reaction: "like" | "dislike") => {
      setBusy(true);
      const prevReaction = myReaction;
      const prevLikes = likeCount;
      const prevDislikes = dislikeCount;

      // Optimistic update
      if (myReaction === reaction) {
        setMyReaction(null);
        if (reaction === "like") setLikeCount((c) => c - 1);
        else setDislikeCount((c) => c - 1);
      } else {
        if (myReaction === "like") setLikeCount((c) => c - 1);
        else if (myReaction === "dislike") setDislikeCount((c) => c - 1);
        setMyReaction(reaction);
        if (reaction === "like") setLikeCount((c) => c + 1);
        else setDislikeCount((c) => c + 1);
      }

      try {
        const data = await api.post<ReactionSnapshot>("/api/v1/social/reactions", {
          target_type: "content",
          target_id: contentId,
          reaction,
        });
        applySnapshot(data);
      } catch (e) {
        silentError(e, { component: 'ReactionBar', action: 'react' });
        setMyReaction(prevReaction);
        setLikeCount(prevLikes);
        setDislikeCount(prevDislikes);
      } finally {
        setBusy(false);
      }
    },
    [myReaction, likeCount, dislikeCount, contentId, applySnapshot],
  );

  const react = useCallback(
    (reaction: "like" | "dislike") => {
      if (interactionBlocked || busy) return;
      if (!user) {
        requireAuth(() => void performReaction(reaction));
        return;
      }
      void performReaction(reaction);
    },
    [user, interactionBlocked, busy, requireAuth, performReaction],
  );

  async function submitReport(reason: string) {
    if (!user) return;
    try {
      await api.post(`/api/v1/contents/${contentId}/report`, { reason });
      setReported(true);
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 409) {
        setReported(true);
        return;
      }
      toast("error", t(getUserFacingErrorKey(e, "social.reportFailed")));
      silentError(e, { component: 'ReactionBar', action: 'report' });
      throw e;
    }
  }

  return (
    <div
      className={cn(
        "flex items-center gap-2 rounded-md border border-border bg-card px-4 py-3 ",
        className,
      )}
    >
      <Button
        variant={myReaction === "like" ? "default" : "outline"}
        size="sm"
        disabled={disabled}
        aria-pressed={myReaction === "like"}
        onClick={() => react("like")}
        title={!user && !interactionBlocked ? t('auth.loginToInteract') : disabled ? t(denialKey) : t('social.like')}
      >
        <ThumbsUp className="mr-1 h-3.5 w-3.5" />
        {likeCount}
      </Button>

      <Button
        variant={myReaction === "dislike" ? "default" : "outline"}
        size="sm"
        disabled={disabled}
        aria-pressed={myReaction === "dislike"}
        onClick={() => react("dislike")}
        title={!user && !interactionBlocked ? t('auth.loginToInteract') : disabled ? t(denialKey) : t('social.dislike')}
      >
        <ThumbsDown className="mr-1 h-3.5 w-3.5" />
        {dislikeCount}
      </Button>

      <div className="flex-1" />

      <Button
        variant="ghost"
        size="sm"
        disabled={!user || interactionBlocked || reported}
        onClick={() => setReportOpen(true)}
        title={reported ? t('social.reported') : !user ? t('auth.loginToInteract') : disabled ? t(denialKey) : t('social.report')}
      >
        <Flag className="mr-1 h-3.5 w-3.5" />
        {reported ? t('social.reported') : t('social.report')}
      </Button>

      <ConfirmModal
        open={reportOpen}
        onOpenChange={setReportOpen}
        title={t('social.reportDialogTitle')}
        description={t('social.reportReason')}
        reasonLabel={t('social.reportReason')}
        confirmLabel={t('social.report')}
        requireReason
        onConfirm={submitReport}
      />
    </div>
  );
}
