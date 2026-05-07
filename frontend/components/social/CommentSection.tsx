"use client";

import { useState, useEffect, useCallback } from "react";
import { useTranslations } from "next-intl";
import { MessageCircle, Send, ThumbsUp, ThumbsDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { cn } from "@/lib/utils";

interface Comment {
  id: number;
  author_id: number;
  author?: { id: number; username: string; avatar_url?: string };
  body: string;
  like_count: number;
  parent_id: number | null;
  created_at: string;
}

interface CommentSectionProps {
  contentId: number;
  className?: string;
}

export function CommentSection({ contentId, className }: CommentSectionProps) {
  const t = useTranslations();
  const { user } = useAuth();
  const [comments, setComments] = useState<Comment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);

  const canComment = user && user.reputation >= 3;

  useEffect(() => {
    void loadComments();
  }, [contentId]);

  async function loadComments() {
    setError("");
    setLoading(true);
    try {
      const data = await api.get<{ comments?: Comment[] }>(
        `/api/v1/social/comments?content_item_id=${contentId}&page=1&page_size=50`,
      );
      setComments(data.comments || []);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t('social.loadFailed'));
    } finally {
      setLoading(false);
    }
  }

  const submit = useCallback(async () => {
    if (!user || !body.trim() || busy) return;
    setBusy(true);
    try {
      const data = await api.post<{ comment: Comment }>("/api/v1/social/comments", {
        content_item_id: contentId,
        body: body.trim(),
      });
      setComments((prev) => [...prev, data.comment]);
      setBody("");
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : t('social.sendFailed'));
    } finally {
      setBusy(false);
    }
  }, [user, body, busy, contentId, t]);

  async function reactToComment(commentId: number, reaction: "like" | "dislike") {
    if (!user) return;
    try {
      await api.post("/api/v1/social/reactions", {
        target_type: "comment",
        target_id: commentId,
        reaction,
      });
    } catch { /* ignore */ }
  }

  return (
    <div className={cn("space-y-4", className)}>
      <div className="flex items-center gap-2">
        <MessageCircle className="h-4 w-4 text-muted-foreground" />
        <h3 className="text-sm font-semibold">{t('social.comments')}</h3>
      </div>

      {user ? (
        <div className="flex gap-2">
          <textarea
            className="min-h-[60px] flex-1 rounded-md border border-border bg-card px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"
            placeholder={canComment ? t('social.commentPlaceholder') : t('social.cannotComment')}
            value={body}
            onChange={(e) => setBody(e.target.value)}
            disabled={!canComment || busy}
            onKeyDown={(e) => {
              if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
                e.preventDefault();
                void submit();
              }
            }}
          />
          <Button
            size="sm"
            disabled={!canComment || !body.trim() || busy}
            onClick={() => void submit()}
            className="self-end"
          >
            <Send className="h-3.5 w-3.5" />
          </Button>
        </div>
      ) : (
        <p className="rounded-md border border-border bg-muted/20 p-3 text-center text-xs text-muted-foreground">
          {t('social.loginToComment')}
        </p>
      )}

      {error && <p className="text-xs text-destructive">{error}</p>}

      {loading ? (
        <div className="space-y-3">
          {[1, 2, 3].map((i) => (
            <div key={i} className="animate-pulse space-y-2 rounded-md border border-border p-3">
              <div className="h-3 w-24 rounded bg-muted" />
              <div className="h-4 w-full rounded bg-muted/50" />
            </div>
          ))}
        </div>
      ) : comments.length === 0 ? (
        <p className="rounded-md border border-border bg-muted/10 p-4 text-center text-xs text-muted-foreground">
          {t('social.noComments')}
        </p>
      ) : (
        <div className="space-y-3">
          {comments
            .filter((c) => !c.parent_id)
            .map((comment) => (
              <CommentItem
                key={comment.id}
                comment={comment}
                replies={comments.filter((c) => c.parent_id === comment.id)}
                onReact={reactToComment}
              />
            ))}
        </div>
      )}
    </div>
  );
}

function CommentItem({
  comment,
  replies,
  onReact,
}: {
  comment: Comment;
  replies: Comment[];
  onReact: (id: number, reaction: "like" | "dislike") => void;
}) {
  const t = useTranslations();
  const { user } = useAuth();

  return (
    <div className="rounded-md border border-border bg-card p-3 shadow-none">
      <div className="flex items-center justify-between">
        <p className="text-xs font-medium">
          {comment.author?.username ?? t('common.userLabel', { id: comment.author_id })}
        </p>
        <p className="text-[10px] text-muted-foreground">
          {new Date(comment.created_at).toLocaleDateString("zh-CN", {
            year: "numeric",
            month: "2-digit",
            day: "2-digit",
            hour: "2-digit",
            minute: "2-digit",
          })}
        </p>
      </div>
      <p className="mt-1 text-sm leading-relaxed text-foreground/90">{comment.body}</p>
      <div className="mt-2 flex items-center gap-2">
        <button
          className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground disabled:opacity-50"
          disabled={!user}
          onClick={() => onReact(comment.id, "like")}
        >
          <ThumbsUp className="h-3 w-3" />
          {comment.like_count}
        </button>
        <button
          className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground disabled:opacity-50"
          disabled={!user}
          onClick={() => onReact(comment.id, "dislike")}
        >
          <ThumbsDown className="h-3 w-3" />
        </button>
      </div>

      {replies.length > 0 && (
        <div className="ml-4 mt-2 space-y-2 border-l-2 border-border pl-3">
          {replies.map((reply) => (
            <div key={reply.id} className="rounded border border-border bg-muted/10 p-2">
              <div className="flex items-center justify-between">
                <p className="text-xs font-medium">
                  {reply.author?.username ?? t('common.userLabel', { id: reply.author_id })}
                </p>
                <p className="text-[10px] text-muted-foreground">
                  {new Date(reply.created_at).toLocaleDateString("zh-CN")}
                </p>
              </div>
              <p className="mt-0.5 text-xs text-foreground/80">{reply.body}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
