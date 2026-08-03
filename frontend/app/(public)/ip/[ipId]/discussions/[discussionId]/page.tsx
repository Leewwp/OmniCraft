"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { ReplyList } from "@/components/social/ReplyList";
import { silentError } from "@/lib/error-handler";
import { useToast } from "@/components/ui/Toast";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { AlertCircle, MessageSquare } from "lucide-react";

interface DiscussionDetail {
  id: number;
  title: string;
  body?: string;
  ip_id?: number;
  author?: { id?: number; username?: string };
  created_at?: string;
  reply_count?: number;
}

interface ReplyData {
  id: number;
  author?: { id?: number; username?: string };
  body: string;
  parent_id?: number | null;
  created_at?: string;
}

export default function DiscussionDetailPage() {
  const t = useTranslations();
  const { toast } = useToast();
  const params = useParams<{ ipId: string; discussionId: string }>();
  const discussionId = parseInt(params.discussionId, 10);
  const [discussion, setDiscussion] = useState<DiscussionDetail | null>(null);
  const [replies, setReplies] = useState<ReplyData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const res = await api.get<{ discussion?: DiscussionDetail; comments?: ReplyData[] }>(
        `/api/v1/discussions/${discussionId}`,
      );
      setDiscussion(res.discussion ?? null);
      setReplies(res.comments ?? []);
    } catch (e) {
      silentError(e, { component: 'DiscussionDetailPage', action: 'load' });
      const message = t(getUserFacingErrorKey(e, "common.loadFailed"));
      setError(message);
      toast("error", message);
    } finally {
      setLoading(false);
    }
  }, [discussionId, t]);

  useEffect(() => { load(); }, [load]);

  if (loading) {
    return (
      <div className="mx-auto min-h-[560px] w-full max-w-full min-[701px]:max-w-[720px] min-[1101px]:max-w-[960px] space-y-6 px-4 py-6" aria-busy="true" aria-live="polite">
        <span className="sr-only" role="status">{t("common.loading")}</span>
        <div className="rounded-md border border-border bg-card p-6">
          <Skeleton className="h-7 w-2/3" />
          <Skeleton className="mt-3 h-4 w-1/3" />
          <Skeleton className="mt-6 h-24 w-full" />
        </div>
        <div className="rounded-md border border-border bg-card p-4">
          <Skeleton className="mb-4 h-5 w-28" />
          <div className="space-y-3"><Skeleton className="h-14 w-full" /><Skeleton className="h-14 w-full" /></div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="mx-auto flex min-h-[560px] w-full max-w-full min-[701px]:max-w-[720px] min-[1101px]:max-w-[960px] items-center justify-center px-4 py-6" aria-live="polite">
        <div className="w-full p-6 text-center">
          <p className="text-sm text-destructive" role="alert">{error}</p>
          <AlertCircle className="mx-auto size-6 text-destructive" aria-hidden="true" />
          <Button className="mt-4" type="button" variant="outline" onClick={() => void load()}>{t("common.retry")}</Button>
        </div>
      </div>
    );
  }

  if (!discussion) {
    return (
      <div className="mx-auto min-h-[560px] max-w-full min-[701px]:max-w-[720px] min-[1101px]:max-w-[960px] px-4 py-6" aria-live="polite">
        <EmptyState icon={MessageSquare} title={t("discussion.notFound")} />
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-full min-[701px]:max-w-[720px] min-[1101px]:max-w-[960px] space-y-6 px-4 py-6">
      <div className="rounded-md border border-border bg-card p-6 ">
        <h1 className="text-xl font-bold tracking-tight">{discussion.title}</h1>
        <div className="mt-2 flex items-center gap-3 text-xs text-muted-foreground">
          {discussion.author?.username && <span>{discussion.author.username}</span>}
          {discussion.created_at && <span>{new Date(discussion.created_at).toLocaleDateString()}</span>}
        </div>
        {discussion.body && (
          <div className="mt-4 text-sm leading-relaxed whitespace-pre-wrap">{discussion.body}</div>
        )}
      </div>

      <div className="rounded-md border border-border bg-card p-4 ">
        <h2 className="mb-4 text-sm font-semibold">
          {t("discussion.replyCount", { count: replies.length })}
        </h2>
        {error && <p className="text-sm text-destructive mb-2">{error}</p>}
        <ReplyList discussionId={discussionId} replies={replies} onRefresh={load} />
      </div>
    </div>
  );
}
