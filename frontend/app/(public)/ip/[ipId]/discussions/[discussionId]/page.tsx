"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { api, ApiRequestError } from "@/lib/api";
import { ReplyList } from "@/components/social/ReplyList";

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
      const [discRes, repliesRes] = await Promise.all([
        api.get<{ discussion?: DiscussionDetail }>(`/api/v1/discussions/${discussionId}`),
        api.get<{ comments?: ReplyData[] }>(`/api/v1/discussions/${discussionId}/comments`),
      ]);
      setDiscussion(discRes.discussion ?? null);
      setReplies(repliesRes.comments ?? []);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t("common.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [discussionId, t]);

  useEffect(() => { load(); }, [load]);

  if (loading) {
    return <div className="mx-auto max-w-3xl px-4 py-6 text-sm text-muted-foreground">{t("common.loading")}</div>;
  }

  if (!discussion) {
    return <div className="mx-auto max-w-3xl px-4 py-6 text-sm text-destructive">{t("discussion.notFound")}</div>;
  }

  return (
    <div className="mx-auto w-full max-w-3xl space-y-6 px-4 py-6">
      <div className="rounded-md border border-border bg-card p-6 shadow-none">
        <h1 className="text-xl font-bold tracking-tight">{discussion.title}</h1>
        <div className="mt-2 flex items-center gap-3 text-xs text-muted-foreground">
          {discussion.author?.username && <span>{discussion.author.username}</span>}
          {discussion.created_at && <span>{new Date(discussion.created_at).toLocaleDateString()}</span>}
        </div>
        {discussion.body && (
          <div className="mt-4 text-sm leading-relaxed whitespace-pre-wrap">{discussion.body}</div>
        )}
      </div>

      <div className="rounded-md border border-border bg-card p-4 shadow-none">
        <h2 className="mb-4 text-sm font-semibold">
          {t("discussion.replyCount", { count: replies.length })}
        </h2>
        {error && <p className="text-sm text-destructive mb-2">{error}</p>}
        <ReplyList discussionId={discussionId} replies={replies} onRefresh={load} />
      </div>
    </div>
  );
}
