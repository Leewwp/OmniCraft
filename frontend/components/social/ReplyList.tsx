"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";
import { silentError } from "@/lib/error-handler";
import { cn } from "@/lib/utils";
import { Composer } from "@/components/ui/composer";

interface Reply {
  id: number;
  author?: { id?: number; username?: string };
  body: string;
  parent_id?: number | null;
  created_at?: string;
}

interface ReplyListProps {
  discussionId: number;
  replies: Reply[];
  onRefresh?: () => void;
  className?: string;
}

export function ReplyList({ discussionId, replies, onRefresh, className }: ReplyListProps) {
  const t = useTranslations();
  const { user } = useAuth();
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit() {
    if (!body.trim()) return;
    setBusy(true);
    setError("");
    try {
      await api.post(`/api/v1/discussions/${discussionId}/comments`, { content: body.trim() });
      setBody("");
      onRefresh?.();
    } catch (e) {
      setError(t(getUserFacingErrorKey(e)));
      silentError(e, { component: 'ReplyList', action: 'handleSubmit' });
    } finally {
      setBusy(false);
    }
  }

  function renderReplies(parentId: number | null, depth: number): React.ReactNode {
    return replies
      .filter((r) => (r.parent_id ?? null) === parentId)
      .map((r) => (
        <div key={r.id} className={cn(depth > 0 && "ml-6 border-l-2 border-border pl-4")}>
          <div className="rounded-md bg-muted/20 p-3">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <span className="font-medium text-foreground">{r.author?.username ?? `#${r.author?.id}`}</span>
              {r.created_at && <span>{new Date(r.created_at).toLocaleDateString()}</span>}
            </div>
            <p className="mt-1 text-sm">{r.body}</p>
          </div>
          {depth < 2 && renderReplies(r.id, depth + 1)}
        </div>
      ));
  }

  return (
    <div className={cn("space-y-3", className)}>
      {replies.length === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-6">{t("discussion.noReplies")}</p>
      ) : (
        <div className="space-y-2">{renderReplies(null, 0)}</div>
      )}

      {user && (
        /* #413 F6a：讨论回复升级共享 Composer 多行形态（Enter 提交保持，
           Shift+Enter 换行为多行化自然新增） */
        <Composer
          value={body}
          onChange={setBody}
          onSubmit={() => void handleSubmit()}
          keyMode="enter"
          placeholder={t("discussion.replyPlaceholder")}
          submitLabel={t("discussion.replyAction")}
          submitDisabled={busy || !body.trim()}
          submitting={busy}
        />
      )}

      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  );
}
