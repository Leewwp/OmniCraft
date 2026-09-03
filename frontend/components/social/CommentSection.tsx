"use client";

import { useState, useEffect, useCallback } from "react";
import { useTranslations, useLocale } from "next-intl";
import { MessageCircle, Send, ThumbsUp, ThumbsDown, Reply, Pencil, Trash2, Flag } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { useToast } from "@/components/ui/Toast";
import { useAuth, interactionDenialKey } from "@/contexts/AuthContext";
import { api, ApiRequestError } from "@/lib/api";
import { fetchPublicConfig, commentFoldThreshold, isHighDislikeRatio } from "@/lib/public-config";
import { silentError } from "@/lib/error-handler";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { cn } from "@/lib/utils";

interface Comment {
  id: number;
  author_id: number;
  author?: { id: number; username: string; avatar_url?: string };
  body: string;
  like_count?: number;
  dislike_count?: number;
  parent_id: number | null;
  created_at: string;
  updated_at?: string;
}

interface CommentSectionProps {
  contentId: number;
  className?: string;
}

const PAGE_SIZE = 20;

export function CommentSection({ contentId, className }: CommentSectionProps) {
  const t = useTranslations();
  const { user, capabilities } = useAuth();
  const [comments, setComments] = useState<Comment[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);
  /* 每条顶层评论的子回复：懒加载（GET ?parent_id=），两级扁平展示 */
  const [repliesByRoot, setRepliesByRoot] = useState<Record<number, Comment[]>>({});
  const [expandedRoots, setExpandedRoots] = useState<Set<number>>(new Set());
  /* T47：折叠阈值随公开配置下发（business-rules：点踩/点赞比 ≥ 阈值默认折叠） */
  const [foldThreshold, setFoldThreshold] = useState(0.30);

  useEffect(() => {
    let active = true;
    fetchPublicConfig()
      .then((config) => {
        if (active) setFoldThreshold(commentFoldThreshold(config));
      })
      .catch(() => {
        /* 配置不可用时保持基线兜底 */
      });
    return () => {
      active = false;
    };
  }, []);

  const canComment = !!user && capabilities.can_interact;
  const denialKey = interactionDenialKey(capabilities.interaction_denial_reason);

  useEffect(() => {
    void loadComments();
  }, [contentId]);

  async function loadComments() {
    setError("");
    setLoading(true);
    try {
      const data = await api.get<{ comments?: Comment[]; total?: number }>(
        `/api/v1/social/comments?content_item_id=${contentId}&page=1&page_size=${PAGE_SIZE}`,
      );
      setComments(data.comments || []);
      setTotal(data.total ?? data.comments?.length ?? 0);
    } catch (e) {
      setError(t(getUserFacingErrorKey(e, "social.loadFailed")));
      silentError(e, { component: 'CommentSection', action: 'loadComments' });
    } finally {
      setLoading(false);
    }
  }

  async function loadMore() {
    if (loadingMore || comments.length >= total) return;
    setLoadingMore(true);
    try {
      const nextPage = Math.ceil(comments.length / PAGE_SIZE) + 1;
      const data = await api.get<{ comments?: Comment[]; total?: number }>(
        `/api/v1/social/comments?content_item_id=${contentId}&page=${nextPage}&page_size=${PAGE_SIZE}`,
      );
      const existing = new Set(comments.map((c) => c.id));
      setComments((prev) => [...prev, ...(data.comments || []).filter((c) => !existing.has(c.id))]);
      setTotal(data.total ?? total);
    } catch (e) {
      setError(t(getUserFacingErrorKey(e, "social.loadFailed")));
      silentError(e, { component: 'CommentSection', action: 'loadMore' });
    } finally {
      setLoadingMore(false);
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
      setTotal((prev) => prev + 1);
      setBody("");
    } catch (e) {
      setError(t(getUserFacingErrorKey(e, "social.sendFailed")));
      silentError(e, { component: 'CommentSection', action: 'submit' });
    } finally {
      setBusy(false);
    }
  }, [user, body, busy, contentId, t]);

  async function loadReplies(rootId: number) {
    const expanded = expandedRoots.has(rootId);
    if (expanded) {
      setExpandedRoots((prev) => { const next = new Set(prev); next.delete(rootId); return next; });
      return;
    }
    if (!repliesByRoot[rootId]) {
      try {
        const data = await api.get<{ comments?: Comment[] }>(
          `/api/v1/social/comments?content_item_id=${contentId}&parent_id=${rootId}&page=1&page_size=${PAGE_SIZE}`,
        );
        setRepliesByRoot((prev) => ({ ...prev, [rootId]: data.comments || [] }));
      } catch (e) {
        setError(t(getUserFacingErrorKey(e, "social.loadFailed")));
        silentError(e, { component: 'CommentSection', action: 'loadReplies' });
        return;
      }
    }
    setExpandedRoots((prev) => new Set(prev).add(rootId));
  }

  async function submitReply(rootId: number, text: string) {
    const trimmed = text.trim();
    if (!user || !trimmed) return;
    try {
      /* 两级扁平：对回复的回复也挂到顶层根上，展示不产生第三层 */
      const data = await api.post<{ comment: Comment }>("/api/v1/social/comments", {
        content_item_id: contentId,
        parent_id: rootId,
        body: trimmed,
      });
      setRepliesByRoot((prev) => ({ ...prev, [rootId]: [...(prev[rootId] || []), data.comment] }));
      setExpandedRoots((prev) => new Set(prev).add(rootId));
    } catch (e) {
      setError(t(getUserFacingErrorKey(e, "social.sendFailed")));
      silentError(e, { component: 'CommentSection', action: 'submitReply' });
    }
  }

  async function reactToComment(commentId: number, reaction: "like" | "dislike") {
    if (!user) return;
    try {
      await api.post("/api/v1/social/reactions", {
        target_type: "comment",
        target_id: commentId,
        reaction,
      });
    } catch (e) { silentError(e, { component: 'CommentSection', action: 'reactToComment' }); }
  }

  async function saveEdit(commentId: number, newText: string): Promise<boolean> {
    try {
      const data = await api.patch<{ comment: Comment }>(`/api/v1/social/comments/${commentId}`, {
        body: newText.trim(),
      });
      setComments((prev) => prev.map((c) => (c.id === commentId ? { ...c, body: data.comment.body, updated_at: data.comment.updated_at } : c)));
      setRepliesByRoot((prev) => {
        const next: Record<number, Comment[]> = {};
        for (const [root, list] of Object.entries(prev)) {
          next[Number(root)] = list.map((c) => (c.id === commentId ? { ...c, body: data.comment.body, updated_at: data.comment.updated_at } : c));
        }
        return next;
      });
      return true;
    } catch (e) {
      setError(t(getUserFacingErrorKey(e, "social.sendFailed")));
      silentError(e, { component: 'CommentSection', action: 'saveEdit' });
      return false;
    }
  }

  async function deleteComment(comment: Comment, rootId?: number) {
    try {
      await api.delete(`/api/v1/social/comments/${comment.id}`);
      if (rootId === undefined) {
        /* 顶层删除：整棵子树一并撤下（后端只隐藏该条，子回复对 UI 不再可达） */
        setComments((prev) => prev.filter((c) => c.id !== comment.id));
        setRepliesByRoot((prev) => {
          const next = { ...prev };
          delete next[comment.id];
          return next;
        });
        setExpandedRoots((prev) => { const n = new Set(prev); n.delete(comment.id); return n; });
        setTotal((prev) => Math.max(0, prev - 1));
      } else {
        setRepliesByRoot((prev) => ({
          ...prev,
          [rootId]: (prev[rootId] || []).filter((c) => c.id !== comment.id),
        }));
      }
    } catch (e) {
      setError(t(getUserFacingErrorKey(e, "social.sendFailed")));
      silentError(e, { component: 'CommentSection', action: 'deleteComment' });
    }
  }

  return (
    <div className={cn("space-y-4", className)}>
      <div className="flex items-center gap-2">
        <MessageCircle className="h-4 w-4 text-muted-foreground" />
        <h3 className="text-sm font-semibold">
          {t('social.comments')}
          {total > 0 && <span className="ml-1.5 text-xs font-normal text-muted-foreground">{t('social.commentCount', { count: total })}</span>}
        </h3>
      </div>

      {user ? (
        <div className="flex gap-2">
          <textarea
            className="min-h-[60px] flex-1 rounded-md border border-border bg-card px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"
            placeholder={canComment ? t('social.commentPlaceholder') : t(denialKey)}
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

      {error && <p className="text-xs text-destructive" role="alert">{error}</p>}

      {loading ? (
        <div className="space-y-3">
          {[1, 2, 3].map((i) => (
            <div key={i} className="animate-pulse space-y-2 rounded-md border border-border p-3">
              <Skeleton className="h-3 w-24" />
              <Skeleton className="h-4 w-full" />
            </div>
          ))}
        </div>
      ) : comments.length === 0 ? (
        <EmptyState
          icon={MessageCircle}
          title={t("social.noComments")}
          description={t("social.noCommentsHint")}
          className="p-4"
        />
      ) : (
        <>
          <div className="space-y-3">
            {comments.map((comment) => (
              <CommentItem
                key={comment.id}
                comment={comment}
                replies={expandedRoots.has(comment.id) ? repliesByRoot[comment.id] ?? [] : undefined}
                repliesLoaded={comment.id in repliesByRoot}
                expanded={expandedRoots.has(comment.id)}
                canInteract={canComment}
                currentUserId={user?.id}
                onToggleReplies={() => void loadReplies(comment.id)}
                onReply={(text) => void submitReply(comment.id, text)}
                onReact={reactToComment}
                onSaveEdit={saveEdit}
                onDelete={(target, rootId) => void deleteComment(target, rootId)}
                foldThreshold={foldThreshold}
              />
            ))}
          </div>
          {comments.length < total && (
            <Button variant="outline" size="sm" className="w-full" disabled={loadingMore} onClick={() => void loadMore()}>
              {loadingMore ? t('common.loading') : t('social.loadMoreComments')}
            </Button>
          )}
        </>
      )}
    </div>
  );
}

function CommentAvatar({ author }: { author?: Comment["author"] }) {
  const initial = (author?.username || "?").charAt(0).toUpperCase();
  if (author?.avatar_url) {
    return (
      <img
        src={author.avatar_url}
        alt=""
        className="h-8 w-8 shrink-0 rounded-full object-cover"
        loading="lazy"
      />
    );
  }
  return (
    <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-accent-subtle text-xs font-semibold text-accent-emphasis" aria-hidden="true">
      {initial}
    </span>
  );
}

function CommentItem({
  comment,
  replies,
  repliesLoaded,
  expanded,
  canInteract,
  currentUserId,
  foldThreshold,
  onToggleReplies,
  onReply,
  onReact,
  onSaveEdit,
  onDelete,
}: {
  comment: Comment;
  replies: Comment[] | undefined;
  repliesLoaded: boolean;
  expanded: boolean;
  canInteract: boolean;
  currentUserId?: number;
  foldThreshold: number;
  onToggleReplies: () => void;
  onReply: (text: string) => void;
  onReact: (id: number, reaction: "like" | "dislike") => void;
  onSaveEdit: (id: number, newText: string) => Promise<boolean>;
  onDelete: (comment: Comment, rootId?: number) => void;
}) {
  const t = useTranslations();
  const locale = useLocale();
  const { user } = useAuth();
  const { toast } = useToast();

  const reactionDisabled = !user || !canInteract;
  const isOwn = currentUserId !== undefined && comment.author_id === currentUserId;
  /* T47：高踩比默认折叠，点击「已折叠 · 点击显示」展开 */
  const shouldFold = isHighDislikeRatio(comment.like_count ?? 0, comment.dislike_count ?? 0, foldThreshold);
  const [foldRevealed, setFoldRevealed] = useState(false);
  const folded = shouldFold && !foldRevealed;

  const [replyOpen, setReplyOpen] = useState(false);
  const [replyText, setReplyText] = useState("");
  const [editing, setEditing] = useState(false);
  const [editText, setEditText] = useState("");
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [reportOpen, setReportOpen] = useState(false);
  /* T54：已举报态（成功或 409 重复举报都进入；重复举报有提示文案） */
  const [reported, setReported] = useState(false);

  /* T54：举报走既有端点；409 提示后与成功同样进入已举报态（ReactionBar 同款语义） */
  async function submitReport(reason: string) {
    try {
      await api.post(`/api/v1/social/comments/${comment.id}/report`, { reason });
      setReported(true);
      toast("success", t('social.reported'));
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 409) {
        setReported(true);
        toast("error", t(getUserFacingErrorKey(e, "social.reportFailed")));
        return;
      }
      toast("error", t(getUserFacingErrorKey(e, "social.reportFailed")));
      silentError(e, { component: 'CommentSection', action: 'reportComment' });
    }
  }

  async function confirmEdit() {
    const trimmed = editText.trim();
    if (!trimmed) return;
    if (await onSaveEdit(comment.id, trimmed)) setEditing(false);
  }

  return (
    <div className="rounded-md border border-border bg-card p-3">
      <div className="flex items-start gap-2.5">
        <CommentAvatar author={comment.author} />
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <p className="truncate text-xs font-medium">
              {comment.author?.username || t('common.userLabel', { id: comment.author_id })}
            </p>
            <p className="shrink-0 text-[10px] text-muted-foreground">
              {new Date(comment.created_at).toLocaleDateString(locale === "en" ? "en-US" : "zh-CN", {
                year: "numeric",
                month: "2-digit",
                day: "2-digit",
                hour: "2-digit",
                minute: "2-digit",
              })}
              {comment.updated_at && comment.updated_at !== comment.created_at && (
                <span className="ml-1">· {t('social.editedMark')}</span>
              )}
            </p>
          </div>

          {folded ? (
            /* T47：高踩比默认折叠——仅保留作者行与展开开关 */
            <button
              type="button"
              className="mt-1.5 inline-flex items-center gap-1.5 rounded-md px-1.5 py-1 text-xs text-muted-foreground transition-colors duration-150 hover:bg-muted/50 hover:text-foreground"
              onClick={() => setFoldRevealed(true)}
            >
              {t('social.commentFolded')}
            </button>
          ) : (
            <>
          {editing ? (
            <div className="mt-1.5 space-y-1.5">
              <textarea
                className="min-h-[52px] w-full rounded-md border border-border bg-card px-2.5 py-1.5 text-sm text-foreground focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
                value={editText}
                onChange={(e) => setEditText(e.target.value)}
                maxLength={5000}
                aria-label={t('social.edit')}
              />
              <div className="flex gap-1.5">
                <Button size="sm" className="h-7" disabled={!editText.trim()} onClick={() => void confirmEdit()}>
                  {t('social.saveEdit')}
                </Button>
                <Button size="sm" variant="outline" className="h-7" onClick={() => setEditing(false)}>
                  {t('social.cancelEdit')}
                </Button>
              </div>
            </div>
          ) : (
            <p className="mt-1 text-sm leading-relaxed text-foreground/90">{comment.body}</p>
          )}

          <div className="mt-2 flex flex-wrap items-center gap-1">
            <button
              type="button"
              className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors duration-150 hover:bg-muted/50 hover:text-foreground disabled:opacity-50"
              disabled={reactionDisabled}
              aria-label={t('social.like')}
              onClick={() => onReact(comment.id, "like")}
            >
              <ThumbsUp className="h-3 w-3" />
              {(comment.like_count ?? 0) > 0 && <span>{comment.like_count}</span>}
            </button>
            <button
              type="button"
              className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors duration-150 hover:bg-muted/50 hover:text-foreground disabled:opacity-50"
              disabled={reactionDisabled}
              aria-label={t('social.dislike')}
              onClick={() => onReact(comment.id, "dislike")}
            >
              <ThumbsDown className="h-3 w-3" />
              {(comment.dislike_count ?? 0) > 0 && <span>{comment.dislike_count}</span>}
            </button>
            {user && (
              <button
                type="button"
                className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors duration-150 hover:bg-muted/50 hover:text-foreground"
                onClick={() => setReplyOpen((prev) => !prev)}
              >
                <Reply className="h-3 w-3" />
                {t('social.reply')}
              </button>
            )}
            {user && (reported ? (
              <span className="inline-flex items-center gap-1 px-1.5 py-0.5 text-xs text-muted-foreground" aria-label={t('social.reported')}>
                <Flag className="h-3 w-3" />
                {t('social.reported')}
              </span>
            ) : (
              <button
                type="button"
                className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors duration-150 hover:bg-muted/50 hover:text-foreground"
                onClick={() => setReportOpen(true)}
              >
                <Flag className="h-3 w-3" />
                {t('social.report')}
              </button>
            ))}
            {isOwn && !editing && (
              <>
                <button
                  type="button"
                  className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors duration-150 hover:bg-muted/50 hover:text-foreground"
                  onClick={() => { setEditText(comment.body); setEditing(true); }}
                >
                  <Pencil className="h-3 w-3" />
                  {t('social.edit')}
                </button>
                <button
                  type="button"
                  className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors duration-150 hover:bg-muted/50 hover:text-foreground"
                  onClick={() => setDeleteOpen(true)}
                >
                  <Trash2 className="h-3 w-3" />
                  {t('social.delete')}
                </button>
              </>
            )}
          </div>

          {replyOpen && (
            <div className="mt-2 flex gap-2">
              <textarea
                className="min-h-[44px] flex-1 rounded-md border border-border bg-card px-2.5 py-1.5 text-sm text-foreground placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
                placeholder={t('social.replyPlaceholder', { name: comment.author?.username || `#${comment.author_id}` })}
                value={replyText}
                onChange={(e) => setReplyText(e.target.value)}
                maxLength={5000}
              />
              <Button
                size="sm"
                className="h-8 self-end"
                disabled={!replyText.trim()}
                onClick={() => { onReply(replyText); setReplyText(""); setReplyOpen(false); }}
              >
                {t('social.reply')}
              </Button>
            </div>
          )}

          <div className="mt-1.5">
            <button
              type="button"
              className="text-xs text-muted-foreground underline-offset-2 transition-colors duration-150 hover:text-foreground hover:underline"
              onClick={onToggleReplies}
            >
              {expanded ? t('social.hideReplies') : t('social.showReplies')}
            </button>
            {expanded && replies !== undefined && replies.length === 0 && (
              <span className="ml-2 text-xs text-muted-foreground">{t('social.noReplies')}</span>
            )}
          </div>

          {expanded && replies !== undefined && replies.length > 0 && (
            <div className="mt-2 space-y-2 border-l-2 border-border pl-3">
              {replies.map((reply) => (
                <ReplyItem
                  key={reply.id}
                  reply={reply}
                  rootId={comment.id}
                  canInteract={canInteract}
                  currentUserId={currentUserId}
                  foldThreshold={foldThreshold}
                  onReact={onReact}
                  onSaveEdit={onSaveEdit}
                  onDelete={onDelete}
                />
              ))}
            </div>
          )}
          {shouldFold && (
            <button
              type="button"
              className="mt-1 text-xs text-muted-foreground underline-offset-2 transition-colors duration-150 hover:text-foreground hover:underline"
              onClick={() => setFoldRevealed(false)}
            >
              {t('social.commentCollapse')}
            </button>
          )}
            </>
          )}
        </div>
      </div>

      <ConfirmModal
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('social.deleteCommentTitle')}
        description={t('social.deleteCommentDesc')}
        confirmVariant="destructive"
        onConfirm={() => { onDelete(comment); setDeleteOpen(false); }}
      />

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

function ReplyItem({
  reply,
  rootId,
  canInteract,
  currentUserId,
  foldThreshold,
  onReact,
  onSaveEdit,
  onDelete,
}: {
  reply: Comment;
  rootId: number;
  canInteract: boolean;
  currentUserId?: number;
  foldThreshold: number;
  onReact: (id: number, reaction: "like" | "dislike") => void;
  onSaveEdit: (id: number, newText: string) => Promise<boolean>;
  onDelete: (comment: Comment, rootId?: number) => void;
}) {
  const t = useTranslations();
  const locale = useLocale();
  const { user } = useAuth();
  const { toast } = useToast();

  const reactionDisabled = !user || !canInteract;
  const isOwn = currentUserId !== undefined && reply.author_id === currentUserId;
  /* T47：高踩比回复同样默认折叠 */
  const shouldFold = isHighDislikeRatio(reply.like_count ?? 0, reply.dislike_count ?? 0, foldThreshold);
  const [foldRevealed, setFoldRevealed] = useState(false);
  const folded = shouldFold && !foldRevealed;
  const [editing, setEditing] = useState(false);
  const [editText, setEditText] = useState("");
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [reportOpen, setReportOpen] = useState(false);
  const [reported, setReported] = useState(false);

  async function submitReport(reason: string) {
    try {
      await api.post(`/api/v1/social/comments/${reply.id}/report`, { reason });
      setReported(true);
      toast("success", t('social.reported'));
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 409) {
        setReported(true);
        toast("error", t(getUserFacingErrorKey(e, "social.reportFailed")));
        return;
      }
      toast("error", t(getUserFacingErrorKey(e, "social.reportFailed")));
      silentError(e, { component: 'CommentSection', action: 'reportReply' });
    }
  }

  async function confirmEdit() {
    const trimmed = editText.trim();
    if (!trimmed) return;
    if (await onSaveEdit(reply.id, trimmed)) setEditing(false);
  }

  return (
    <div className="rounded border border-border bg-muted/10 p-2">
      <div className="flex items-center justify-between">
        <p className="text-xs font-medium">
          {reply.author?.username || t('common.userLabel', { id: reply.author_id })}
        </p>
        <p className="text-[10px] text-muted-foreground">
          {new Date(reply.created_at).toLocaleDateString(locale === "en" ? "en-US" : "zh-CN")}
          {reply.updated_at && reply.updated_at !== reply.created_at && (
            <span className="ml-1">· {t('social.editedMark')}</span>
          )}
        </p>
      </div>

      {folded ? (
        <button
          type="button"
          className="mt-1 inline-flex items-center gap-1.5 rounded-md px-1.5 py-1 text-xs text-muted-foreground transition-colors duration-150 hover:bg-muted/50 hover:text-foreground"
          onClick={() => setFoldRevealed(true)}
        >
          {t('social.commentFolded')}
        </button>
      ) : (
        <>
      {editing ? (
        <div className="mt-1 space-y-1.5">
          <textarea
            className="min-h-[44px] w-full rounded-md border border-border bg-card px-2.5 py-1.5 text-xs text-foreground focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
            value={editText}
            onChange={(e) => setEditText(e.target.value)}
            maxLength={5000}
          />
          <div className="flex gap-1.5">
            <Button size="sm" className="h-7" disabled={!editText.trim()} onClick={() => void confirmEdit()}>
              {t('social.saveEdit')}
            </Button>
            <Button size="sm" variant="outline" className="h-7" onClick={() => setEditing(false)}>
              {t('social.cancelEdit')}
            </Button>
          </div>
        </div>
      ) : (
        <p className="mt-0.5 text-xs text-foreground/80">{reply.body}</p>
      )}

      <div className="mt-1.5 flex items-center gap-1">
        <button
          type="button"
          className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors duration-150 hover:bg-muted/50 hover:text-foreground disabled:opacity-50"
          disabled={reactionDisabled}
          aria-label={t('social.like')}
          onClick={() => onReact(reply.id, "like")}
        >
          <ThumbsUp className="h-3 w-3" />
          {(reply.like_count ?? 0) > 0 && <span>{reply.like_count}</span>}
        </button>
        {user && (reported ? (
          <span className="inline-flex items-center gap-1 px-1.5 py-0.5 text-xs text-muted-foreground" aria-label={t('social.reported')}>
            <Flag className="h-3 w-3" />
            {t('social.reported')}
          </span>
        ) : (
          <button
            type="button"
            className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors duration-150 hover:bg-muted/50 hover:text-foreground"
            onClick={() => setReportOpen(true)}
          >
            <Flag className="h-3 w-3" />
            {t('social.report')}
          </button>
        ))}
        {isOwn && !editing && (
          <>
            <button
              type="button"
              className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors duration-150 hover:bg-muted/50 hover:text-foreground"
              onClick={() => { setEditText(reply.body); setEditing(true); }}
            >
              <Pencil className="h-3 w-3" />
              {t('social.edit')}
            </button>
            <button
              type="button"
              className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors duration-150 hover:bg-muted/50 hover:text-foreground"
              onClick={() => setDeleteOpen(true)}
            >
              <Trash2 className="h-3 w-3" />
              {t('social.delete')}
            </button>
          </>
        )}
      </div>

      {shouldFold && !folded && (
        <button
          type="button"
          className="mt-1 text-xs text-muted-foreground underline-offset-2 transition-colors duration-150 hover:text-foreground hover:underline"
          onClick={() => setFoldRevealed(false)}
        >
          {t('social.commentCollapse')}
        </button>
      )}
        </>
      )}

      <ConfirmModal
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('social.deleteReplyTitle')}
        description={t('social.deleteReplyDesc')}
        confirmVariant="destructive"
        onConfirm={() => { onDelete(reply, rootId); setDeleteOpen(false); }}
      />

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
