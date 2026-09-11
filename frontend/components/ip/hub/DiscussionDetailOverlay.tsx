"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { MessageSquareWarning, X } from "lucide-react";
import { ReplyList } from "@/components/social/ReplyList";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { api } from "@/lib/api";

interface ReplyData {
  id: number;
  author?: { id?: number; username?: string };
  body: string;
  parent_id?: number | null;
  created_at?: string;
}

interface DiscussionDetail {
  id: number;
  title: string;
  body?: string;
  author?: { id?: number; username?: string };
  created_at?: string;
  reply_count?: number;
}

interface DiscussionDetailOverlayProps {
  discussionId: number;
  onClose: () => void;
}

const OVERLAY_HISTORY_KEY = "ipHubDiscussionOverlay";

// 讨论帖详情浮层（#290 新内容）：页内浮层阅读 + 回帖，Esc / 浏览器后退 /
// 点遮罩关闭（story 16）。打开时压一条 history 记录，程序化关闭回退该记录。
export function DiscussionDetailOverlay({ discussionId, onClose }: DiscussionDetailOverlayProps) {
  const t = useTranslations();
  const [discussion, setDiscussion] = useState<DiscussionDetail | null>(null);
  const [replies, setReplies] = useState<ReplyData[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") closeRef.current();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const closeRef = useRef<() => void>(() => {});
  closeRef.current = () => {
    if (window.history.state?.[OVERLAY_HISTORY_KEY]) window.history.go(-1);
    onCloseRef.current();
  };
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  // 挂载时压一条 history 记录（仅一次，不随父组件重渲染重复压栈）；
  // 后退键 popstate 弹出该记录时关闭浮层（history.go 已回退，无需再退）。
  useEffect(() => {
    window.history.pushState({ ...(window.history.state ?? {}), [OVERLAY_HISTORY_KEY]: 1 }, "");
    const handlePopState = () => onCloseRef.current();
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.get<{ discussion?: DiscussionDetail; comments?: ReplyData[] }>(
        `/api/v1/discussions/${discussionId}`,
      );
      setDiscussion(res.discussion ?? null);
      setReplies(res.comments ?? []);
    } catch (e) {
      silentError(e, { component: "DiscussionDetailOverlay", action: "load" });
      setDiscussion(null);
    } finally {
      setLoading(false);
    }
  }, [discussionId]);

  useEffect(() => { void load(); }, [load]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      role="dialog"
      aria-modal="true"
      aria-label={discussion?.title ?? t('ip.hubTab_discussions')}
      onClick={(e) => { if (e.target === e.currentTarget) closeRef.current(); }}
    >
      <div className="flex max-h-[86vh] w-full max-w-2xl flex-col overflow-hidden rounded-md border border-border bg-card">
        <div className="flex items-start justify-between gap-3 border-b border-border p-4">
          <div className="min-w-0">
            {loading || !discussion ? (
              <Skeleton className="h-6 w-2/3" />
            ) : (
              <>
                <h2 className="text-base font-semibold leading-snug">{discussion.title}</h2>
                <p className="mt-1 text-xs text-muted-foreground">
                  {discussion.author?.username ?? ""}
                  {discussion.created_at ? ` · ${new Date(discussion.created_at).toLocaleDateString()}` : ""}
                </p>
              </>
            )}
          </div>
          <button
            type="button"
            onClick={closeRef.current}
            aria-label={t('common.close')}
            className="rounded-md p-1 text-muted-foreground transition-colors duration-150 hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <X className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-4">
          {loading ? (
            <div className="space-y-3" aria-busy="true">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-5/6" />
              <Skeleton className="h-4 w-2/3" />
            </div>
          ) : !discussion ? (
            <div role="alert">
              <EmptyState
                icon={MessageSquareWarning}
                title={t(getUserFacingErrorKey(null, "common.loadFailed"))}
                description={t('ip.hubDiscussionLoadHint')}
                action={
                  <Button size="sm" variant="outline" onClick={() => void load()}>
                    {t('common.retry')}
                  </Button>
                }
              />
            </div>
          ) : (
            <>
              {discussion.body && (
                <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground/90">{discussion.body}</p>
              )}
              <div className="mt-6 border-t border-border pt-4">
                <ReplyList discussionId={discussion.id} replies={replies} onRefresh={() => void load()} />
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
