"use client";

import { useState, useEffect, useCallback } from "react";
import { useTranslations } from "next-intl";
import { ThumbsUp, ThumbsDown, Flag, Heart } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { useAuth, interactionDenialKey } from "@/contexts/AuthContext";
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

export function ReactionBar({
  contentId,
  initialLikes = 0,
  initialDislikes = 0,
  className,
}: ReactionBarProps) {
  const t = useTranslations();
  const { toast } = useToast();
  const { user, capabilities } = useAuth();
  const [myReaction, setMyReaction] = useState<"like" | "dislike" | null>(null);
  const [likeCount, setLikeCount] = useState(initialLikes);
  const [dislikeCount, setDislikeCount] = useState(initialDislikes);
  const [busy, setBusy] = useState(false);
  const [reported, setReported] = useState(false);
  const [reportOpen, setReportOpen] = useState(false);
  const interactionBlocked = !capabilities.can_interact;
  const disabled = !user || interactionBlocked || busy;
  const denialKey = interactionDenialKey(capabilities.interaction_denial_reason);

  useEffect(() => {
    if (!user) return;
    void fetchMyReaction();
  }, [user, contentId]);

  async function fetchMyReaction() {
    try {
      const data = await api.get<{
        reaction?: { reaction: string };
      }>(`/api/v1/social/reactions?target_type=content&target_id=${contentId}`);
      if (data.reaction) {
        setMyReaction(data.reaction.reaction as "like" | "dislike");
      }
    } catch (e) { silentError(e, { component: 'ReactionBar', action: 'fetchMyReaction' }); }
  }

  const react = useCallback(
    async (reaction: "like" | "dislike") => {
      if (!user || interactionBlocked || busy) return;
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
        await api.post("/api/v1/social/reactions", {
          target_type: "content",
          target_id: contentId,
          reaction,
        });
      } catch (e) {
        silentError(e, { component: 'ReactionBar', action: 'react' });
        setMyReaction(prevReaction);
        setLikeCount(prevLikes);
        setDislikeCount(prevDislikes);
      } finally {
        setBusy(false);
      }
    },
    [user, interactionBlocked, busy, myReaction, likeCount, dislikeCount, contentId],
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
        onClick={() => react("like")}
        title={disabled ? t(denialKey) : t('social.like')}
      >
        <ThumbsUp className="mr-1 h-3.5 w-3.5" />
        {likeCount}
      </Button>

      <Button
        variant={myReaction === "dislike" ? "default" : "outline"}
        size="sm"
        disabled={disabled}
        onClick={() => react("dislike")}
        title={disabled ? t(denialKey) : t('social.dislike')}
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
        title={reported ? t('social.reported') : disabled ? t(denialKey) : t('social.report')}
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
